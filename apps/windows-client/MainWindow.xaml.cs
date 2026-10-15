using System.Globalization;
using System.IO;
using System.Reflection;
using System.Text.Json;
using System.Windows;
using System.Windows.Automation;
using System.Windows.Controls;
using System.Windows.Media;
using System.Windows.Media.Animation;
using System.Windows.Shell;
using System.Windows.Threading;
using FlowLight.Core;
using Brush = System.Windows.Media.Brush;
using Color = System.Windows.Media.Color;
using KeyEventArgs = System.Windows.Input.KeyEventArgs;
using Point = System.Windows.Point;

namespace FlowLight.Client;

public partial class MainWindow : Window
{
    const string ServiceMissingNotice = "流光加速器的本地服务没有运行。请重新运行安装程序，或联系客服。";
    const string ServiceLostNotice = "正在尝试重新连接本地服务。";
    const string ServiceBusyNotice = "稍后会自动重试。";
    const string VerificationNotice = "无法确认本地服务的身份。请从官方安装包重新安装；开发调试请设置 FLOWLIGHT_DEV_PIPE=1。";

    static readonly TimeSpan InfoHold = TimeSpan.FromSeconds(6);
    static readonly TimeSpan ErrorHold = TimeSpan.FromSeconds(20);
    static readonly TimeSpan ExitFailureHold = TimeSpan.FromSeconds(60);

    sealed record Game(string ID, string Name, string[] Regions, int Version)
    {
        // Cover colour from the control service's cover index, so a game
        // looks the same here and in the admin; an older service sends
        // none and the colour comes from the game's id instead.
        public double Hue { get; init; } = double.NaN;
        // The picture uploaded for the game in the admin, if any.
        public string? Art { get; init; }
        public string RegionLabel => Regions.Length == 0 ? "暂未开放区服" : string.Join(" · ", Regions.Select(r => ToRegion(r).Name));
        public string Initial => Name.Length == 0 ? "?" : Name[..1].ToUpperInvariant();
    }

    // A line from the control service. Latency is the control plane's
    // historical probe; load is active sessions against capacity.
    sealed record Line(string ID, string Name, bool Available, double Rtt, int Capacity, long Active)
    {
        public bool Recommended { get; init; }
        // The service refused to start on this line moments ago.
        public bool Refused { get; init; }
        public bool Selectable => Available && !Refused;
        public int Bars => Selectable ? SignalLevel(Rtt) : 0;
        public Brush QualityBrush => Selectable && Rtt > 0 ? Quality(Rtt) : Palette.Get("TextTertiary");
        public string RttText => Selectable && Rtt > 0 ? Rtt.ToString("0") : "—";
        public string RttUnit => Selectable && Rtt > 0 ? "ms" : "";
        public Visibility RecommendedVisibility => Recommended ? Visibility.Visible : Visibility.Collapsed;
        double Load => Capacity > 0 ? (double)Active / Capacity : -1;
        public string Detail => Refused ? "刚才无法使用，刷新后可重试" : !Available ? "暂不可用" : Load < 0 ? "负载未知" : Load < 0.5 ? "负载低" : Load < 0.8 ? "负载中" : "负载高";
        public Brush LoadBrush => Palette.Get(Refused ? "Bad" : Load < 0 ? "TextTertiary" : Load < 0.5 ? "Good" : Load < 0.8 ? "Warn" : "Bad");
        public Visibility LoadVisibility => Refused || Available && Load >= 0 ? Visibility.Visible : Visibility.Collapsed;
    }

    // Region identifiers come from the control service as short codes; the
    // familiar ones get a readable name, anything else is shown as is.
    sealed record Region(string ID, string Name)
    {
        public int Slot { get; init; }
        public string Code => (ID.Length <= 3 ? ID : ID[..3]).ToUpperInvariant();
        public Brush Badge => Palette.Solid(Palette.Hsl(RegionHues[Slot % RegionHues.Length], 0.45, 0.42));
    }

    // Consecutive entries are at least 100 degrees apart.
    static readonly double[] RegionHues = [205, 345, 155, 25, 270, 120];

    static readonly Dictionary<string, string> RegionNames = new(StringComparer.OrdinalIgnoreCase)
    {
        ["kr"] = "韩服", ["jp"] = "日服", ["tw"] = "台服", ["hk"] = "港服", ["sg"] = "新加坡", ["sea"] = "东南亚",
        ["na"] = "美服", ["us"] = "美服", ["eu"] = "欧服", ["cn"] = "国服", ["br"] = "巴西", ["latam"] = "拉美",
        ["oce"] = "大洋洲", ["ru"] = "俄服", ["global"] = "国际服",
    };

    static Region ToRegion(string id) => new(id, RegionNames.TryGetValue(id, out var name) ? name : id);

    static readonly ImageSource RunningBadge = MakeRunningBadge();

