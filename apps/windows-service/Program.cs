using System.IO.Pipes;
using System.Runtime.InteropServices;
using System.Security.AccessControl;
using System.Security.Cryptography;
using System.Security.Principal;
using System.Text;
using System.Text.Json;
using FlowLight.Core;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;

var builder = Host.CreateApplicationBuilder(args);
builder.Services.AddWindowsService(o => o.ServiceName = "FlowLightAccelerator");
builder.Services.AddHostedService<AcceleratorService>();
await builder.Build().RunAsync();

// A request the service can read but not act on: not JSON, or a field that is
// missing or has the wrong type. The message goes back to the client as is.
sealed class MalformedRequestException(string message) : Exception(message);

sealed class ServiceConfig
{
    public string ControlUrl { get; set; } = "";
    public string CorePath { get; set; } = "";
    public string CoreSha256 { get; set; } = "";
    public bool AllowLocalHttp { get; set; }
    public string DataDir { get; set; } = Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.CommonApplicationData),
        "FlowLightAccelerator",
        "runtime");
}

// What `status` reports about the session. It is replaced as a whole, so a
// reader never sees one field from before an update and another from after.
// Game, region and line let a client that starts mid-session show what the
// session is for.
sealed record SessionView(
    string State,
    string Error,
    ProbeResult? Probe,
    DateTime? LeaseUntil,
    string? SessionID,
    string? GameID,
    string? RegionID,
    string? LineID)
{
    public static readonly SessionView Idle = new("IDLE", "", null, null, null, null, null, null);
}

// The Windows user who owns the session signed out.
sealed class OwnerSignedOutException() : Exception("Windows 用户已注销，加速已停止");

static class LogonSessions
{
    // Whether `sid` still has a logon session on this machine. Unknown
    // (enumeration failed or the service lacks the privilege, as when run
    // from a console) counts as present, so a session is never stopped on a
    // guess.
    public static bool HasLogon(string sid)
    {
        if (!WTSEnumerateSessions(IntPtr.Zero, 0, 1, out var list, out var count))
        {
            return true;
        }
        try
        {
            var size = Marshal.SizeOf<SessionInfo>();
            for (var i = 0; i < count; i++)
            {
                var info = Marshal.PtrToStructure<SessionInfo>(list + i * size);
                if (!WTSQueryUserToken(info.SessionId, out var token))
                {
                    if (Marshal.GetLastWin32Error() == PrivilegeNotHeld)
                    {
                        return true;
                    }
                    continue;
                }
                try
                {
                    using var identity = new WindowsIdentity(token);
                    if (identity.User?.Value == sid)
                    {
                        return true;
                    }
                }
                finally
                {
                    CloseHandle(token);
                }
            }
            return false;
        }
        finally
        {
            WTSFreeMemory(list);
        }
    }

    const int PrivilegeNotHeld = 1314;

    [StructLayout(LayoutKind.Sequential)]
    struct SessionInfo
    {
        public int SessionId;
        public IntPtr WinStationName;
        public int State;
    }

    [DllImport("wtsapi32.dll", SetLastError = true)]
    static extern bool WTSEnumerateSessions(IntPtr server, int reserved, int version, out IntPtr sessions, out int count);

    [DllImport("wtsapi32.dll")]
    static extern void WTSFreeMemory(IntPtr memory);

    [DllImport("wtsapi32.dll", SetLastError = true)]
    static extern bool WTSQueryUserToken(int sessionId, out IntPtr token);

    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool CloseHandle(IntPtr handle);
}

sealed class Identity(ApiClient api, JsonElement user, string deviceId) : IDisposable
{
    public ApiClient Api { get; } = api;
    public JsonElement User { get; } = user;
    public string DeviceID { get; } = deviceId;

    public void Dispose() => Api.Dispose();
}

sealed class AcceleratorService(ILogger<AcceleratorService> log) : BackgroundService
{
    // A client that connects and sends nothing gives up its pipe instance
    // after ReadTimeout. The dispatch then gets its own, longer budget: stop
    // and logout wait for the session to wind down.
    static readonly TimeSpan ReadTimeout = TimeSpan.FromSeconds(5);
    static readonly TimeSpan DispatchTimeout = TimeSpan.FromSeconds(30);
    const int MaxRequestChars = 65536;

