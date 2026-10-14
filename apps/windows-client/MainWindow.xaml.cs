using System.Text.Json;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Threading;
using FlowLight.Core;

namespace FlowLight.Client;

public partial class MainWindow : Window
{
    const string ServiceMissingNotice = "本地服务尚未启动。请先安装并配置 FlowLight 本地服务。";
    const string ServiceBusyNotice = "本地服务正忙，稍后会自动重试。";
    const string SignInNotice = "请登录后查看游戏与线路。首次使用需要安装本地服务。";

    sealed record Game(string ID, string Name, string[] Regions, int Version)
    {
        public string RuleLabel =>
            Version > 0 ? $"规则版本 {Version} · 选择区服继续" : "规则草稿 · 等待运营发布";
    }

    sealed record Line(string ID, string Name, bool Available, double Rtt);

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
    string state = "IDLE";
    string? catalogError;
    int selectionGeneration;
    int catalogGeneration;
    int stateGeneration;
    Task pendingCommand = Task.CompletedTask;

    public MainWindow()
    {
        InitializeComponent();
        trayMenu = new System.Windows.Forms.ContextMenuStrip();
        trayMenu.Items.Add("打开流光加速器", null, (_, _) => Dispatcher.Invoke(ShowFromTray));
        trayMenu.Items.Add("退出并停止加速", null, (_, _) => Dispatcher.InvokeAsync(Exit));
        tray = new System.Windows.Forms.NotifyIcon
        {
            Icon = System.Drawing.SystemIcons.Application,
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

    void ShowFromTray()
    {
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
        state = Text(result, "state") ?? "IDLE";
        loggedIn = Flag(result, "logged_in");
        if (Text(result, "username") is { } username)
        {
            AccountLabel.Text = username;
        }
        coreAvailable = Flag(result, "core_available");
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
            ProbeTitle.Text = "当前隧道探测";
            Latency.Text = Number(probe, "rtt_ms").ToString("0") + " ms";
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
        else if (loggedIn)
        {
            NoticeBox.Visibility = Visibility.Collapsed;
        }
        else if (Notice.Text == ServiceMissingNotice)
        {
            // The service is back; the outage notice no longer applies.
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
            ServiceLabel.Text = "本地服务未连接";
            ShowNotice(ServiceMissingNotice);
        }
        else
        {
            ServiceLabel.Text = "本地服务异常";
            ShowNotice(Describe(ex));
        }
        UpdateButtons();
    }

    static string Describe(Exception ex) => ex switch
    {
        ServiceUnavailableException { Busy: true } => ServiceBusyNotice,
        ServiceUnavailableException => ServiceMissingNotice,
        _ => Summarize(ex.Message),
    };

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
            RegionList.SelectedItem is string &&
            LineList.SelectedItem is Line { Available: true };
        StopButton.IsEnabled = interactive && sessionRunning;
        var locked = sessionRunning || commandPending;
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
        ExitAddress.Text = "尚无当前出口探测";
    }

    void ShowNotice(string text)
    {
        Notice.Text = text;
        NoticeBox.Visibility = Visibility.Visible;
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
        GameList.ItemsSource = list;
        EmptyGames.Text = list.Length == 0 ? "暂无可用游戏，请联系运营人员配置。" : "";
        if (list.Length > 0)
        {
            GameList.SelectedIndex = 0;
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

    static double Number(JsonElement element, string name)
    {
        var value = Child(element, name);
        return value.ValueKind == JsonValueKind.Number && value.TryGetDouble(out var number) ? number : 0;
    }

    async void Login_Click(object sender, RoutedEventArgs e)
    {
        LoginButton.IsEnabled = false;
        LoginError.Text = "";
        JsonElement user;
        try
        {
            user = await Wire.CallAsync("login", new { username = Username.Text, password = Password.Password });
        }
        catch (Exception ex)
        {
            LoginError.Text = Describe(ex);
            return;
        }
        finally
        {
            LoginButton.IsEnabled = true;
        }

        stateGeneration++;
        Password.Clear();
        AccountLabel.Text = Text(user, "username") ?? Username.Text;
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

    void Account_Click(object sender, RoutedEventArgs e)
    {
        LoginOverlay.Visibility = Visibility.Visible;
        Username.Focus();
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
            ShowNotice("已同步服务端游戏与线路配置。");
        }
        catch (Exception ex)
        {
            ShowNotice(Describe(ex));
        }
    }

    void Library_Click(object sender, RoutedEventArgs e)
    {
        GameList.Focus();
    }

    void Game_Changed(object sender, SelectionChangedEventArgs e)
    {
        var game = GameList.SelectedItem as Game;
        var regions = game?.Regions ?? Array.Empty<string>();
        // Replacing the regions may or may not raise Region_Changed (WPF keeps
        // an equal selected string), so lines are reloaded explicitly below.
        updatingRegions = true;
        try
        {
            RegionList.ItemsSource = game is null ? null : regions;
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
        if (GameList.SelectedItem is not Game game || RegionList.SelectedItem is not string region)
        {
            UpdateButtons();
            return;
        }
        UpdateButtons();
        try
        {
            var rows = await Wire.CallAsync("lines", new { game_id = game.ID, region_id = region });
            if (generation != selectionGeneration)
            {
                return;
            }
            var lines = rows.ValueKind == JsonValueKind.Array
                ? rows.EnumerateArray().Select(ParseLine).OfType<Line>().ToArray()
                : [];
            LineList.ItemsSource = lines;
            LineList.SelectedItem = lines.FirstOrDefault(l => l.Available);
            LineHint.Text = lines.Length == 0 ? "此区服暂无已验证的授权线路" : "线路按历史探测延迟排列";
        }
        catch (Exception ex)
        {
            if (generation != selectionGeneration)
            {
                return;
            }
            ShowNotice(Describe(ex));
        }
        UpdateButtons();
    }

    void Line_Changed(object sender, SelectionChangedEventArgs e)
    {
        ResetEvidence();
        UpdateButtons();
    }

    async void Start_Click(object sender, RoutedEventArgs e)
    {
        if (GameList.SelectedItem is not Game game ||
            RegionList.SelectedItem is not string region ||
            LineList.SelectedItem is not Line line)
        {
            return;
        }
        if (await RunCommand("start", new { game_id = game.ID, region_id = region, line_id = line.ID }))
        {
            ShowNotice("正在准备连接。通过隧道探测后才会显示线路已连接。");
        }
        await Status();
    }

    async void Stop_Click(object sender, RoutedEventArgs e)
    {
        await RunCommand("stop");
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
            ShowNotice(Describe(ex));
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
            ShowNotice("连接诊断已复制，不包含账号密码或隧道密钥。");
        }
        catch (Exception ex)
        {
            ShowNotice(Describe(ex));
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
            ShowNotice("未能停止加速，客户端未退出。" + failure);
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
