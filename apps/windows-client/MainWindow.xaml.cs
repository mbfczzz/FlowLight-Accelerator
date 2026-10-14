using System.IO;
using System.Reflection;
using System.Text.Json;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Threading;
using FlowLight.Core;

namespace FlowLight.Client;

public partial class MainWindow : Window
{
    const string ServiceMissingNotice = "本地服务尚未启动。请先安装并配置 FlowLight 本地服务。";
    const string ServiceLostNotice = "本地服务连接已断开，正在尝试重新连接…";
    const string ServiceBusyNotice = "本地服务正忙，稍后会自动重试。";
    const string SignInNotice = "请登录后查看游戏与线路。首次使用需要安装本地服务。";
    const string SignedOutLabel = "尚未登录";
    const string EmptyCatalogText = "登录后将从服务端加载游戏目录。";
    const string VerificationNotice = "无法确认本地服务身份。请从正式安装包重新安装；开发调试请设置 FLOWLIGHT_DEV_PIPE=1。";

    // A notice the user should get to read survives this many status polls
    // before the poll is allowed to collapse it.
    static readonly TimeSpan InfoHold = TimeSpan.FromSeconds(6);
    static readonly TimeSpan ErrorHold = TimeSpan.FromSeconds(20);
    static readonly TimeSpan ExitFailureHold = TimeSpan.FromSeconds(60);

    sealed record Game(string ID, string Name, string[] Regions, int Version)
    {
        public string RuleLabel =>
            Version > 0 ? $"规则版本 {Version} · 选择区服继续" : "规则草稿 · 等待运营发布";
    }

    sealed record Line(string ID, string Name, bool Available, double Rtt)
    {
        public string Tag => Available ? "" : "· 不可用";
        public double Dim => Available ? 1 : 0.55;
    }

    // Region identifiers come from the control service as short codes; the
    // familiar ones get a readable name, anything else is shown as is.
    sealed record Region(string ID, string Name);

    static readonly Dictionary<string, string> RegionNames = new(StringComparer.OrdinalIgnoreCase)
    {
        ["kr"] = "韩服", ["jp"] = "日服", ["tw"] = "台服", ["hk"] = "港服", ["sg"] = "新加坡", ["sea"] = "东南亚",
        ["na"] = "美服", ["us"] = "美服", ["eu"] = "欧服", ["cn"] = "国服", ["br"] = "巴西", ["latam"] = "拉美",
        ["oce"] = "大洋洲", ["ru"] = "俄服", ["global"] = "国际服",
    };

    static Region ToRegion(string id) => new(id, RegionNames.TryGetValue(id, out var name) ? name : id);

    // What the user picked last time, so the next launch starts where they
    // left off. Stored per Windows user; nothing in it is a secret.
    sealed class Preferences
    {
        public string? Username { get; set; }
        public string? GameID { get; set; }
        public string? RegionID { get; set; }
        public string? LineID { get; set; }

        static readonly string FilePath = Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
            "FlowLightAccelerator",
            "client.json");

        public static Preferences Load()
        {
            try
            {
                if (File.Exists(FilePath))
                {
                    return JsonSerializer.Deserialize<Preferences>(File.ReadAllText(FilePath), Wire.Json) ?? new();
                }
            }
            catch (Exception e) when (e is IOException or JsonException or UnauthorizedAccessException)
            {
            }
            return new();
        }