    readonly Dictionary<string, Identity> users = [];
    readonly SemaphoreSlim gate = new(1, 1);
    ServiceConfig config = new();
    MihomoKernel? kernel;
    string? owner;
    // Who owned the session whose failure the view still shows. The error
    // can carry kernel output and the probe an exit address, so no other
    // Windows user gets to see them.
    string? failedOwner;
    CancellationTokenSource? running;
    Task? sessionTask;
    CancellationToken serviceToken;
    readonly object viewLock = new();
    volatile SessionView view = SessionView.Idle;

    void Update(Func<SessionView, SessionView> change)
    {
        lock (viewLock)
        {
            view = change(view);
        }
    }

    protected override async Task ExecuteAsync(CancellationToken ct)
    {
        serviceToken = ct;
        var path = Environment.GetEnvironmentVariable("FLOWLIGHT_SERVICE_CONFIG") ?? Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.CommonApplicationData),
            "FlowLightAccelerator",
            "service.json");
        config = JsonSerializer.Deserialize<ServiceConfig>(await File.ReadAllTextAsync(path, ct), Wire.Json)
            ?? throw new InvalidOperationException("本地服务配置缺失，请重新安装");
        Directory.CreateDirectory(config.DataDir);
        kernel = new MihomoKernel(config.CorePath, config.CoreSha256, config.DataDir);

        var security = new PipeSecurity();
        security.AddAccessRule(new PipeAccessRule(
            new SecurityIdentifier(WellKnownSidType.NetworkSid, null),
            PipeAccessRights.FullControl,
            AccessControlType.Deny));
        security.AddAccessRule(new PipeAccessRule(
            new SecurityIdentifier(WellKnownSidType.AuthenticatedUserSid, null),
            PipeAccessRights.ReadWrite,
            AccessControlType.Allow));
        security.AddAccessRule(new PipeAccessRule(
            new SecurityIdentifier(WellKnownSidType.LocalSystemSid, null),
            PipeAccessRights.FullControl,
            AccessControlType.Allow));
        var serviceSid = WindowsIdentity.GetCurrent().User
            ?? throw new InvalidOperationException("本地服务身份信息不可用，请重新安装");
        security.AddAccessRule(new PipeAccessRule(serviceSid, PipeAccessRights.FullControl, AccessControlType.Allow));

        // A client that disconnects early, or all instances being busy, must
        // not end ExecuteAsync: that would stop the host and the tunnel.
        while (!ct.IsCancellationRequested)
        {
            NamedPipeServerStream? pipe = null;
            try
            {
                pipe = NamedPipeServerStreamAcl.Create(
                    Wire.PipeName,
                    PipeDirection.InOut,
                    8,
                    PipeTransmissionMode.Byte,
                    PipeOptions.Asynchronous,
                    65536,
                    65536,
                    security);
                await pipe.WaitForConnectionAsync(ct);
                _ = Handle(pipe, ct);
                pipe = null;
            }
            catch (OperationCanceledException) when (ct.IsCancellationRequested)
            {
                break;
            }
            catch (Exception e)
            {
                log.LogWarning(e, "Local pipe connection failed");
                try
                {
                    await Task.Delay(250, ct);
                }
                catch (OperationCanceledException)
                {
                    break;
                }
            }
            finally
            {
                pipe?.Dispose();
            }
        }
    }

    async Task Handle(NamedPipeServerStream pipe, CancellationToken stoppingToken)
    {
        using (pipe)
        {
            try
            {
                using var reader = new StreamReader(pipe, leaveOpen: true);
                using var writer = new StreamWriter(pipe, leaveOpen: true) { AutoFlush = true };

                string line;
                using (var readDeadline = CancellationTokenSource.CreateLinkedTokenSource(stoppingToken))
                {
                    readDeadline.CancelAfter(ReadTimeout);
                    line = await ReadRequestLine(reader, readDeadline.Token);
                }

                string sid = "";
                pipe.RunAsClient(() => sid = WindowsIdentity.GetCurrent().User?.Value ?? "");
                if (sid == "")
                {
                    throw new UnauthorizedAccessException();
                }

                using var dispatchDeadline = CancellationTokenSource.CreateLinkedTokenSource(stoppingToken);
                dispatchDeadline.CancelAfter(DispatchTimeout);
                string reply;
                try
                {
                    var result = await Dispatch(sid, line, dispatchDeadline.Token);
                    reply = JsonSerializer.Serialize(new { success = true, data = result }, Wire.Json);
                }
                catch (Exception e)
                {
                    var timedOut = dispatchDeadline.IsCancellationRequested && !stoppingToken.IsCancellationRequested;
                    reply = JsonSerializer.Serialize(
                        new { success = false, data = (object?)null, error = ReplyError(e, timedOut) },
                        Wire.Json);
                }
                await writer.WriteLineAsync(reply);
            }
            catch (IOException)
            {
            }
            catch (OperationCanceledException)
            {
            }
            catch (Exception e)
            {
                log.LogWarning(e, "Local request failed");
            }
        }
    }

    // The client shows the reply's error as is. Our own messages are Chinese;
    // a dispatch that ran out of budget (`timedOut`, or an HTTP timeout, or a
    // wait on the session) surfaces a framework message, which is not.
    static string ReplyError(Exception e, bool timedOut) => e switch
    {
        TimeoutException => "本地服务响应超时",
        OperationCanceledException { InnerException: TimeoutException } => "本地服务响应超时",
        OperationCanceledException when timedOut => "本地服务响应超时",
        OperationCanceledException => "操作被取消",
        _ => e.Message,
    };

    static async Task<string> ReadRequestLine(StreamReader reader, CancellationToken ct)
    {
        var buffer = new StringBuilder();
        var chars = new char[1];
        for (var i = 0; i < MaxRequestChars; i++)
        {
            if (await reader.ReadAsync(chars, ct) == 0)
            {
                throw new IOException("Interrupted request");
            }
            if (chars[0] == '\n')
            {
                break;
            }
            buffer.Append(chars[0]);
        }
        if (buffer.Length >= MaxRequestChars)
        {
            throw new IOException("Request exceeds limit");
        }
        return buffer.ToString();
    }

    async Task<object?> Dispatch(string sid, string line, CancellationToken ct)
    {
        using var doc = ParseRequest(line);
        var request = doc.RootElement;
        var command = Arg(request, "command");
        var args = request.TryGetProperty("args", out var given) ? given : default;

        if (command == "status")
        {
            await gate.WaitAsync(ct);
            try
            {
                // A failed session has no owner any more, but its view still
                // belongs to the user it failed for; everyone else sees idle.
                var current = view;
                var failedOwn = owner == null && current.State == "FAILED" && failedOwner == sid;
                if (owner == null && current.State == "FAILED" && !failedOwn)
                {
                    current = SessionView.Idle;
                }
                var own = owner == sid || failedOwn;
                var visible = owner == null || own;
                return new
                {
                    state = visible ? current.State : "BUSY",
                    error = visible ? current.Error : "本机正在为其他 Windows 用户加速",
                    session_id = own ? current.SessionID : null,
                    game_id = own ? current.GameID : null,
                    region_id = own ? current.RegionID : null,
                    line_id = own ? current.LineID : null,
                    probe = own ? current.Probe : null,
                    lease_until = own ? current.LeaseUntil : null,
                    control_url = config.ControlUrl,
                    logged_in = users.ContainsKey(sid),
                    username = users.TryGetValue(sid, out var signedIn)
                        ? signedIn.User.GetProperty("username").GetString()
                        : null,
                    core_available = File.Exists(config.CorePath) && config.CoreSha256.Length == 64,
                };
            }
            finally
            {
                gate.Release();
            }
        }

        if (command == "login")
        {
            // Read before anything is allocated, so a malformed request
            // leaves no client behind.
            var username = Arg(args, "username");
            var password = Arg(args, "password");
            var api = new ApiClient(config.ControlUrl, config.AllowLocalHttp);
            try
            {
                var user = await api.Login(username, password, ct);
                var seed = Environment.MachineName + "|" + sid + "|" + user.GetProperty("id").GetString();
                var id = Convert.ToHexString(SHA256.HashData(Encoding.UTF8.GetBytes(seed))).ToLowerInvariant();
                var device = new
                {
                    id,
                    name = Environment.MachineName,
                    capabilities = new { tcp = true, udp = true, ipv4 = true, ipv6 = SocketSupportsIPv6() },
                };
                await api.Send("devices", HttpMethod.Post, device, ct: ct);
                await gate.WaitAsync(ct);
                try
                {
                    if (owner == sid)
                    {
                        throw new InvalidOperationException("请先停止加速再切换账号");
                    }
                    if (users.Remove(sid, out var old))
                    {
                        old.Dispose();
                    }
                    users[sid] = new Identity(api, user, id);
                }
                finally
                {
                    gate.Release();
                }
                return user;
            }
            catch
            {
                api.Dispose();
                throw;
            }
        }

        Identity identity;
        await gate.WaitAsync(ct);
        try
        {
            identity = users.TryGetValue(sid, out var signedIn)
                ? signedIn
                : throw new UnauthorizedAccessException("请先登录");
        }
        finally
        {
            gate.Release();
        }

        switch (command)
        {
            case "games":
                return await identity.Api.Send("games", ct: ct);

            case "lines":
            {
                var game = Uri.EscapeDataString(Arg(args, "game_id"));
                var region = Uri.EscapeDataString(Arg(args, "region_id"));
                return await identity.Api.Send("lines?game_id=" + game + "&region_id=" + region, ct: ct);
            }

            case "start":
            {
                // The arguments are read before the session is claimed: a
                // malformed start must not leave this user owning a session
                // that never runs.
                var gameID = Arg(args, "game_id");
                var regionID = Arg(args, "region_id");
                var lineID = Arg(args, "line_id");
                var input = new
                {
                    game_id = gameID,
                    region_id = regionID,
                    line_id = lineID,
                    device_id = identity.DeviceID,
                    egress_binding_id = (string?)null,
                };
                await gate.WaitAsync(ct);
                try
                {
                    if (owner != null)
                    {
                        throw new InvalidOperationException("已有加速会话在运行");
                    }
                    owner = sid;
                    failedOwner = null;
                    Update(_ => SessionView.Idle with
                    {
                        State = "PREPARING",
                        GameID = gameID,
                        RegionID = regionID,
                        LineID = lineID,
                    });
                    running = CancellationTokenSource.CreateLinkedTokenSource(serviceToken);
                    sessionTask = StartSession(identity, sid, input, running.Token);
                    return new { state = "PREPARING" };
                }
                finally
                {
                    gate.Release();
                }
            }

            case "stop":
            {
                var (owned, task) = await Claim(sid, ct);
                if (!owned)
                {
                    throw new UnauthorizedAccessException("该会话属于其他用户");
                }
                if (task != null)
                {
                    await task.WaitAsync(TimeSpan.FromSeconds(20), ct);
                }
                return new { state = view.State };
            }

            case "logout":
            {
                var (owned, task) = await Claim(sid, ct);
                if (owned && task != null)
                {
                    await task.WaitAsync(TimeSpan.FromSeconds(20), ct);
                }
                await identity.Api.Send("auth/logout", HttpMethod.Post, ct: ct);
                await gate.WaitAsync(ct);
                try
                {
                    users.Remove(sid);
                    identity.Dispose();
                }
                finally
                {
                    gate.Release();
                }
                return new { ok = true };
            }

            default:
                throw new ArgumentException("不支持的操作");
        }
    }

    // Reads the owner under the gate. If `sid` may act on the session (it owns
    // it, or there is none), cancels it and returns the task to wait for.
    async Task<(bool Owned, Task? Running)> Claim(string sid, CancellationToken ct)
    {
        CancellationTokenSource? session;
        Task? task;
        await gate.WaitAsync(ct);
        try
        {
            if (owner != null && owner != sid)
            {
                return (false, null);
            }
            if (owner != sid)
            {
                return (true, null);
            }
            session = running;
            task = sessionTask;
        }
        finally
        {
            gate.Release();
        }
        // Cancel after releasing the gate: the session's continuation can run
        // inline on Cancel, and its synchronous kernel stop takes seconds that
        // every other request's `status` would otherwise wait out.
        session?.Cancel();
        return (true, task);
    }

    static JsonDocument ParseRequest(string line)
    {
        JsonDocument doc;
        try
        {
            doc = JsonDocument.Parse(line);
        }
        catch (JsonException)
        {
            throw new MalformedRequestException("Malformed request: not valid JSON");
        }
        if (doc.RootElement.ValueKind != JsonValueKind.Object)
        {
            doc.Dispose();
            throw new MalformedRequestException("Malformed request: expected a JSON object");
        }
        return doc;
    }

    // Reads a string field from a request object. A missing args object, a
    // missing field and a field of the wrong type all mean a malformed request.
    static string Arg(JsonElement element, string name)
    {
        if (element.ValueKind == JsonValueKind.Object &&
            element.TryGetProperty(name, out var value) &&
            value.ValueKind == JsonValueKind.String)
        {
            return value.GetString()!;
        }
        throw new MalformedRequestException($"Malformed request: {name} must be a string");
    }

    async Task StartSession(Identity identity, string ownerSid, object input, CancellationToken ct)
    {
        var requestID = Guid.NewGuid().ToString("N");
        string? sessionID = null;
        try
        {
            JsonElement submitted = default;
            for (var attempt = 0; ; attempt++)
            {
                try
                {
                    submitted = await identity.Api.Send("sessions", HttpMethod.Post, input, requestID, ct);
                    break;
                }
                catch (Exception ex) when (attempt < 2 && Transient(ex, ct))
                {
                    await Task.Delay(1000, ct);
                }
            }

            sessionID = submitted.GetProperty("id").GetString();
            Update(v => v with { SessionID = sessionID });
            var deadline = DateTime.UtcNow.AddMinutes(3);
            Connection? conn = null;
            while (DateTime.UtcNow < deadline)
            {
                ct.ThrowIfCancellationRequested();
                JsonElement res;
                try
                {
                    res = await identity.Api.Send("sessions/" + sessionID, ct: ct);
                }
                catch (Exception ex) when (Transient(ex, ct))
                {
                    // Keep polling: the node may still finish within the deadline.
                    Update(v => v with { Error = "控制服务暂时不可达，正在重试" });
                    await Task.Delay(1500, ct);
                    continue;
                }
                var session = res.GetProperty("session");
                var current = session.GetProperty("state").GetString()!;
                var lease = identity.Api.LocalDeadline(session.GetProperty("lease_until").GetDateTime());
                Update(v => v with { State = current, Error = "", LeaseUntil = lease });
                if (current == "CLOSED" || current == "STOPPING")
                {
                    throw new InvalidOperationException(EndReason(session.GetProperty("end_reason").GetString()));
                }
                if (current == "CONNECTING")
                {
                    conn = res.GetProperty("connection").Deserialize<Connection>(Wire.Json);
                    break;
                }
                await Task.Delay(1500, ct);
            }
            if (conn == null)
            {
                throw new TimeoutException("节点准备超时");
            }

            await kernel!.Start(conn, ct);
            var first = await TunnelProbe.Run(conn, kernel.SocksPort, ct);
            Update(v => v with { State = "ACTIVE", Probe = first });
            var lastProbe = DateTime.UtcNow;
            var failedProbes = 0;
            while (!ct.IsCancellationRequested)
            {
                if (!kernel.Running)
                {
                    throw new InvalidOperationException(KernelStopped(kernel));
                }
                if (!LogonSessions.HasLogon(ownerSid))
                {
                    throw new OwnerSignedOutException();
                }
                if (DateTime.UtcNow - lastProbe > TimeSpan.FromSeconds(30))
                {
                    try
                    {
                        var measured = await TunnelProbe.Run(conn, kernel.SocksPort, ct);
                        Update(v => v with { State = "ACTIVE", Probe = measured });
                        failedProbes = 0;
                    }
                    catch (Exception ex) when (!ct.IsCancellationRequested)
                    {
                        // Report what the failed probe measured (UDP loss, a
                        // TCP sample) rather than the last good result.
                        var measured = (ex as TunnelProbeException)?.Result;
                        Update(v => v with { State = "DEGRADED", Error = "隧道探测失败", Probe = measured ?? v.Probe });
                        if (++failedProbes >= 3)
                        {
                            throw new IOException("隧道探测连续失败", ex);
                        }
                    }
                    lastProbe = DateTime.UtcNow;
                }
                var snapshot = view;
                try
                {
                    var renewed = await identity.Api.Send(
                        "sessions/" + sessionID + "/heartbeat",
                        HttpMethod.Post,
                        new { state = snapshot.State, probe = snapshot.Probe },
                        ct: ct);
                    var lease = identity.Api.LocalDeadline(renewed.GetProperty("lease_until").GetDateTime());
                    Update(v => v with { LeaseUntil = lease, Error = v.State == "DEGRADED" ? v.Error : "" });
                }
                catch (Exception ex) when (Transient(ex, ct))
                {
                    Update(v => v with { Error = "控制服务不可达；租约到期后本地隧道将停止" });
                }
                if (view.LeaseUntil is { } until && DateTime.UtcNow >= until)
                {
                    throw new TimeoutException("会话租约已过期");
                }
                await Task.Delay(TimeSpan.FromSeconds(10), ct);
            }
        }
        catch (OperationCanceledException) when (ct.IsCancellationRequested)
        {
            // Stop requested by the user or the service shutting down.
            Update(v => v with { State = "STOPPING" });
        }
        catch (OwnerSignedOutException e)
        {
            Update(v => v with { State = "STOPPING", Error = e.Message });
        }
        catch (Exception e)
        {
            // A request timeout is an OperationCanceledException too, but it is
            // a failure, not a stop, and must be reported as one.
            var message = e is OperationCanceledException ? "控制服务未在规定时间内响应" : e.Message;
            Update(v => v with { State = "FAILED", Error = message });
        }
        finally
        {
            kernel?.Stop();
            if (sessionID != null)
            {
                using var cleanup = new CancellationTokenSource(TimeSpan.FromSeconds(10));
                try
                {
                    await identity.Api.Send("sessions/" + sessionID + "/stop", HttpMethod.Post, ct: cleanup.Token);
                }
                catch
                {
                    Update(v => v with { Error = (v.Error == "" ? "" : v.Error + "；") + "节点授权将随租约到期而失效" });
                }
            }
            await gate.WaitAsync();
            try
            {
                var failed = view.State == "FAILED";
                owner = null;
                failedOwner = failed ? ownerSid : null;
                Update(v => v with
                {
                    State = failed ? "FAILED" : "IDLE",
                    Probe = failed ? v.Probe : null,
                    LeaseUntil = null,
                    SessionID = null,
                    GameID = null,
                    RegionID = null,
                    LineID = null,
                });
            }
            finally
            {
                gate.Release();
            }
        }
    }

    // A network failure or request timeout while the session itself was not
    // stopped; worth retrying until the lease or deadline runs out.
    static bool Transient(Exception ex, CancellationToken ct) =>
        !ct.IsCancellationRequested && (ex is HttpRequestException || ex is OperationCanceledException);

    // The control service ends sessions with short English reasons; the
    // client shows them to the player.
    static string EndReason(string? reason) => reason switch
    {
        null or "" => "会话已被控制服务结束",
        "lease or preparation timeout" => "节点准备或租约超时",
        "node task failed: prepare" => "节点准备转发失败",
        "node task failed: probe" => "线路探测未通过",
        "node task failed: revoke" => "节点回收失败",
        "runtime_failed" => "节点转发进程异常退出",
        "lease_expired" => "节点租约已过期",
        "requested" => "已按请求停止",
        "line terminated by operator" => "线路已被运营方下线",
        "validation complete" => "线路验证已完成",
        _ => "会话已结束：" + reason,
    };

    static string KernelStopped(MihomoKernel kernel)
    {
        var output = kernel.RecentOutput;
        return output.Length == 0
            ? "网络内核已停止"
            : "网络内核已停止。最近的内核输出：" + Environment.NewLine + output;
    }

    static bool SocketSupportsIPv6() => System.Net.NetworkInformation.NetworkInterface
        .GetAllNetworkInterfaces()
        .Any(n => n.GetIPProperties().UnicastAddresses.Any(a =>
            a.Address.AddressFamily == System.Net.Sockets.AddressFamily.InterNetworkV6 &&
            !a.Address.IsIPv6LinkLocal));

    public override async Task StopAsync(CancellationToken ct)
    {
        running?.Cancel();
        if (sessionTask != null)
        {
            try
            {
                await sessionTask.WaitAsync(TimeSpan.FromSeconds(15), ct);
            }
            catch
            {
            }
        }
        kernel?.Stop();
        await base.StopAsync(ct);
    }
}