    static ImageSource MakeRunningBadge()
    {
        var group = new DrawingGroup();
        group.Children.Add(new GeometryDrawing(Palette.Solid(Color.FromRgb(0x5C, 0xC9, 0x8B)), null, new EllipseGeometry(new Point(8, 8), 8, 8)));
        group.Children.Add(new GeometryDrawing(Palette.Solid(Color.FromRgb(0x0E, 0x2A, 0x1B)), null, Geometry.Parse("M8.8,2.6 L4.2,9 H7.6 L7,13.4 L11.8,7 H8.4 Z")));
        var image = new DrawingImage(group);
        image.Freeze();
        return image;
    }

    static int SignalLevel(double rtt) => rtt <= 0 ? 0 : rtt <= 60 ? 4 : rtt <= 100 ? 3 : rtt <= 160 ? 2 : 1;

    static Brush Quality(double rtt) => Palette.Get(rtt <= 60 ? "Good" : rtt <= 120 ? "Warn" : "Bad");

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

    // Library cards flow into as many columns as fit at about 236 DIP each.
    public static readonly DependencyProperty CardWidthProperty = DependencyProperty.Register(
        nameof(CardWidth), typeof(double), typeof(MainWindow), new PropertyMetadata(236.0));
    public static readonly DependencyProperty CoverHeightProperty = DependencyProperty.Register(
        nameof(CoverHeight), typeof(double), typeof(MainWindow), new PropertyMetadata(116.0));
    public static readonly DependencyProperty RunningGameIDProperty = DependencyProperty.Register(
        nameof(RunningGameID), typeof(string), typeof(MainWindow), new PropertyMetadata(null));
    public static readonly DependencyProperty LastGameIDProperty = DependencyProperty.Register(
        nameof(LastGameID), typeof(string), typeof(MainWindow), new PropertyMetadata(null));

    public double CardWidth { get => (double)GetValue(CardWidthProperty); set => SetValue(CardWidthProperty, value); }
    public double CoverHeight { get => (double)GetValue(CoverHeightProperty); set => SetValue(CoverHeightProperty, value); }
    public string? RunningGameID { get => (string?)GetValue(RunningGameIDProperty); set => SetValue(RunningGameIDProperty, value); }
    public string? LastGameID { get => (string?)GetValue(LastGameIDProperty); set => SetValue(LastGameIDProperty, value); }

    void UpdateCardWidth(double listWidth)
    {
        var usable = Math.Max(200, listWidth - 14);
        var columns = Math.Max(1, (int)(usable / 236));
        CardWidth = Math.Floor(usable / columns);
        // Card margin 16, padding 8 on each side; covers are 16:9.
        CoverHeight = Math.Round((CardWidth - 16 - 16) * 9 / 16);
    }

    enum View { Library, Game }

    readonly Preferences prefs = Preferences.Load();
    readonly DispatcherTimer timer = new() { Interval = TimeSpan.FromSeconds(2) };
    readonly DispatcherTimer clock = new() { Interval = TimeSpan.FromSeconds(1) };
    readonly DispatcherTimer toastTimer = new();
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
    string? account;
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
    View view = View.Library;

    // The running session as last reported, and its probe history.
    string? sessionGameID;
    string? sessionRegionID;
    string? trackedSession;
    string? restoredSession;
    string? lastObserved;
    DateTime? activeSince;
    double? liveRtt;
    double? liveJitter;
    double? liveLoss;
    string? liveExit;
    double? cometSeconds;
    readonly List<(DateTime At, double Ms)> samples = new();
    // Lines the service refused to start on, and when; a refresh clears
    // them and they lapse on their own after RefusalLifetime.
    readonly Dictionary<string, DateTime> refusedLines = new();
    static readonly TimeSpan RefusalLifetime = TimeSpan.FromMinutes(3);

    public MainWindow()
    {
        InitializeComponent();
        // 750 DIP at 150% scaling is taller than a 1080p work area and WPF
        // would not clamp it.
        Height = Math.Min(Height, SystemParameters.WorkArea.Height);
        Width = Math.Min(Width, SystemParameters.WorkArea.Width);
        VersionLabel.Text = "v" + ProductVersion() + " · Windows x64";
        pendingLineID = prefs.LineID;
        LastGameID = prefs.GameID;
        GameList.SizeChanged += (_, e) => UpdateCardWidth(e.NewSize.Width);
        Dashboard.SizeChanged += (_, e) => StatGrid.Columns = e.NewSize.Width < 540 ? 2 : 4;
        SearchBox.TextChanged += (_, _) => ApplyFilter();
        toastTimer.Tick += (_, _) =>
        {
            toastTimer.Stop();
            Toast.Visibility = Visibility.Collapsed;
        };
        clock.Tick += (_, _) => RenderElapsed();
        clock.Start();
        SetAccount(null);
        UpdatePanes();
        UpdateAction();

        trayMenu = new System.Windows.Forms.ContextMenuStrip();
        trayMenu.Items.Add("打开流光加速器", null, (_, _) => Dispatcher.Invoke(ShowFromTray));
        trayMenu.Items.Add("停止加速", null, (_, _) => Dispatcher.InvokeAsync(StopFromTray));
        trayMenu.Items.Add(new System.Windows.Forms.ToolStripSeparator());
        trayMenu.Items.Add("退出流光加速器", null, (_, _) => Dispatcher.InvokeAsync(Exit));
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
                // The running session can only be shown once its game is listed.
                RestoreSelection(result);
                UpdatePanes();
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
            SetAccount(Text(result, "username") ?? account ?? "");
            // A later sign-out (service restart, logout) prompts once more.
            loginPrompted = false;
        }
        if (Text(result, "control_url") is { } control && Uri.TryCreate(control, UriKind.Absolute, out var uri))
        {
            MenuServer.Text = "服务器 " + uri.Host;
            CoverArt.SetServer(uri);
        }
        coreAvailable = Flag(result, "core_available");

