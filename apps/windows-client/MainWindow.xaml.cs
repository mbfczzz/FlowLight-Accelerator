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
    const string ServiceMissingNotice = "流光加速器的本地服务没有运行。请重新运行安装程序，或联系客服。";
    const string ServiceLostNotice = "正在尝试重新连接本地服务。";
    const string ServiceBusyNotice = "稍后会自动重试。";
    const string SignedOutLabel = "未登录";
    const string VerificationNotice = "无法确认本地服务的身份。请从官方安装包重新安装；开发调试请设置 FLOWLIGHT_DEV_PIPE=1。";

    // A notice the user should get to read survives this many status polls
    // before the poll is allowed to collapse it.
    static readonly TimeSpan InfoHold = TimeSpan.FromSeconds(6);
    static readonly TimeSpan ErrorHold = TimeSpan.FromSeconds(20);
    static readonly TimeSpan ExitFailureHold = TimeSpan.FromSeconds(60);

    sealed record Game(string ID, string Name, string[] Regions, int Version)
    {
        public string RegionLabel => string.Join(" · ", Regions.Select(r => ToRegion(r).Name));
        public string Initial => Name.Length == 0 ? "?" : Name[..1].ToUpperInvariant();
        // A stable muted hue per game stands in for cover art.
        public System.Windows.Media.Brush TileBrush => TileColor(ID);
    }

    sealed record Line(string ID, string Name, bool Available, double Rtt)
    {
        public string Tag => Available ? "" : "不可用";
        public string RttLabel => Available && Rtt > 0 ? Rtt.ToString("0") + " ms" : "";
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

    // The semantic colours from App.xaml (Good / Warn / Bad / Text).
    static readonly System.Windows.Media.Brush LatencyGood = Frozen(0x5C, 0xC9, 0x8B);
    static readonly System.Windows.Media.Brush LatencyFair = Frozen(0xEE, 0x9A, 0x3C);
    static readonly System.Windows.Media.Brush LatencyPoor = Frozen(0xE4, 0x60, 0x6A);
    static readonly System.Windows.Media.Brush LatencyNeutral = Frozen(0xF3, 0xF0, 0xF6);

    static System.Windows.Media.Brush Frozen(byte r, byte g, byte b)
    {
        var brush = new System.Windows.Media.SolidColorBrush(System.Windows.Media.Color.FromRgb(r, g, b));
        brush.Freeze();
        return brush;
    }

    static readonly Dictionary<string, System.Windows.Media.Brush> tiles = new();

    static System.Windows.Media.Brush TileColor(string id)
    {
        if (!tiles.TryGetValue(id, out var brush))
        {
            var hue = 0;
            foreach (var ch in id)
            {
                hue = (hue * 31 + ch) % 360;
            }
            var (r, g, b) = Hsl(hue, 0.42, 0.34);
            brush = Frozen(r, g, b);
            tiles[id] = brush;
        }
        return brush;
    }

    static (byte, byte, byte) Hsl(double h, double s, double l)
    {
        var c = (1 - Math.Abs(2 * l - 1)) * s;
        var x = c * (1 - Math.Abs(h / 60 % 2 - 1));
        var m = l - c / 2;
        var (r, g, b) = h switch
        {
            < 60 => (c, x, 0.0),
            < 120 => (x, c, 0.0),
            < 180 => (0.0, c, x),
            < 240 => (0.0, x, c),
            < 300 => (x, 0.0, c),
            _ => (c, 0.0, x),
        };
        return ((byte)((r + m) * 255), (byte)((g + m) * 255), (byte)((b + m) * 255));
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
    // The line a running session uses, or the one selected before a catalog
    // reload; LoadLines re-selects it instead of the first available line.
    string? pendingLineID;
    int selectionGeneration;
    int catalogGeneration;
    int stateGeneration;
    Task pendingCommand = Task.CompletedTask;
    Game[]? allGames;
    string? statusError;
    string? commandError;
    string? emptyAction;
    (string Title, string Text) serviceProblem = ("正在连接本地服务", "");
    readonly DispatcherTimer toastTimer = new();

    public MainWindow()
    {
        InitializeComponent();
        // 750 DIP at 150% scaling is taller than a 1080p work area and WPF
        // would not clamp it, hiding the footer behind the taskbar.
        Height = Math.Min(Height, SystemParameters.WorkArea.Height);
        Width = Math.Min(Width, SystemParameters.WorkArea.Width);
        VersionLabel.Text = "v" + ProductVersion() + " · Windows x64";
        pendingLineID = prefs.LineID;
        toastTimer.Tick += (_, _) =>
        {
            toastTimer.Stop();
            Toast.Visibility = Visibility.Collapsed;
        };
        SearchBox.TextChanged += (_, _) => ApplyFilter();
        UpdatePanes();
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

    protected override void OnSourceInitialized(EventArgs e)
    {
        base.OnSourceInitialized(e);
        DarkChrome.Apply(this);
    }

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
            if (loggedIn && allGames is null)
            {
                await LoadCatalogForStatus();
            }
        }
        catch (Exception ex)
        {
            ShowNotice(Describe(ex), kind: NoticeKind.Error);
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
                SetAccount(username);
            }
            // A later sign-out (service restart, logout) prompts once more.
            loginPrompted = false;
        }
        if (Text(result, "control_url") is { } control && Uri.TryCreate(control, UriKind.Absolute, out var uri))
        {
            MenuServer.Text = "服务器 " + uri.Host;
        }
        coreAvailable = Flag(result, "core_available");
        RestoreSelection(result);
        // First contact with a signed-out service: open the sign-in dialog
        // instead of leaving the user to find the button.
        if (!loggedIn && !loginPrompted && !exiting && LoginOverlay.Visibility != Visibility.Visible)
        {
            loginPrompted = true;
            ShowLogin();
        }

        ConnectionState.Text = state switch
        {
            "PREPARING" => "正在分配线路",
            "CONNECTING" => "正在连接",
            "ACTIVE" => "已连接，可以启动游戏了",
            "FAILED" => "连接失败",
            "DEGRADED" => "线路波动，正在重新检测",
            "RECONNECTING" => "正在重新连接",
            "STOPPING" => "正在停止",
            "BUSY" => "本机正在为另一个 Windows 用户加速",
            _ => "未连接",
        };
        StateDot.Fill = Tone(state switch
        {
            "ACTIVE" => "Good",
            "PREPARING" or "CONNECTING" or "DEGRADED" or "RECONNECTING" or "STOPPING" => "Warn",
            "FAILED" or "BUSY" => "Bad",
            _ => "TextTertiary",
        });

        var probe = Child(result, "probe");
        var running = IsRunning(state);
        SessionPane.Visibility = running ? Visibility.Visible : Visibility.Collapsed;
        if (running && probe.ValueKind == JsonValueKind.Object)
        {
            var rtt = Number(probe, "rtt_ms");
            Latency.Text = rtt.ToString("0") + " ms";
            Latency.Foreground = rtt <= 60 ? LatencyGood : rtt <= 120 ? LatencyFair : LatencyPoor;
            ExitAddress.Text = Text(probe, "exit_ip") ?? "—";
        }
        else
        {
            Latency.Text = "—";
            Latency.Foreground = LatencyNeutral;
            ExitAddress.Text = "—";
        }
        LineName.Text = (LineList.SelectedItem as Line)?.Name ?? "—";

        var error = Text(result, "error");
        statusError = string.IsNullOrWhiteSpace(error) ? null : Summarize(error);
        UpdateInlineError();
        UpdatePanes();
        UpdateAction();
    }

    // The service could not be reached at all, or it was reached and failed.
    // Only the first means "the service is not running".
    void ShowServiceFailure(Exception ex)
    {
        serviceConnected = false;
        SessionPane.Visibility = Visibility.Collapsed;
        serviceProblem = ex switch
        {
            ServiceUnavailableException { Busy: true } => ("本地服务正忙", ServiceBusyNotice),
            ServiceUnavailableException => hadService
                ? ("本地服务连接已断开", ServiceLostNotice)
                : ("本地服务未运行", ServiceMissingNotice),
            _ => ("本地服务异常", Describe(ex)),
        };
        UpdatePanes();
        UpdateAction();
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
        if (gameID is null || allGames is not { } games)
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
            if (GameList.ItemsSource is Game[] shown && !shown.Contains(game))
            {
                SearchBox.Text = "";
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
        SetAccount(null);
        catalogError = null;
        pendingLineID = null;
        allGames = null;
        if (GameList.ItemsSource is not null)
        {
            GameList.ItemsSource = null;
        }
        SearchBox.Text = "";
        EmptyGames.Text = "";
        LineHint.Text = "";
    }

    void SetAccount(string? username)
    {
        var signedIn = !string.IsNullOrEmpty(username);
        AccountLabel.Text = signedIn ? username! : SignedOutLabel;
        MenuAccount.Text = AccountLabel.Text;
        AvatarInitial.Text = signedIn ? username![..1].ToUpperInvariant() : "?";
        AccountButton.Visibility = signedIn ? Visibility.Visible : Visibility.Collapsed;
        SignInButton.Visibility = signedIn ? Visibility.Collapsed : Visibility.Visible;
        if (!signedIn)
        {
            CloseMenu();
        }
    }

    // Kernel failures carry the kernel's last output. The notice shows the
    // first line; the full text is in the copied connection diagnostics.
    static string Summarize(string message)
    {
        var lines = message.Split('\n', 2);
        var first = lines[0].TrimEnd('\r');
        return lines.Length == 1 ? first : first + "（完整信息请使用“连接诊断”复制）";
    }

    // One button: start when idle, stop while a session runs.
    void UpdateAction()
    {
        var sessionRunning = serviceConnected && IsRunning(state);
        var idle = state is "IDLE" or "FAILED";
        var interactive = serviceConnected && !commandPending && !exiting;
        if (sessionRunning)
        {
            ActionButton.Content = "停止加速";
            ActionButton.Style = (Style)FindResource(typeof(System.Windows.Controls.Button));
            ActionButton.IsEnabled = interactive;
        }
        else if (serviceConnected && state == "STOPPING")
        {
            ActionButton.Content = "正在停止";
            ActionButton.Style = (Style)FindResource(typeof(System.Windows.Controls.Button));
            ActionButton.IsEnabled = false;
        }
        else
        {
            ActionButton.Content = "开始加速";
            ActionButton.Style = (Style)FindResource("Primary");
            ActionButton.IsEnabled = interactive && loggedIn && coreAvailable && idle &&
                GameList.SelectedItem is Game { Version: > 0 } &&
                RegionList.SelectedItem is Region &&
                LineList.SelectedItem is Line { Available: true };
        }
        var locked = sessionRunning || commandPending || state == "STOPPING";
        // Reloading the catalog mid-session would re-select the first game.
        RefreshButton.IsEnabled = interactive && loggedIn && !locked;
        GameList.IsEnabled = !locked;
        SearchBox.IsEnabled = !locked;
        RegionList.IsEnabled = !locked;
        LineList.IsEnabled = !locked;
    }

    static bool IsRunning(string? state) =>
        state is "PREPARING" or "CONNECTING" or "ACTIVE" or "DEGRADED" or "RECONNECTING";

    static (string?, string?, string?) Pane(string? title, string? text, string? action) => (title, text, action);

    // Which centre pane shows: the game detail, or a message saying why there
    // is nothing to do yet and what would change that.
    void UpdatePanes()
    {
        var (title, text, action) =
            !serviceConnected ? Pane(serviceProblem.Title, serviceProblem.Text, null)
            : !loggedIn ? Pane("登录后开始加速", "使用运营方开通的账号登录。", "login")
            : catalogError is not null ? Pane("游戏列表加载失败", catalogError, "retry")
            : !coreAvailable ? Pane("网络内核未就绪", "请重新运行安装程序完成配置。", null)
            : allGames is null ? Pane("正在加载游戏列表", null, null)
            : allGames.Length == 0 ? Pane("暂无可用游戏", "请联系客服为你的账号开通游戏。", null)
            : GameList.SelectedItem is null ? Pane("没有匹配的游戏", "换个关键词试试。", null)
            : Pane(null, null, null);
        emptyAction = action;
        if (title is null)
        {
            EmptyPane.Visibility = Visibility.Collapsed;
            DetailPane.Visibility = Visibility.Visible;
            return;
        }
        EmptyTitle.Text = title;
        EmptyText.Text = text ?? "";
        EmptyText.Visibility = string.IsNullOrEmpty(text) ? Visibility.Collapsed : Visibility.Visible;
        EmptyAction.Content = action == "login" ? "登录账号" : "重试";
        EmptyAction.Visibility = action is null ? Visibility.Collapsed : Visibility.Visible;
        DetailPane.Visibility = Visibility.Collapsed;
        EmptyPane.Visibility = Visibility.Visible;
    }

    enum NoticeKind { Info, Success, Warning, Error }

    // Transient feedback at the bottom right; it leaves on its own.
    void ShowNotice(string text, TimeSpan? hold = null, NoticeKind kind = NoticeKind.Info)
    {
        Notice.Text = text;
        NoticeStripe.Background = Tone(kind switch
        {
            NoticeKind.Success => "Good",
            NoticeKind.Warning => "Warn",
            NoticeKind.Error => "Bad",
            _ => "Info",
        });
        Toast.Visibility = Visibility.Visible;
        toastTimer.Stop();
        toastTimer.Interval = hold ?? (kind == NoticeKind.Error ? ErrorHold : InfoHold);
        toastTimer.Start();
    }

    System.Windows.Media.Brush Tone(string key) => (System.Windows.Media.Brush)FindResource(key);

    // A failed start or stop, or the service's own failure reason, sits
    // under the status line rather than in a toast somewhere else.
    void UpdateInlineError()
    {
        var text = commandError ?? statusError;
        InlineError.Text = text ?? "";
        InlineError.Visibility = text is null ? Visibility.Collapsed : Visibility.Visible;
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

        // Unpublished games cannot be started, so they are not offered.
        var list = result.ValueKind == JsonValueKind.Array
            ? result.EnumerateArray().Select(ParseGame).OfType<Game>().Where(g => g.Version > 0).ToArray()
            : [];
        catalogError = null;
        // A reload keeps what the user had picked when it still exists.
        var keepGame = (GameList.SelectedItem as Game)?.ID ?? prefs.GameID;
        var keepRegion = (RegionList.SelectedItem as Region)?.ID ?? prefs.RegionID;
        var keepLine = pendingLineID;
        allGames = list;
        ApplyFilter();
        if (list.Length == 0)
        {
            UpdatePanes();
            return;
        }
        var game = list.FirstOrDefault(g => g.ID == keepGame) ?? list[0];
        if (GameList.ItemsSource is Game[] shown && !shown.Contains(game))
        {
            SearchBox.Text = "";
        }
        // Selecting the game clears the line list, which would otherwise
        // overwrite the remembered line before LoadLines can re-apply it.
        GameList.SelectedItem = game;
        pendingLineID = keepLine;
        if (keepRegion is not null && RegionList.ItemsSource is Region[] regions &&
            regions.FirstOrDefault(r => r.ID == keepRegion) is { } region)
        {
            RegionList.SelectedItem = region;
        }
        UpdatePanes();
    }

    // The sidebar lists the games whose name contains the search text; the
    // selection survives as long as its game is still listed.
    void ApplyFilter()
    {
        if (allGames is null)
        {
            GameList.ItemsSource = null;
            EmptyGames.Text = "";
            return;
        }
        var query = SearchBox.Text.Trim();
        var shown = query.Length == 0
            ? allGames
            : allGames.Where(g => g.Name.Contains(query, StringComparison.OrdinalIgnoreCase)).ToArray();
        var selected = GameList.SelectedItem as Game;
        GameList.ItemsSource = shown;
        if (selected is not null && shown.Contains(selected))
        {
            GameList.SelectedItem = selected;
        }
        else if (selected is null && shown.Length > 0 && !IsRunning(state))
        {
            GameList.SelectedItem = shown[0];
        }
        EmptyGames.Text = shown.Length == 0 && allGames.Length > 0 ? "没有匹配的游戏" : "";
        UpdatePanes();
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
            UpdatePanes();
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
        SetAccount(Text(user, "username") ?? username);
        LoginOverlay.Visibility = Visibility.Collapsed;
        loggedIn = true;
        UpdatePanes();
        // The overlay is gone, so a catalog failure goes to the notice area
        // where it can be seen, not to the hidden login error text.
        try
        {
            await LoadGames();
        }
        catch (Exception ex)
        {
            catalogError = Describe(ex);
            UpdatePanes();
        }
        await Status();
    }

    void Account_Click(object sender, RoutedEventArgs e) => ShowLogin();

    void Switch_Click(object sender, RoutedEventArgs e) => ShowLogin();

    void Menu_Click(object sender, RoutedEventArgs e) =>
        MenuOverlay.Visibility = MenuOverlay.Visibility == Visibility.Visible ? Visibility.Collapsed : Visibility.Visible;

    void CloseMenu() => MenuOverlay.Visibility = Visibility.Collapsed;

    // A click anywhere outside the menu closes it and goes no further.
    void MenuOverlay_MouseDown(object sender, System.Windows.Input.MouseButtonEventArgs e)
    {
        if (!MenuPanel.IsMouseOver)
        {
            CloseMenu();
            e.Handled = true;
        }
    }

    void EmptyAction_Click(object sender, RoutedEventArgs e)
    {
        if (emptyAction == "login")
        {
            ShowLogin();
        }
        else if (emptyAction == "retry")
        {
            Refresh_Click(sender, e);
        }
    }

    // Pre-fills the last account so a returning user only types the password.
    void ShowLogin()
    {
        if (Username.Text.Length == 0)
        {
            Username.Text = loggedIn && AccountLabel.Text != SignedOutLabel ? AccountLabel.Text : prefs.Username ?? "";
        }
        CloseMenu();
        LoginTitle.Text = loggedIn ? "切换账号" : "登录";
        LoginSubtitle.Text = loggedIn ? "登录另一个账号后，当前账号会退出。" : "";
        LoginSubtitle.Visibility = loggedIn ? Visibility.Visible : Visibility.Collapsed;
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
            ShowNotice("已停止加速", InfoHold, NoticeKind.Success);
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
            ShowNotice("游戏列表已更新", InfoHold, NoticeKind.Success);
        }
        catch (Exception ex)
        {
            ShowNotice(Describe(ex), ErrorHold, NoticeKind.Error);
        }
    }

    async void Logout_Click(object sender, RoutedEventArgs e)
    {
        CloseMenu();
        if (IsRunning(state))
        {
            ShowNotice("请先停止加速，再退出登录", InfoHold, NoticeKind.Warning);
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
        GameTitle.Text = game?.Name ?? "";
        GameRegions.Text = game?.RegionLabel ?? "";
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
        commandError = null;
        UpdateInlineError();
        LineHint.Text = game is not null && regions.Length == 0 ? "这款游戏还没有开放区服" : "";
        UpdatePanes();
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
            UpdateAction();
            return;
        }
        UpdateAction();
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
            LineHint.Text = lines.Length == 0 ? "这个区服暂时没有可用线路"
                : lines.All(l => !l.Available) ? "这个区服的线路暂时都不可用"
                : "";
        }
        catch (Exception ex)
        {
            if (generation != selectionGeneration)
            {
                return;
            }
            LineHint.Text = "线路加载失败：" + Describe(ex);
        }
        UpdateAction();
    }

    void Line_Changed(object sender, SelectionChangedEventArgs e)
    {
        // Mirrors the current choice so a catalog reload or region change
        // brings it back instead of the first available line.
        if (LineList.SelectedItem is Line line)
        {
            pendingLineID = line.ID;
            LineName.Text = line.Name;
        }
        commandError = null;
        UpdateInlineError();
        UpdateAction();
    }

    async void Action_Click(object sender, RoutedEventArgs e)
    {
        if (IsRunning(state))
        {
            await Stop();
        }
        else
        {
            await Start();
        }
    }

    async Task Start()
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
        }
        await Status();
    }

    async Task Stop()
    {
        if (await RunCommand("stop"))
        {
            ShowNotice("已停止加速", InfoHold, NoticeKind.Success);
        }
        await Status();
    }

    // Commands change the service's state. While one is in flight the
    // buttons stay disabled, and any status answer requested before it
    // completed is discarded as stale.
    async Task<bool> RunCommand(string command, object? args = null)
    {
        commandPending = true;
        commandError = null;
        UpdateInlineError();
        stateGeneration++;
        UpdateAction();
        try
        {
            var call = Wire.CallAsync(command, args);
            pendingCommand = call;
            await call;
            return true;
        }
        catch (Exception ex)
        {
            // A failed start or stop belongs next to the button; anything
            // else was asked for from the menu and gets a toast.
            if (command is "start" or "stop")
            {
                commandError = Describe(ex);
                UpdateInlineError();
            }
            else
            {
                ShowNotice(Describe(ex), ErrorHold, NoticeKind.Error);
            }
            return false;
        }
        finally
        {
            commandPending = false;
            stateGeneration++;
            UpdateAction();
        }
    }

    async void Diagnostics_Click(object sender, RoutedEventArgs e)
    {
        CloseMenu();
        try
        {
            var result = await Wire.CallAsync("status");
            var text = JsonSerializer.Serialize(result, new JsonSerializerOptions { WriteIndented = true });
            System.Windows.Clipboard.SetText(text);
            ShowNotice("诊断信息已复制到剪贴板", InfoHold, NoticeKind.Success);
        }
        catch (Exception ex)
        {
            ShowNotice(Describe(ex), ErrorHold, NoticeKind.Error);
        }
    }

    async void Exit_Click(object sender, RoutedEventArgs e)
    {
        CloseMenu();
        await Exit();
    }

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
        UpdateAction();
        ShowNotice("正在停止加速并退出", ExitFailureHold);

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
            ShowNotice("未能停止加速，客户端未退出。" + failure, ExitFailureHold, NoticeKind.Error);
            UpdateAction();
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