        public void Save()
        {
            try
            {
                Directory.CreateDirectory(Path.GetDirectoryName(FilePath)!);
                File.WriteAllText(FilePath, JsonSerializer.Serialize(this, Wire.Json));
            }
            catch (Exception e) when (e is IOException or UnauthorizedAccessException)
            {
            }
        }
    }

    static readonly System.Windows.Media.Brush LatencyGood = Frozen(0x4A, 0xDE, 0x80);
    static readonly System.Windows.Media.Brush LatencyFair = Frozen(0xFB, 0xBF, 0x24);
    static readonly System.Windows.Media.Brush LatencyPoor = Frozen(0xF8, 0x71, 0x71);
    static readonly System.Windows.Media.Brush LatencyNeutral = Frozen(0xE7, 0xED, 0xF5);

    static System.Windows.Media.Brush Frozen(byte r, byte g, byte b)
    {
        var brush = new System.Windows.Media.SolidColorBrush(System.Windows.Media.Color.FromRgb(r, g, b));
        brush.Freeze();
        return brush;
    }

    // Game cards flow into as many columns as fit at 420 px each, so a
    // maximised window shows a grid instead of one card stretched across it.
    public static readonly DependencyProperty CardWidthProperty = DependencyProperty.Register(
        nameof(CardWidth), typeof(double), typeof(MainWindow), new PropertyMetadata(double.NaN));

    public double CardWidth
    {
        get => (double)GetValue(CardWidthProperty);
        set => SetValue(CardWidthProperty, value);
    }

    const double MinCardWidth = 420;

    void UpdateCardWidth(double listWidth)
    {
        var usable = Math.Max(0, listWidth - 20);
        var columns = Math.Max(1, (int)(usable / MinCardWidth));
        CardWidth = Math.Floor(usable / columns);
    }

    readonly Preferences prefs = Preferences.Load();
    readonly DispatcherTimer timer = new() { Interval = TimeSpan.FromSeconds(2) };
    readonly System.Windows.Forms.NotifyIcon tray;
    readonly System.Windows.Forms.ContextMenuStrip trayMenu;

    // Everything below is touched only on the UI thread. The hazard is not
    // concurrency but ordering: an answer that arrives after a newer request
    // was made must not overwrite what that newer request produced.
    bool quitting;
    bool exiting;
    bool released;
    bool loggedIn;
    bool coreAvailable;
    bool serviceConnected;
    bool polling;
    bool pollAgain;
    bool commandPending;
    bool updatingRegions;
    bool loginPrompted;
    bool trayHintShown;
    bool hadService;
    string state = "IDLE";
    string? catalogError;
    DateTime noticeHoldUntil = DateTime.MinValue;
    // The line a running session uses, or the one selected before a catalog
    // reload; LoadLines re-selects it instead of the first available line.
    string? pendingLineID;
    int selectionGeneration;
    int catalogGeneration;
    int stateGeneration;
    Task pendingCommand = Task.CompletedTask;

    public MainWindow()
    {
        InitializeComponent();
        // 750 DIP at 150% scaling is taller than a 1080p work area and WPF
        // would not clamp it, hiding the footer behind the taskbar.
        Height = Math.Min(Height, SystemParameters.WorkArea.Height);
        Width = Math.Min(Width, SystemParameters.WorkArea.Width);
        VersionLabel.Text = "v" + ProductVersion() + " · Windows x64";
        pendingLineID = prefs.LineID;
        GameList.SizeChanged += (_, e) => UpdateCardWidth(e.NewSize.Width);
        trayMenu = new System.Windows.Forms.ContextMenuStrip();
        trayMenu.Items.Add("打开流光加速器", null, (_, _) => Dispatcher.Invoke(ShowFromTray));
        trayMenu.Items.Add("停止加速", null, (_, _) => Dispatcher.InvokeAsync(StopFromTray));
        trayMenu.Items.Add(new System.Windows.Forms.ToolStripSeparator());
        trayMenu.Items.Add("退出并停止加速", null, (_, _) => Dispatcher.InvokeAsync(Exit));
        tray = new System.Windows.Forms.NotifyIcon
        {
            Icon = TrayIcon(),
            Text = "流光加速器 · FlowLight Accelerator",
            Visible = true,
            ContextMenuStrip = trayMenu,
        };
        tray.DoubleClick += (_, _) => Dispatcher.Invoke(ShowFromTray);

        // Closing the window only hides it: the session lives in the service
        // and the tray icon stays as the way back to it. Only Exit() quits.
        Closing += (_, e) =>
        {
            if (!quitting)
            {
                e.Cancel = true;
                Hide();
                // Said once per run: users otherwise assume the close button
                // ended the acceleration, or that the app is gone.
                if (!trayHintShown)
                {
                    trayHintShown = true;
                    var text = IsRunning(state) ? "已最小化到托盘，加速继续运行。" : "已最小化到托盘，双击图标可重新打开。";
                    tray.ShowBalloonTip(3000, "流光加速器", text, System.Windows.Forms.ToolTipIcon.Info);
                }
            }
        };
        Closed += (_, _) => ReleaseResources();
        Loaded += async (_, _) =>
        {
            await Status();
            if (!exiting && !released)
            {
                timer.Start();
            }
        };
        timer.Tick += OnTick;
    }

    async void OnTick(object? sender, EventArgs e) => await Status();

    // The build stamps the informational version (e.g. 0.1.0-abc1234+sha);
    // the label shows it without the commit hash.
    static string ProductVersion()
    {
        var info = typeof(MainWindow).Assembly.GetCustomAttribute<AssemblyInformationalVersionAttribute>()?.InformationalVersion;
        if (string.IsNullOrWhiteSpace(info))
        {
            return typeof(MainWindow).Assembly.GetName().Version?.ToString(3) ?? "0.0.0";
        }
        var plus = info.IndexOf('+');
        return plus > 0 ? info[..plus] : info;
    }

    // Also reached from a second launch via App; a window that is quitting
    // must not be shown again, Show() would throw after Close().
    internal void ShowFromTray()
    {
        if (quitting || released)
        {
            return;
        }
        Show();
        if (WindowState == WindowState.Minimized)
        {
            WindowState = WindowState.Normal;
        }
        Activate();
    }

    // Runs one status poll. A call made while a poll is in flight asks that
    // poll to run again rather than being dropped, so the result of a command
    // shows up without waiting for the next tick.
    async Task Status()
    {
        if (polling)
        {
            pollAgain = true;
            return;
        }
        polling = true;
        try
        {
            do
            {
                pollAgain = false;
                await PollOnce();
            }
            while (pollAgain && !quitting && !released);
        }
        finally
        {
            polling = false;
        }
    }

    async Task PollOnce()
    {
        var generation = stateGeneration;
        JsonElement result;
        try
        {
            result = await Wire.CallAsync("status");
        }
        catch (Exception ex)
        {
            if (generation != stateGeneration)
            {
                pollAgain = true;
            }
            else
            {
                ShowServiceFailure(ex);
            }
            return;
        }

        // A start, stop or sign-in finished while this request was in flight,
        // so the answer may describe the state before it. Ask again.
        if (generation != stateGeneration)
        {
            pollAgain = true;
            return;
        }

        try
        {
            ApplyStatus(result);
            if (loggedIn && GameList.ItemsSource == null)
            {
                await LoadCatalogForStatus();
            }
        }
        catch (Exception ex)
        {
            ShowNotice(Describe(ex));
        }
    }

    void ApplyStatus(JsonElement result)
    {
        serviceConnected = true;
        hadService = true;
        state = Text(result, "state") ?? "IDLE";
        loggedIn = Flag(result, "logged_in");
        if (!loggedIn)
        {
            // The service restarted or the account was signed out elsewhere:
            // a stale name and catalog would suggest the session still exists.
            ClearCatalog();
        }
        else
        {
            if (Text(result, "username") is { } username)
            {
                AccountLabel.Text = username;
            }
            // A later sign-out (service restart, logout) prompts once more.
            loginPrompted = false;
        }
        AccountButton.Content = loggedIn ? "切换账号" : "账号登录";
        LogoutButton.Visibility = loggedIn ? Visibility.Visible : Visibility.Collapsed;
        coreAvailable = Flag(result, "core_available");
        RestoreSelection(result);
        // First contact with a signed-out service: open the sign-in dialog
        // instead of leaving the user to find the button in the sidebar.
        if (!loggedIn && !loginPrompted && !exiting && LoginOverlay.Visibility != Visibility.Visible)
        {
            loginPrompted = true;
            ShowLogin();
        }
        ServiceLabel.Text = coreAvailable ? "本地服务已连接" : "网络内核待配置";
        ConnectionState.Text = state switch
        {
            "PREPARING" => "正在准备节点与授权…",
            "CONNECTING" => "正在连接线路…",
            "ACTIVE" => "线路已连接 · 请启动所选游戏",
            "FAILED" => "连接失败",
            "DEGRADED" => "线路探测异常 · 正在复测",
            "RECONNECTING" => "正在重新连接…",
            "STOPPING" => "正在停止并回收资源…",
            "BUSY" => "本机正在为另一用户加速",
            _ => "未连接",
        };

        var probe = Child(result, "probe");
        if (probe.ValueKind == JsonValueKind.Object)
        {
            var rtt = Number(probe, "rtt_ms");
            ProbeTitle.Text = "当前隧道探测";
            Latency.Text = rtt.ToString("0") + " ms";
            Latency.Foreground = rtt <= 60 ? LatencyGood : rtt <= 120 ? LatencyFair : LatencyPoor;
            ExitAddress.Text = "实际出口 " + (Text(probe, "exit_ip") ?? "—");
        }
        else
        {
            ResetEvidence();
        }

        var error = Text(result, "error");
        if (!string.IsNullOrWhiteSpace(error))
        {
            ShowNotice(Summarize(error));
        }
        else if (loggedIn && catalogError is not null)
        {
            ShowNotice(catalogError);
        }
        else if (loggedIn && !coreAvailable)
        {
            ShowNotice("本地服务已连接，网络内核尚未配置。请完成安装配置后开始加速。");
        }
        else if (loggedIn && DateTime.UtcNow >= noticeHoldUntil)
        {
            NoticeBox.Visibility = Visibility.Collapsed;
        }
        else if (!loggedIn && Notice.Text != SignInNotice && DateTime.UtcNow >= noticeHoldUntil)
        {
            // Signed out, whether by a service restart or on purpose: whatever
            // the banner said before no longer applies.
            ShowNotice(SignInNotice);
        }
        UpdateButtons();
    }

    // The service could not be reached at all, or it was reached and failed.
    // Only the first means "the service is not running".
    void ShowServiceFailure(Exception ex)
    {
        serviceConnected = false;
        ResetEvidence(false);
        if (ex is ServiceUnavailableException { Busy: true })
        {
            ServiceLabel.Text = "本地服务繁忙";
            ShowNotice(ServiceBusyNotice);
        }
        else if (ex is ServiceUnavailableException)
        {
            // "Install it first" only makes sense if it was never reached.
            ServiceLabel.Text = "本地服务未连接";
            ShowNotice(hadService ? ServiceLostNotice : ServiceMissingNotice);
        }
        else
        {
            ServiceLabel.Text = "本地服务异常";
            ShowNotice(Describe(ex));
        }
        UpdateButtons();
    }

    // Only ServiceUnavailableException wraps I/O failures, so a bare
    // IOException from the pipe layer is the service identity check failing.
    static string Describe(Exception ex) => ex switch
    {
        ServiceUnavailableException { Busy: true } => ServiceBusyNotice,
        ServiceUnavailableException => ServiceMissingNotice,
        IOException => VerificationNotice,
        JsonException or KeyNotFoundException => "本地服务返回了无法识别的响应，请重新安装或更新客户端。",
        _ => Summarize(ex.Message),
    };

    // Re-selects the game, region and line of the session the service reports
    // as running, so a restarted or second client shows what is connected
    // rather than the first entries of each list.
    void RestoreSelection(JsonElement result)
    {
        if (!IsRunning(state))
        {
            return;
        }
        var gameID = Text(result, "game_id");
        if (gameID is null || GameList.ItemsSource is not Game[] games)
        {
            return;
        }
        pendingLineID = Text(result, "line_id");
        if (GameList.SelectedItem is not Game current || current.ID != gameID)
        {
            var game = games.FirstOrDefault(g => g.ID == gameID);
            if (game is null)
            {
                return;
            }
            GameList.SelectedItem = game;
        }
        if (Text(result, "region_id") is { } regionID &&
            (RegionList.SelectedItem as Region)?.ID != regionID &&
            RegionList.ItemsSource is Region[] regions &&
            regions.FirstOrDefault(r => r.ID == regionID) is { } region)
        {
            RegionList.SelectedItem = region;
        }
        if (pendingLineID is { } lineID &&
            LineList.ItemsSource is Line[] lines &&
            (LineList.SelectedItem as Line)?.ID != lineID &&
            lines.FirstOrDefault(l => l.ID == lineID) is { } line)
        {
            LineList.SelectedItem = line;
        }
    }

    void ClearCatalog()
    {
        AccountLabel.Text = SignedOutLabel;
        catalogError = null;
        pendingLineID = null;
        if (GameList.ItemsSource is not null)
        {
            GameList.ItemsSource = null;
        }
        EmptyGames.Text = EmptyCatalogText;
        LineHint.Text = "请选择游戏与区服";
    }

    // Kernel failures carry the kernel's last output. The notice shows the
    // first line; the full text is in the copied connection diagnostics.
    static string Summarize(string message)
    {
        var lines = message.Split('\n', 2);
        var first = lines[0].TrimEnd('\r');
        return lines.Length == 1 ? first : first + "（完整信息请使用“连接诊断”复制）";
    }

    void UpdateButtons()
    {
        var sessionRunning = serviceConnected && IsRunning(state);
        var idle = state == "IDLE" || state == "FAILED";
        var interactive = serviceConnected && !commandPending && !exiting;
        StartButton.IsEnabled = interactive && loggedIn && coreAvailable && idle &&
            GameList.SelectedItem is Game { Version: > 0 } &&
            RegionList.SelectedItem is Region &&
            LineList.SelectedItem is Line { Available: true };
        // The button itself reports progress; a user should not have to read
        // the footer to know a click did something.
        StartButton.Content = !sessionRunning ? "开始加速"
            : state is "ACTIVE" or "DEGRADED" ? "加速中"
            : "正在连接…";
        StopButton.IsEnabled = interactive && sessionRunning;
        var locked = sessionRunning || commandPending;
        // Reloading the catalog mid-session would re-select the first game.
        RefreshButton.IsEnabled = interactive && loggedIn && !locked;
        GameList.IsEnabled = !locked;
        RegionList.IsEnabled = !locked;
        LineList.IsEnabled = !locked;
    }

    static bool IsRunning(string? state) =>
        state is "PREPARING" or "CONNECTING" or "ACTIVE" or "DEGRADED" or "RECONNECTING";

    void ResetEvidence(bool allowHistory = true)
    {
        ProbeTitle.Text = "线路历史探测";
        Latency.Text = allowHistory && LineList.SelectedItem is Line { Rtt: > 0 } line
            ? line.Rtt.ToString("0") + " ms"
            : "—";
        Latency.Foreground = LatencyNeutral;
        ExitAddress.Text = "尚无当前出口探测";
    }

    // A notice with a hold stays until it expires; the status poll otherwise
    // collapses the banner on its next pass.
    void ShowNotice(string text, TimeSpan? hold = null)
    {
        Notice.Text = text;
        NoticeBox.Visibility = Visibility.Visible;
        noticeHoldUntil = hold is { } duration ? DateTime.UtcNow + duration : DateTime.MinValue;
    }

    async Task LoadGames()
    {
        var generation = ++catalogGeneration;
        JsonElement result;
        try
        {
            result = await Wire.CallAsync("games");
        }
        catch (Exception) when (generation != catalogGeneration)
        {
            return;
        }
        if (generation != catalogGeneration)
        {
            return;
        }

        var list = result.ValueKind == JsonValueKind.Array
            ? result.EnumerateArray().Select(ParseGame).OfType<Game>().ToArray()
            : [];
        catalogError = null;
        // A reload keeps what the user had picked when it still exists.
        var keepGame = (GameList.SelectedItem as Game)?.ID ?? prefs.GameID;
        var keepRegion = (RegionList.SelectedItem as Region)?.ID ?? prefs.RegionID;
        var keepLine = pendingLineID;
        GameList.ItemsSource = list;
        EmptyGames.Text = list.Length == 0 ? "暂无可用游戏，请联系运营人员配置。" : "";
        if (list.Length == 0)
        {
            return;
        }
        var game = list.FirstOrDefault(g => g.ID == keepGame) ?? list[0];
        // Selecting the game clears the line list, which would otherwise
        // overwrite the remembered line before LoadLines can re-apply it.
        GameList.SelectedItem = game;
        pendingLineID = keepLine;
        if (keepRegion is not null && RegionList.ItemsSource is Region[] regions &&
            regions.FirstOrDefault(r => r.ID == keepRegion) is { } region)
        {
            RegionList.SelectedItem = region;
        }
    }

    // A catalog that fails to load during a poll is retried on the next one;
    // its error stays visible until then instead of flickering.
    async Task LoadCatalogForStatus()
    {
        try
        {
            await LoadGames();
        }
        catch (Exception ex)
        {
            catalogError = Describe(ex);
            ShowNotice(catalogError);
        }
    }

    static Game? ParseGame(JsonElement game)
    {
        var id = Text(game, "id");
        if (string.IsNullOrEmpty(id))
        {
            return null;
        }
        var regions = Child(game, "regions");
        var regionNames = regions.ValueKind == JsonValueKind.Array
            ? regions.EnumerateArray()
                .Where(r => r.ValueKind == JsonValueKind.String)
                .Select(r => r.GetString()!)
                .ToArray()
            : [];
        var version = Child(game, "published_version");
        var published = version.ValueKind == JsonValueKind.Number && version.TryGetInt32(out var v) ? v : 0;
        return new Game(id, Text(game, "name") ?? id, regionNames, published);
    }

    // A line the control plane has not probed yet still belongs in the list;
    // it simply has no history to show.
    static Line? ParseLine(JsonElement line)
    {
        var id = Text(line, "id");
        if (string.IsNullOrEmpty(id))
        {
            return null;
        }
        return new Line(id, Text(line, "name") ?? id, Flag(line, "available"), Number(Child(line, "probe"), "rtt_ms"));
    }

    static JsonElement Child(JsonElement element, string name) =>
        element.ValueKind == JsonValueKind.Object && element.TryGetProperty(name, out var value) ? value : default;

    static string? Text(JsonElement element, string name)
    {
        var value = Child(element, name);
        return value.ValueKind == JsonValueKind.String ? value.GetString() : null;
    }

    static bool Flag(JsonElement element, string name) => Child(element, name).ValueKind == JsonValueKind.True;

    // The executable carries the brand icon, so the tray uses the same one.
    // Fall back to the generic system icon if it cannot be read.
    static System.Drawing.Icon TrayIcon()
    {
        try
        {
            if (Environment.ProcessPath is { } path && System.Drawing.Icon.ExtractAssociatedIcon(path) is { } extracted)
            {
                return extracted;
            }
        }
        catch (Exception e) when (e is ArgumentException or System.IO.IOException or System.Security.SecurityException)
        {
        }
        return System.Drawing.SystemIcons.Application;
    }

    static double Number(JsonElement element, string name)
    {
        var value = Child(element, name);
        return value.ValueKind == JsonValueKind.Number && value.TryGetDouble(out var number) ? number : 0;
    }

    async void Login_Click(object sender, RoutedEventArgs e)
    {
        var username = Username.Text.Trim();
        if (username.Length == 0 || Password.Password.Length == 0)
        {
            LoginError.Text = "请输入账号和密码。";
            (username.Length == 0 ? (System.Windows.Controls.Control)Username : Password).Focus();
            return;
        }
        LoginButton.IsEnabled = false;
        LoginButton.Content = "正在登录…";
        LoginError.Text = "";
        JsonElement user;
        try
        {
            user = await Wire.CallAsync("login", new { username, password = Password.Password });
        }
        catch (Exception ex)
        {
            LoginError.Text = Describe(ex);
            Password.Focus();
            Password.SelectAll();
            return;
        }
        finally
        {
            LoginButton.IsEnabled = true;
            LoginButton.Content = "登录";
        }

        stateGeneration++;
        Password.Clear();
        prefs.Username = username;
        prefs.Save();
        AccountLabel.Text = Text(user, "username") ?? username;
        LoginOverlay.Visibility = Visibility.Collapsed;
        NoticeBox.Visibility = Visibility.Collapsed;
        loggedIn = true;
        // The overlay is gone, so a catalog failure goes to the notice area
        // where it can be seen, not to the hidden login error text.
        try
        {
            await LoadGames();
        }
        catch (Exception ex)
        {
            catalogError = Describe(ex);
            ShowNotice(catalogError);
        }
        await Status();
    }

    void Account_Click(object sender, RoutedEventArgs e) => ShowLogin();

    // Pre-fills the last account so a returning user only types the password.
    void ShowLogin()
    {
        if (Username.Text.Length == 0)
        {
            Username.Text = loggedIn && AccountLabel.Text != SignedOutLabel ? AccountLabel.Text : prefs.Username ?? "";
        }
        LoginTitle.Text = loggedIn ? "切换账号" : "登录流光加速器";
        LoginSubtitle.Text = loggedIn ? "登录另一个账号后，当前账号会退出。" : "使用平台账号获取游戏与线路授权。";
        LoginError.Text = "";
        LoginOverlay.Visibility = Visibility.Visible;
        if (Username.Text.Length > 0)
        {
            Password.Focus();
        }
        else
        {
            Username.Focus();
        }
    }

    async Task StopFromTray()
    {
        if (!IsRunning(state) || commandPending || exiting)
        {
            return;
        }
        if (await RunCommand("stop"))
        {
            ShowNotice("已停止加速。", InfoHold);
        }
        await Status();
    }

    void LoginCancel_Click(object sender, RoutedEventArgs e)
    {
        Password.Clear();
        LoginOverlay.Visibility = Visibility.Collapsed;
    }

    async void Refresh_Click(object sender, RoutedEventArgs e)
    {
        try
        {
            await LoadGames();
            ShowNotice("已同步服务端游戏与线路配置。", InfoHold);
        }
        catch (Exception ex)
        {
            ShowNotice(Describe(ex), ErrorHold);
        }
    }

    async void Logout_Click(object sender, RoutedEventArgs e)
    {
        if (IsRunning(state))
        {
            ShowNotice("请先停止加速，再退出登录。", InfoHold);
            return;
        }
        if (await RunCommand("logout"))
        {
            ClearCatalog();
            loggedIn = false;
        }
        await Status();
    }

    void Game_Changed(object sender, SelectionChangedEventArgs e)
    {
        var game = GameList.SelectedItem as Game;
        var regions = game?.Regions ?? Array.Empty<string>();
        // Replacing the regions may or may not raise Region_Changed (WPF keeps
        // an equal selected item), so lines are reloaded explicitly below.
        updatingRegions = true;
        try
        {
            RegionList.ItemsSource = game is null ? null : regions.Select(ToRegion).ToArray();
            RegionList.SelectedIndex = regions.Length > 0 ? 0 : -1;
        }
        finally
        {
            updatingRegions = false;
        }
        ResetEvidence();
        LineHint.Text = game is null
            ? "请选择游戏与区服"
            : game.Version == 0
                ? "该游戏规则尚未发布"
                : regions.Length == 0 ? "尚未配置游戏区服" : "选择可用线路";
        _ = LoadLines();
    }

    async void Region_Changed(object sender, SelectionChangedEventArgs e)
    {
        if (!updatingRegions)
        {
            await LoadLines();
        }
    }

    // Each call supersedes the previous one. The list is cleared first so a
    // line from another region can never be started, and an answer for an
    // older selection, success or failure, is ignored.
    async Task LoadLines()
    {
        var generation = ++selectionGeneration;
        LineList.ItemsSource = null;
        if (GameList.SelectedItem is not Game game || RegionList.SelectedItem is not Region region)
        {
            UpdateButtons();
            return;
        }
        UpdateButtons();
        try
        {
            var rows = await Wire.CallAsync("lines", new { game_id = game.ID, region_id = region.ID });
            if (generation != selectionGeneration)
            {
                return;
            }
            var lines = rows.ValueKind == JsonValueKind.Array
                ? rows.EnumerateArray().Select(ParseLine).OfType<Line>().ToArray()
                : [];
            LineList.ItemsSource = lines;
            LineList.SelectedItem = lines.FirstOrDefault(l => l.ID == pendingLineID) ?? lines.FirstOrDefault(l => l.Available);
            LineHint.Text = lines.Length == 0 ? "此区服暂无已验证的授权线路" : "线路按历史探测延迟排列";
        }
        catch (Exception ex)
        {
            if (generation != selectionGeneration)
            {
                return;
            }
            ShowNotice(Describe(ex), ErrorHold);
        }
        UpdateButtons();
    }

    void Line_Changed(object sender, SelectionChangedEventArgs e)
    {
        // Mirrors the current choice so a catalog reload or region change
        // brings it back instead of the first available line.
        if (LineList.SelectedItem is Line line)
        {
            pendingLineID = line.ID;
            // The disabled start button needs a reason next to the choice.
            LineHint.Text = line.Available ? "线路按历史探测延迟排列" : "该线路当前不可用，请选择其他线路。";
        }
        ResetEvidence();
        UpdateButtons();
    }

    async void Start_Click(object sender, RoutedEventArgs e)
    {
        if (GameList.SelectedItem is not Game game ||
            RegionList.SelectedItem is not Region region ||
            LineList.SelectedItem is not Line line)
        {
            return;
        }
        if (await RunCommand("start", new { game_id = game.ID, region_id = region.ID, line_id = line.ID }))
        {
            pendingLineID = line.ID;
            prefs.GameID = game.ID;
            prefs.RegionID = region.ID;
            prefs.LineID = line.ID;
            prefs.Save();
            ShowNotice("正在准备连接。通过隧道探测后才会显示线路已连接。", InfoHold);
        }
        await Status();
    }

    async void Stop_Click(object sender, RoutedEventArgs e)
    {
        if (await RunCommand("stop"))
        {
            ShowNotice("已停止加速。", InfoHold);
        }
        await Status();
    }

    // Commands change the service's state. While one is in flight the
    // buttons stay disabled, and any status answer requested before it
    // completed is discarded as stale.
    async Task<bool> RunCommand(string command, object? args = null)
    {
        commandPending = true;
        stateGeneration++;
        UpdateButtons();
        try
        {
            var call = Wire.CallAsync(command, args);
            pendingCommand = call;
            await call;
            return true;
        }
        catch (Exception ex)
        {
            ShowNotice(Describe(ex), ErrorHold);
            return false;
        }
        finally
        {
            commandPending = false;
            stateGeneration++;
            UpdateButtons();
        }
    }

    async void Diagnostics_Click(object sender, RoutedEventArgs e)
    {
        try
        {
            var result = await Wire.CallAsync("status");
            var text = JsonSerializer.Serialize(result, new JsonSerializerOptions { WriteIndented = true });
            System.Windows.Clipboard.SetText(text);
            ShowNotice("连接诊断已复制，不包含账号密码或隧道密钥。", InfoHold);
        }
        catch (Exception ex)
        {
            ShowNotice(Describe(ex), ErrorHold);
        }
    }

    async void Exit_Click(object sender, RoutedEventArgs e) => await Exit();

    // Quits only once the service confirms this user has no running session.
    // If the stop cannot be confirmed the window comes back with the reason,
    // so the session is never left running behind a client that is gone.
    async Task Exit()
    {
        if (exiting || quitting)
        {
            return;
        }
        exiting = true;
        timer.Stop();
        UpdateButtons();
        ShowNotice("正在停止加速并退出…");

        string? failure;
        try
        {
            failure = await StopForExit();
        }
        catch (Exception ex)
        {
            failure = Describe(ex);
        }

        if (failure is not null)
        {
            exiting = false;
            if (!released)
            {
                timer.Start();
            }
            ShowFromTray();
            ShowNotice("未能停止加速，客户端未退出。" + failure, ExitFailureHold);
            UpdateButtons();
            return;
        }

        quitting = true;
        ReleaseResources();
        System.Windows.Application.Current.Shutdown();
    }

    async Task<string?> StopForExit()
    {
        // A start that is still in flight must land before it can be stopped.
        try
        {
            await pendingCommand;
        }
        catch (Exception)
        {
        }

        // Whatever the stop call says, the status that follows decides: a
        // session owned by another Windows user, or none at all, lets the
        // client quit; a session of ours that is still running does not.
        string? lastError = null;
        for (var attempt = 0; attempt < 2; attempt++)
        {
            try
            {
                await Wire.CallAsync("stop");
            }
            catch (Exception ex)
            {
                lastError = Describe(ex);
            }

            JsonElement status;
            try
            {
                status = await Wire.CallAsync("status");
            }
            catch (ServiceUnavailableException)
            {
                // No service, no session: the kernel dies with the service's
                // job object.
                return null;
            }
            catch (Exception ex)
            {
                lastError = Describe(ex);
                continue;
            }
            if (!IsRunning(Text(status, "state")))
            {
                return null;
            }
        }
        return lastError ?? "本地服务仍在加速。";
    }

    // Called from Exit() and from Closed, whichever comes first.
    void ReleaseResources()
    {
        if (released)
        {
            return;
        }
        released = true;
        timer.Stop();
        timer.Tick -= OnTick;
        tray.Visible = false;
        tray.Dispose();
        trayMenu.Dispose();
    }
}