        var running = IsRunning(state);
        sessionGameID = running ? Text(result, "game_id") : null;
        sessionRegionID = running ? Text(result, "region_id") : null;
        RunningGameID = sessionGameID;
        LastGameID = running ? null : prefs.GameID;
        TrackSession(result, running);
        RestoreSelection(result);

        // First contact with a signed-out service: open the sign-in dialog
        // instead of leaving the user to find the button.
        if (!loggedIn && !loginPrompted && !exiting && LoginOverlay.Visibility != Visibility.Visible)
        {
            loginPrompted = true;
            ShowLogin();
        }

        var error = Text(result, "error");
        statusError = string.IsNullOrWhiteSpace(error) ? null : Summarize(error);
        UpdateInlineError();
        UpdatePanes();
        UpdateAction();
    }

    // Keeps the live figures of the running session and one sample per
    // tunnel probe for the chart; a new session starts a new history.
    void TrackSession(JsonElement result, bool running)
    {
        var session = running ? Text(result, "session_id") ?? "session" : null;
        if (session != trackedSession)
        {
            trackedSession = session;
            samples.Clear();
            lastObserved = null;
            activeSince = null;
            Chart.SetSamples(samples);
        }
        if (!running)
        {
            liveRtt = liveJitter = liveLoss = null;
            liveExit = null;
            return;
        }

        if (Text(result, "active_since") is { } since && TryParseTime(since, out var startedAt))
        {
            activeSince = startedAt;
        }
        else if (state is "ACTIVE" or "DEGRADED" && activeSince is null)
        {
            // An older service does not report it; count from first sight.
            activeSince = DateTime.UtcNow;
        }

        var probe = Child(result, "probe");
        if (probe.ValueKind != JsonValueKind.Object)
        {
            liveRtt = liveJitter = liveLoss = null;
            liveExit = null;
            return;
        }
        var rtt = Number(probe, "rtt_ms");
        liveRtt = rtt > 0 ? rtt : null;
        liveJitter = Child(probe, "jitter_ms").ValueKind == JsonValueKind.Number ? Number(probe, "jitter_ms") : null;
        // Loss is only measured over UDP; a TCP-only probe says nothing about it.
        var loss = Number(probe, "loss");
        liveLoss = Flag(probe, "udp") || loss > 0 ? loss : null;
        liveExit = Text(probe, "exit_ip");

        var observed = Text(probe, "observed_at");
        if (liveRtt is { } ms && (observed is null ? samples.Count == 0 : observed != lastObserved))
        {
            lastObserved = observed;
            samples.Add((observed is not null && TryParseTime(observed, out var at) ? at : DateTime.UtcNow, ms));
            if (samples.Count > 120)
            {
                samples.RemoveAt(0);
            }
            Chart.Ink = Quality(ms);
            Chart.SetSamples(samples);
        }
    }

    static bool TryParseTime(string text, out DateTime utc)
    {
        if (DateTime.TryParse(text, CultureInfo.InvariantCulture, DateTimeStyles.RoundtripKind, out var parsed))
        {
            utc = parsed.Kind == DateTimeKind.Unspecified ? DateTime.SpecifyKind(parsed, DateTimeKind.Utc) : parsed.ToUniversalTime();
            return true;
        }
        utc = default;
        return false;
    }

    // The service could not be reached at all, or it was reached and failed.
    // Only the first means "the service is not running".
    void ShowServiceFailure(Exception ex)
    {
        serviceConnected = false;
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
    // as running, so a restarted or second client shows what is connected,
    // and opens that game's page once per session.
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
        if (restoredSession != trackedSession)
        {
            restoredSession = trackedSession;
            ShowView(View.Game);
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
        SearchEmpty.Visibility = Visibility.Collapsed;
        LineHint.Text = "";
        view = View.Library;
    }

    void SetAccount(string? username)
    {
        var signedIn = !string.IsNullOrEmpty(username);
        account = signedIn ? username : null;
        var initial = signedIn ? username![..1].ToUpperInvariant() : "";
        AvatarInitial.Text = initial;
        MenuInitial.Text = initial;
        MenuAccount.Text = username ?? "";
        AvatarInitial.Visibility = signedIn ? Visibility.Visible : Visibility.Collapsed;
        AvatarGlyph.Visibility = signedIn ? Visibility.Collapsed : Visibility.Visible;
        // Signed in, the menu shows the name; a tooltip would sit under it.
        AvatarButton.ToolTip = signedIn ? null : "登录账号";
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
        return lines.Length == 1 ? first : first + "（完整信息请复制连接诊断查看）";
    }

    static bool IsRunning(string? state) =>
        state is "PREPARING" or "CONNECTING" or "ACTIVE" or "DEGRADED" or "RECONNECTING";

    void ShowView(View next)
    {
        if (next == View.Game && GameList.SelectedItem is not Game)
        {
            next = View.Library;
        }
        view = next;
        UpdatePanes();
    }

    static (string?, string?, string?, string) Pane(string? title, string? text, string? action, string icon) => (title, text, action, icon);

    // Which centre pane shows: a page, or a message saying why there is
    // nothing to do yet and what would change that.
    void UpdatePanes()
    {
        var (title, text, action, icon) =
            !serviceConnected ? Pane(serviceProblem.Title, serviceProblem.Text, null, "IconPlug")
            : !loggedIn ? Pane("登录后开始加速", "使用运营方开通的账号登录，即可看到你的游戏和线路。", "login", "IconUser")
            : catalogError is not null ? Pane("游戏列表加载失败", catalogError, "retry", "IconAlert")
            : !coreAvailable ? Pane("网络内核未就绪", "请重新运行安装程序完成配置。", null, "IconAlert")
            : allGames is null ? Pane("正在加载游戏列表", null, null, "IconRefresh")
            : allGames.Length == 0 ? Pane("暂无可用游戏", "请联系客服为你的账号开通游戏。", null, "IconGamepad")
            : Pane(null, null, null, "");
        emptyAction = action;
        var hasGame = GameList.SelectedItem is Game;
        NavGame.IsEnabled = title is null && hasGame;
        NavLibrary.IsEnabled = title is null;
        if (view == View.Game && !hasGame)
        {
            view = View.Library;
        }
        if (title is null)
        {
            EmptyPane.Visibility = Visibility.Collapsed;
            LibraryPage.Visibility = view == View.Library ? Visibility.Visible : Visibility.Collapsed;
            GamePage.Visibility = view == View.Game ? Visibility.Visible : Visibility.Collapsed;
            NavLibrary.IsChecked = view == View.Library;
            NavGame.IsChecked = view == View.Game;
            return;
        }
        NavLibrary.IsChecked = false;
        NavGame.IsChecked = false;
        EmptyGlyph.Data = (Geometry)FindResource(icon);
        EmptyTitle.Text = title;
        EmptyText.Text = text ?? "";
        EmptyText.Visibility = string.IsNullOrEmpty(text) ? Visibility.Collapsed : Visibility.Visible;
        EmptyAction.Content = action == "login" ? "登录账号" : "重试";
        EmptyAction.Visibility = action is null ? Visibility.Collapsed : Visibility.Visible;
        LibraryPage.Visibility = Visibility.Collapsed;
        GamePage.Visibility = Visibility.Collapsed;
        EmptyPane.Visibility = Visibility.Visible;
    }

    // Everything that follows from the session state: the dial, the stop
    // button, which half of the game page shows, what is locked.
    void UpdateAction()
    {
        var running = serviceConnected && IsRunning(state);
        var stopping = serviceConnected && state == "STOPPING";
        var idle = state is "IDLE" or "FAILED";
        var interactive = serviceConnected && !commandPending && !exiting;
        var ready = !running && !stopping && interactive && loggedIn && coreAvailable && idle &&
            GameList.SelectedItem is Game { Version: > 0 } &&
            RegionList.SelectedItem is Region &&
            LineList.SelectedItem is Line { Selectable: true };

        ActionButton.IsEnabled = ready;
        RenderDial(ready, running, stopping);
        StopButton.Visibility = running ? Visibility.Visible : Visibility.Collapsed;
        StopButton.IsEnabled = interactive && running;
        var live = running || stopping;
        Selectors.Visibility = live ? Visibility.Collapsed : Visibility.Visible;
        Dashboard.Visibility = live ? Visibility.Visible : Visibility.Collapsed;
        Summary.Visibility = live ? Visibility.Collapsed : Visibility.Visible;

        var locked = running || commandPending || stopping;
        // Reloading the catalog mid-session would re-select the first game.
        RefreshButton.IsEnabled = interactive && loggedIn && !locked;
        // While a session runs the library stays readable; GameCard_Down
        // explains why another game cannot be picked.
        GameList.IsEnabled = !(commandPending || stopping);
        SearchBox.IsEnabled = !locked;
        RegionList.IsEnabled = !locked;
        LineList.IsEnabled = !locked;
        LinesRefreshButton.IsEnabled = interactive && loggedIn && !locked;

        ConnectionState.Text = !serviceConnected ? "本地服务未连接" : state switch
        {
            "PREPARING" => "正在分配线路",
            "CONNECTING" => "正在建立连接",
            "ACTIVE" => "已连接，可以启动游戏了",
            "FAILED" => "连接失败",
            "DEGRADED" => "线路波动，正在重新检测",
            "RECONNECTING" => "正在重新连接",
            "STOPPING" => "正在停止",
            "BUSY" => "本机正在为另一个 Windows 用户加速",
            _ => "未加速",
        };
        var tone = !serviceConnected ? "TextTertiary" : state switch
        {
            "ACTIVE" => "Good",
            "PREPARING" or "CONNECTING" or "DEGRADED" or "RECONNECTING" or "STOPPING" => "Warn",
            "FAILED" or "BUSY" => "Bad",
            _ => "TextTertiary",
        };
        StateDot.Fill = Palette.Get(tone);
        HeroDot.Fill = Palette.Get(tone);
        NavRunningDot.Visibility = running ? Visibility.Visible : Visibility.Collapsed;
        TaskbarItemInfo ??= new TaskbarItemInfo();
        TaskbarItemInfo.Overlay = running ? RunningBadge : null;
        TaskbarItemInfo.Description = running ? "流光加速器 · 加速中" : "流光加速器";
        RenderSession();
    }

    // The dial is the page's one big control: a gold disc to start, a
    // spinning light while connecting, and the live latency once connected.
    void RenderDial(bool ready, bool running, bool stopping)
    {
        DialIdle.Visibility = Visibility.Collapsed;
        DialBusy.Visibility = Visibility.Collapsed;
        DialLive.Visibility = Visibility.Collapsed;
        string name;
        if (running && state is "ACTIVE" or "DEGRADED")
        {
            var tone = state == "DEGRADED" ? "Warn" : "Good";
            var color = Palette.ColorOf(tone);
            ActionButton.Background = Palette.Get("Elevated");
            DialLive.Visibility = Visibility.Visible;
            DialLatency.Text = liveRtt is { } rtt ? rtt.ToString("0") : "—";
            DialLatency.Foreground = liveRtt is { } quality ? Quality(quality) : Palette.Get("Text");
            DialLiveText.Text = state == "DEGRADED" ? "线路波动" : "当前延迟";
            DialTrack.Stroke = Palette.Solid(Palette.WithAlpha(color, 70));
            DialGlow.Fill = Palette.Get(tone);
            DialGlow.Opacity = 0.22;
            SetComet(color, 3.2);
            name = "加速中";
        }
        else if (running || stopping || commandPending && !ready && IsStartPending())
        {
            ActionButton.Background = Palette.Get("Elevated");
            DialBusy.Visibility = Visibility.Visible;
            DialBusyTitle.Text = stopping ? "正在停止" : "正在连接";
            DialBusyText.Text = state switch
            {
                "PREPARING" => "正在分配线路",
                "CONNECTING" => "正在建立连接",
                "RECONNECTING" => "正在重新连接",
                "STOPPING" => "正在回收资源",
                _ => "正在提交",
            };
            DialTrack.Stroke = Palette.Get("Border");
            DialGlow.Opacity = 0;
            SetComet(Palette.ColorOf(stopping ? "TextTertiary" : "Accent"), 1.1);
            name = DialBusyTitle.Text;
        }
        else
        {
            DialIdle.Visibility = Visibility.Visible;
            ActionButton.Background = ready ? Palette.Get("AccentFill") : Palette.Get("Elevated");
            var ink = Palette.Get(ready ? "OnAccent" : "TextTertiary");
            DialBolt.Ink = ink;
            DialStartText.Foreground = ink;
            DialTrack.Stroke = Palette.Get(ready ? "AccentRing" : "Border");
            DialGlow.Fill = Palette.Get("Accent");
            DialGlow.Opacity = ready ? 0.32 : 0;
            SetComet(ready ? Palette.ColorOf("AccentHover") : null, 6);
            name = "开始加速";
        }
        AutomationProperties.SetName(ActionButton, name);
    }

    bool startPending;

    bool IsStartPending() => startPending;

    // A short bright arc that travels round the dial's ring: amber and quick
    // while connecting, the state colour and slow once connected.
    void SetComet(Color? color, double seconds)
    {
        if (color is not { } c)
        {
            DialComet.Visibility = Visibility.Collapsed;
            CometSpin.BeginAnimation(RotateTransform.AngleProperty, null);
            cometSeconds = null;
            return;
        }
        var brush = new LinearGradientBrush
        {
            MappingMode = BrushMappingMode.Absolute,
            StartPoint = new Point(100, 3),
            EndPoint = new Point(195.5, 83.2),
        };
        brush.GradientStops.Add(new GradientStop(Palette.WithAlpha(c, 0), 0));
        brush.GradientStops.Add(new GradientStop(c, 1));
        brush.Freeze();
        DialComet.Stroke = brush;
        DialComet.Visibility = Visibility.Visible;
        if (!SystemParameters.ClientAreaAnimation)
        {
            CometSpin.BeginAnimation(RotateTransform.AngleProperty, null);
            cometSeconds = null;
            return;
        }
        if (cometSeconds != seconds)
        {
            cometSeconds = seconds;
            CometSpin.BeginAnimation(RotateTransform.AngleProperty, new DoubleAnimation(0, 360, TimeSpan.FromSeconds(seconds))
            {
                RepeatBehavior = RepeatBehavior.Forever,
            });
        }
    }

    // The live figures wherever they appear: dashboard tiles, the library
    // banner, the hero's status pill.
    void RenderSession()
    {
        var running = serviceConnected && IsRunning(state);
        StatLatency.Text = liveRtt is { } rtt ? rtt.ToString("0") : "—";
        StatLatency.Foreground = liveRtt is { } q ? Quality(q) : Palette.Get("Text");
        StatJitter.Text = liveJitter is { } jitter ? jitter.ToString("0") : "—";
        StatLoss.Text = liveLoss is { } loss ? (loss * 100).ToString("0") : "—";
        StatLossUnit.Visibility = liveLoss is null ? Visibility.Collapsed : Visibility.Visible;
        DashExit.Text = liveExit ?? "—";
        DashLine.Text = (LineList.SelectedItem as Line)?.Name ?? "—";
        DashRegion.Text = (RegionList.SelectedItem as Region)?.Name ?? (sessionRegionID is null ? "—" : ToRegion(sessionRegionID).Name);

        var runningGame = allGames?.FirstOrDefault(g => g.ID == sessionGameID);
        RunningBanner.Visibility = running && runningGame is not null ? Visibility.Visible : Visibility.Collapsed;
        if (runningGame is not null)
        {
            BannerCover.Seed = runningGame.ID;
            BannerCover.Hue = runningGame.Hue;
            BannerCover.Art = runningGame.Art;
            BannerCover.Monogram = runningGame.Initial;
            BannerTitle.Text = "正在加速 " + runningGame.Name;
            BannerDetail.Text = string.Join(" · ", new[] { DashRegion.Text, DashLine.Text }.Where(t => t != "—"));
            BannerLatency.Text = StatLatency.Text;
            BannerLatency.Foreground = StatLatency.Foreground;
        }
        RenderElapsed();
    }

    void RenderElapsed()
    {
        var connected = serviceConnected && state is "ACTIVE" or "DEGRADED";
        var elapsed = connected && activeSince is { } since ? DateTime.UtcNow - since : (TimeSpan?)null;
        var text = elapsed is { } span ? $"{(int)Math.Max(0, span.TotalHours):00}:{Math.Max(0, span.Minutes):00}:{Math.Max(0, span.Seconds):00}" : "—";
        StatElapsed.Text = text;
        HeroStatus.Text = !serviceConnected ? "本地服务未连接"
            : connected ? "加速中 " + text
            : IsRunning(state) ? "正在连接"
            : state == "STOPPING" ? "正在停止"
            : "未加速";
    }

    enum NoticeKind { Info, Success, Warning, Error }

    // Transient feedback at the bottom right; it leaves on its own.
    void ShowNotice(string text, TimeSpan? hold = null, NoticeKind kind = NoticeKind.Info)
    {
        var (tone, icon) = kind switch
        {
            NoticeKind.Success => ("Good", "IconCheck"),
            NoticeKind.Warning => ("Warn", "IconAlert"),
            NoticeKind.Error => ("Bad", "IconError"),
            _ => ("Info", "IconInfo"),
        };
        Notice.Text = text;
        ToastGlyph.Data = (Geometry)FindResource(icon);
        ToastGlyph.Ink = Palette.Get(tone);
        ToastBadge.Background = Palette.Get(tone + "Soft");
        Toast.Visibility = Visibility.Visible;
        toastTimer.Stop();
        toastTimer.Interval = hold ?? (kind == NoticeKind.Error ? ErrorHold : InfoHold);
        toastTimer.Start();
    }

    // A failed start or stop, or the service's own failure reason, sits
    // under the dial rather than in a toast somewhere else.
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
        // The game played last comes first.
        list = list.OrderByDescending(g => g.ID == prefs.GameID).ToArray();
        catalogError = null;
        // A reload keeps what the user had picked when it still exists.
        var keepGame = (GameList.SelectedItem as Game)?.ID ?? prefs.GameID;
        var keepRegion = (RegionList.SelectedItem as Region)?.ID ?? prefs.RegionID;
        var keepLine = pendingLineID;
        allGames = list;
        try
        {
            ApplyFilter();
            if (list.Length == 0)
            {
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
        }
        finally
        {
            UpdatePanes();
        }
    }

    // The library lists the games whose name contains the search text; the
    // selection survives as long as its game is still listed.
    void ApplyFilter()
    {
        if (allGames is null)
        {
            GameList.ItemsSource = null;
            SearchEmpty.Visibility = Visibility.Collapsed;
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
        var noMatch = shown.Length == 0 && allGames.Length > 0;
        SearchEmptyTitle.Text = "没有找到“" + query + "”";
        SearchEmpty.Visibility = noMatch ? Visibility.Visible : Visibility.Collapsed;
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
        var cover = Child(game, "cover_index");
        return new Game(id, Text(game, "name") ?? id, regionNames, published)
        {
            Hue = cover.ValueKind == JsonValueKind.Number && cover.TryGetInt32(out var index) && index >= 0
                ? Palette.SteppedHue(index)
                : double.NaN,
            Art = Text(game, "cover_image") is { } art && CoverArt.IsName(art) ? art : null,
        };
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
        return new Line(
            id,
            Text(line, "name") ?? id,
            Flag(line, "available"),
            Number(Child(line, "probe"), "rtt_ms"),
            (int)Number(line, "capacity"),
            (long)Number(line, "active_sessions"));
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
        catch (Exception e) when (e is ArgumentException or IOException or System.Security.SecurityException)
        {
        }
        return System.Drawing.SystemIcons.Application;
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
        view = View.Library;
        UpdatePanes();
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

    void Avatar_Click(object sender, RoutedEventArgs e)
    {
        if (!loggedIn)
        {
            ShowLogin();
            return;
        }
        MenuOverlay.Visibility = MenuOverlay.Visibility == Visibility.Visible ? Visibility.Collapsed : Visibility.Visible;
    }

    void Switch_Click(object sender, RoutedEventArgs e) => ShowLogin();

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
            Username.Text = account ?? prefs.Username ?? "";
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

    void LoginCancel_Click(object sender, RoutedEventArgs e)
    {
        Password.Clear();
        LoginOverlay.Visibility = Visibility.Collapsed;
    }

    async void Refresh_Click(object sender, RoutedEventArgs e)
    {
        try
        {
            catalogError = null;
            refusedLines.Clear();
            await LoadGames();
            await LoadLines();
            ShowNotice("游戏列表已更新", InfoHold, NoticeKind.Success);
        }
        catch (Exception ex)
        {
            catalogError = Describe(ex);
            UpdatePanes();
            ShowNotice(catalogError, ErrorHold, NoticeKind.Error);
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

    // UpdatePanes checks the rail item that matches the view, which raises
    // Checked again; that call finds the view already in place.
    // The line list's own refresh, reachable where a refused line says to
    // refresh: forgets refusals and asks the service again.
    async void RefreshLines_Click(object sender, RoutedEventArgs e)
    {
        refusedLines.Clear();
        commandError = null;
        UpdateInlineError();
        await LoadLines();
        ShowNotice("线路已刷新", InfoHold, NoticeKind.Success);
    }

    void ClearSearch_Click(object sender, RoutedEventArgs e)
    {
        SearchBox.Text = "";
        SearchBox.Focus();
    }

    void SearchBox_KeyDown(object sender, KeyEventArgs e)
    {
        if (e.Key == System.Windows.Input.Key.Escape && SearchBox.Text.Length > 0)
        {
            SearchBox.Text = "";
            e.Handled = true;
        }
    }

    void NavLibrary_Checked(object sender, RoutedEventArgs e)
    {
        if (view != View.Library)
        {
            ShowView(View.Library);
        }
    }

    void NavGame_Checked(object sender, RoutedEventArgs e)
    {
        if (view != View.Game)
        {
            ShowView(View.Game);
        }
    }

    void Back_Click(object sender, RoutedEventArgs e) => ShowView(View.Library);

    void OpenRunning_Click(object sender, RoutedEventArgs e) => ShowView(View.Game);

    bool SwitchBlocked(Game game) => IsRunning(state) && game.ID != sessionGameID;

    // Picking another game mid-session would orphan the running one.
    void GameCard_Down(object sender, System.Windows.Input.MouseButtonEventArgs e)
    {
        if (sender is ListBoxItem { DataContext: Game game } && SwitchBlocked(game))
        {
            e.Handled = true;
            var running = allGames?.FirstOrDefault(g => g.ID == sessionGameID)?.Name ?? "当前游戏";
            ShowNotice("正在为 " + running + " 加速，停止后才能切换游戏", InfoHold, NoticeKind.Warning);
        }
    }

    void GameList_PreviewKeyDown(object sender, KeyEventArgs e)
    {
        if (IsRunning(state) && e.Key is System.Windows.Input.Key.Up or System.Windows.Input.Key.Down
            or System.Windows.Input.Key.Left or System.Windows.Input.Key.Right
            or System.Windows.Input.Key.Home or System.Windows.Input.Key.End
            or System.Windows.Input.Key.PageUp or System.Windows.Input.Key.PageDown)
        {
            e.Handled = true;
        }
    }

    // A click on a card opens the game even when it is already selected.
    void GameCard_Click(object sender, System.Windows.Input.MouseButtonEventArgs e)
    {
        if (sender is ListBoxItem { DataContext: Game game } && GameList.IsEnabled && !SwitchBlocked(game))
        {
            if (!ReferenceEquals(GameList.SelectedItem, game))
            {
                GameList.SelectedItem = game;
            }
            ShowView(View.Game);
        }
    }

    void GameCard_Key(object sender, KeyEventArgs e)
    {
        if (e.Key is System.Windows.Input.Key.Enter or System.Windows.Input.Key.Space && sender is ListBoxItem { DataContext: Game })
        {
            ShowView(View.Game);
            e.Handled = true;
        }
    }

    void Game_Changed(object sender, SelectionChangedEventArgs e)
    {
        var game = GameList.SelectedItem as Game;
        var regions = game?.Regions ?? Array.Empty<string>();
        HeroCover.Seed = game?.ID ?? "";
        HeroCover.Hue = game?.Hue ?? double.NaN;
        HeroCover.Art = game?.Art;
        HeroCover.Monogram = game?.Initial ?? "";
        HeroTitle.Text = game?.Name ?? "";
        var regionItems = regions.Select((r, i) => ToRegion(r) with { Slot = i }).ToArray();
        HeroRegions.ItemsSource = regionItems;
        // Replacing the regions may or may not raise Region_Changed (WPF keeps
        // an equal selected item), so lines are reloaded explicitly below.
        updatingRegions = true;
        try
        {
            RegionList.ItemsSource = game is null ? null : regionItems;
            RegionList.SelectedIndex = regions.Length > 0 ? 0 : -1;
        }
        finally
        {
            updatingRegions = false;
        }
        commandError = null;
        UpdateInlineError();
        LineHint.Text = game is not null && regions.Length == 0 ? "这款游戏还没有开放区服" : "";
        UpdateSummary();
        UpdatePanes();
        _ = LoadLines();
    }

    async void Region_Changed(object sender, SelectionChangedEventArgs e)
    {
        UpdateSummary();
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
        LineHint.Text = "正在加载线路…";
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
            foreach (var stale in refusedLines.Where(r => DateTime.UtcNow - r.Value > RefusalLifetime).Select(r => r.Key).ToArray())
            {
                refusedLines.Remove(stale);
            }
            lines = lines.Select(l => refusedLines.ContainsKey(l.ID) ? l with { Refused = true } : l).ToArray();
            var best = lines.Where(l => l.Selectable && l.Rtt > 0).OrderBy(l => l.Rtt).FirstOrDefault();
            lines = lines.Select(l => l == best ? l with { Recommended = true } : l).ToArray();
            LineList.ItemsSource = lines;
            LineList.SelectedItem = lines.FirstOrDefault(l => l.ID == pendingLineID && l.Selectable)
                ?? lines.FirstOrDefault(l => l.Recommended)
                ?? lines.FirstOrDefault(l => l.Selectable);
            LineHint.Text = lines.Length == 0 ? "这个区服暂时没有可用线路，请换个区服或稍后刷新。"
                : lines.All(l => !l.Selectable) ? "这个区服的线路暂时都不可用，请稍后刷新。"
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
        UpdateSummary();
        UpdateAction();
    }

    void Line_Changed(object sender, SelectionChangedEventArgs e)
    {
        // Mirrors the current choice so a catalog reload or region change
        // brings it back instead of the first available line.
        if (LineList.SelectedItem is Line line)
        {
            pendingLineID = line.ID;
        }
        commandError = null;
        UpdateInlineError();
        UpdateSummary();
        UpdateAction();
    }

    void UpdateSummary()
    {
        SummaryRegion.Text = (RegionList.SelectedItem as Region)?.Name ?? "—";
        SummaryLine.Text = LineList.SelectedItem is Line line
            ? line.Name + (line.Rtt > 0 ? " · 历史 " + line.Rtt.ToString("0") + " ms" : "")
            : "—";
    }

    async void Action_Click(object sender, RoutedEventArgs e)
    {
        if (!IsRunning(state))
        {
            await Start();
        }
    }

    async void Stop_Click(object sender, RoutedEventArgs e) => await Stop();

    async Task Start()
    {
        if (GameList.SelectedItem is not Game game ||
            RegionList.SelectedItem is not Region region ||
            LineList.SelectedItem is not Line line)
        {
            return;
        }
        startPending = true;
        try
        {
            if (await RunCommand("start", new { game_id = game.ID, region_id = region.ID, line_id = line.ID }))
            {
                pendingLineID = line.ID;
                prefs.GameID = game.ID;
                prefs.RegionID = region.ID;
                prefs.LineID = line.ID;
                prefs.Save();
            }
            else if (commandError is { } reason && reason.Contains("线路"))
            {
                // The line itself was refused (full, withdrawn, node down):
                // keep it out of reach until the next refresh, move to the
                // best remaining line and say so.
                refusedLines[line.ID] = DateTime.UtcNow;
                await LoadLines();
                // The first clause is the cause; the advice after it is what
                // the client has just done.
                var cause = reason.Split('，')[0].TrimEnd('。');
                commandError = LineList.SelectedItem is Line next
                    ? $"{line.Name} 暂时无法使用（{cause}），已切换到 {next.Name}"
                    : $"{line.Name} 暂时无法使用（{cause}），这个区服暂时没有其他可用线路";
                UpdateInlineError();
            }
        }
        finally
        {
            startPending = false;
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

    async Task StopFromTray()
    {
        if (!IsRunning(state) || commandPending || exiting)
        {
            return;
        }
        await Stop();
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
            // A failed start or stop belongs next to the dial; anything else
            // was asked for from the menu and gets a toast.
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
            ShowNotice("连接诊断已复制到剪贴板，不含密码和密钥", InfoHold, NoticeKind.Success);
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
        clock.Stop();
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