// Asks the Desktop Window Manager for a title bar in the window's own colours.
// Attributes a Windows build does not know are rejected, which leaves the
// stock light title bar; nothing else depends on them.
static class DarkChrome
{
    [System.Runtime.InteropServices.DllImport("dwmapi.dll")]
    static extern int DwmSetWindowAttribute(IntPtr hwnd, int attribute, ref int value, int size);

    public static void Apply(Window window)
    {
        var hwnd = new System.Windows.Interop.WindowInteropHelper(window).Handle;
        if (hwnd == IntPtr.Zero)
        {
            return;
        }
        Set(hwnd, 20, 1);          // DWMWA_USE_IMMERSIVE_DARK_MODE
        Set(hwnd, 35, 0x00171213); // DWMWA_CAPTION_COLOR, COLORREF of #131217
        Set(hwnd, 36, 0x00F6F0F3); // DWMWA_TEXT_COLOR, COLORREF of #F3F0F6
        Set(hwnd, 34, 0x0048393D); // DWMWA_BORDER_COLOR, COLORREF of #3D3948
    }

    static void Set(IntPtr hwnd, int attribute, int value)
    {
        try
        {
            _ = DwmSetWindowAttribute(hwnd, attribute, ref value, sizeof(int));
        }
        catch (Exception e) when (e is DllNotFoundException or EntryPointNotFoundException)
        {
        }
    }
}
