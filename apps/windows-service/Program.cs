using System.IO.Pipes;
using System.Security.AccessControl;
using System.Security.Principal;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using FlowLight.Core;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.DependencyInjection;
var builder = Host.CreateApplicationBuilder(args);
builder.Services.AddWindowsService(o => o.ServiceName = "FlowLightAccelerator");
builder.Services.AddHostedService<AcceleratorService>();
await builder.Build().RunAsync();

sealed class ServiceConfig
{
    public string ControlUrl { get; set; } = ""; public string CorePath { get; set; } = ""; public string CoreSha256 { get; set; } = "";
    public bool AllowLocalHttp { get; set; }
    public string DataDir { get; set; } = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.CommonApplicationData), "FlowLightAccelerator", "runtime");
}
sealed class Identity(ApiClient api, JsonElement user, string deviceId) : IDisposable
{
    public ApiClient Api { get; } = api; public JsonElement User { get; } = user; public string DeviceID { get; } = deviceId;
    public void Dispose() => Api.Dispose();
}
sealed class AcceleratorService : BackgroundService
{
    readonly Dictionary<string, Identity> users = []; readonly SemaphoreSlim gate = new(1, 1);
    ServiceConfig config = new(); MihomoKernel? kernel; string? owner, sessionID;
    string state = "IDLE", error = ""; ProbeResult? probe; CancellationTokenSource? running; Task? sessionTask;
    DateTime leaseUntil; CancellationToken serviceToken;
    protected override async Task ExecuteAsync(CancellationToken ct)
    {
        serviceToken = ct;
        var path = Environment.GetEnvironmentVariable("FLOWLIGHT_SERVICE_CONFIG") ?? Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.CommonApplicationData), "FlowLightAccelerator", "service.json");
        config = JsonSerializer.Deserialize<ServiceConfig>(await File.ReadAllTextAsync(path, ct), Wire.Json) ?? throw new InvalidOperationException("Service configuration missing");
        Directory.CreateDirectory(config.DataDir); kernel = new MihomoKernel(config.CorePath, config.CoreSha256, config.DataDir);
        while (!ct.IsCancellationRequested)
        {
            var security = new PipeSecurity();
            security.AddAccessRule(new PipeAccessRule(new SecurityIdentifier(WellKnownSidType.NetworkSid, null), PipeAccessRights.FullControl, AccessControlType.Deny));
            security.AddAccessRule(new PipeAccessRule(new SecurityIdentifier(WellKnownSidType.AuthenticatedUserSid, null), PipeAccessRights.ReadWrite, AccessControlType.Allow));
            security.AddAccessRule(new PipeAccessRule(new SecurityIdentifier(WellKnownSidType.LocalSystemSid, null), PipeAccessRights.FullControl, AccessControlType.Allow));
            var serviceSid = WindowsIdentity.GetCurrent().User ?? throw new InvalidOperationException("Service identity unavailable");
            security.AddAccessRule(new PipeAccessRule(serviceSid, PipeAccessRights.FullControl, AccessControlType.Allow));
            var pipe = NamedPipeServerStreamAcl.Create(Wire.PipeName, PipeDirection.InOut, 8, PipeTransmissionMode.Byte, PipeOptions.Asynchronous, 65536, 65536, security);
            try { await pipe.WaitForConnectionAsync(ct); _ = Handle(pipe, ct); } catch { pipe.Dispose(); if (ct.IsCancellationRequested) break; throw; }
        }
    }
    async Task Handle(NamedPipeServerStream pipe, CancellationToken stoppingToken)
    {
        using (pipe) using (var timeout = CancellationTokenSource.CreateLinkedTokenSource(stoppingToken))
        {
            timeout.CancelAfter(TimeSpan.FromSeconds(30)); var ct = timeout.Token;
            try
            {
                using var reader = new StreamReader(pipe, leaveOpen: true); using var writer = new StreamWriter(pipe, leaveOpen: true) { AutoFlush = true };
                var buffer = new StringBuilder(); var chars = new char[1];
                for (var i = 0; i < 65536; i++) { if (await reader.ReadAsync(chars, ct) == 0) throw new IOException("Interrupted request"); if (chars[0] == '\n') break; buffer.Append(chars[0]); }
                if (buffer.Length >= 65536) throw new IOException("Request exceeds limit");
                string sid = ""; pipe.RunAsClient(() => sid = WindowsIdentity.GetCurrent().User?.Value ?? "");
                if (sid == "") throw new UnauthorizedAccessException();
                using var doc = JsonDocument.Parse(buffer.ToString());
                try { var result = await Dispatch(sid, doc.RootElement, ct); await writer.WriteLineAsync(JsonSerializer.Serialize(new { success = true, data = result }, Wire.Json)); }
                catch (Exception e) { await writer.WriteLineAsync(JsonSerializer.Serialize(new { success = false, data = (object?)null, error = e.Message }, Wire.Json)); }
            }
            catch (IOException) { }
            catch (OperationCanceledException) { }
            catch (JsonException) { }
        }
    }
    async Task<object?> Dispatch(string sid, JsonElement request, CancellationToken ct)
    {
        var command = request.GetProperty("command").GetString(); var args = request.GetProperty("args");
        if (command == "status")
        {
            await gate.WaitAsync(ct);
            try { return new { state = owner == null || owner == sid ? state : "BUSY", error = owner == null || owner == sid ? error : "Another Windows user owns this session", session_id = owner == sid ? sessionID : null, probe = owner == sid ? probe : null, lease_until = owner == sid ? leaseUntil : (DateTime?)null, control_url = config.ControlUrl, logged_in = users.ContainsKey(sid), username = users.TryGetValue(sid, out var signedIn) ? signedIn.User.GetProperty("username").GetString() : null, core_available = File.Exists(config.CorePath) && config.CoreSha256.Length == 64 }; }
            finally { gate.Release(); }
        }
        if (command == "login")
        {
            var api = new ApiClient(config.ControlUrl, config.AllowLocalHttp);
            try
            {
                var user = await api.Login(args.GetProperty("username").GetString()!, args.GetProperty("password").GetString()!, ct);
                var id = Convert.ToHexString(SHA256.HashData(Encoding.UTF8.GetBytes(Environment.MachineName + "|" + sid + "|" + user.GetProperty("id").GetString()))).ToLowerInvariant();
                await api.Send("devices", HttpMethod.Post, new { id, name = Environment.MachineName, capabilities = new { tcp = true, udp = true, ipv4 = true, ipv6 = SocketSupportsIPv6() } }, ct: ct);
                await gate.WaitAsync(ct);
                try { if (owner == sid) throw new InvalidOperationException("Stop the session before changing accounts"); if (users.Remove(sid, out var old)) old.Dispose(); users[sid] = new Identity(api, user, id); } finally { gate.Release(); }
                return user;
            }
            catch { api.Dispose(); throw; }
        }
        Identity identity; await gate.WaitAsync(ct);
        try { identity = users.TryGetValue(sid, out var i) ? i : throw new UnauthorizedAccessException("Please sign in first"); } finally { gate.Release(); }
        switch (command)
        {
            case "games": return await identity.Api.Send("games", ct: ct);
            case "lines": return await identity.Api.Send("lines?game_id=" + Uri.EscapeDataString(args.GetProperty("game_id").GetString()!) + "&region_id=" + Uri.EscapeDataString(args.GetProperty("region_id").GetString()!), ct: ct);
            case "start":
                await gate.WaitAsync(ct);
                try
                {
                    if (owner != null) throw new InvalidOperationException("A session is already running");
                    owner = sid; state = "PREPARING"; error = ""; probe = null; running = CancellationTokenSource.CreateLinkedTokenSource(serviceToken);
                    var input = new { game_id = args.GetProperty("game_id").GetString(), region_id = args.GetProperty("region_id").GetString(), line_id = args.GetProperty("line_id").GetString(), device_id = identity.DeviceID, egress_binding_id = (string?)null };
                    sessionTask = StartSession(identity, input, running.Token); return new { state };
                }
                finally { gate.Release(); }
            case "stop":
                if (owner != sid && owner != null) throw new UnauthorizedAccessException("Session belongs to another user");
                running?.Cancel(); if (sessionTask != null) await sessionTask.WaitAsync(TimeSpan.FromSeconds(20), ct); return new { state };
            case "logout":
                if (owner == sid) { running?.Cancel(); if (sessionTask != null) await sessionTask.WaitAsync(TimeSpan.FromSeconds(20), ct); }
                await identity.Api.Send("auth/logout", HttpMethod.Post, ct: ct);
                await gate.WaitAsync(ct); try { users.Remove(sid); identity.Dispose(); } finally { gate.Release(); }
                return new { ok = true };
            default: throw new ArgumentException("Unsupported operation");
        }
    }
    async Task StartSession(Identity identity, object input, CancellationToken ct)
    {
        var requestID = Guid.NewGuid().ToString("N");
        try
        {
            JsonElement submitted = default;
            for (var attempt = 0; ; attempt++) { try { submitted = await identity.Api.Send("sessions", HttpMethod.Post, input, requestID, ct); break; } catch (Exception ex) when (attempt < 2 && !ct.IsCancellationRequested && (ex is HttpRequestException || ex is OperationCanceledException)) { await Task.Delay(1000, ct); } }
            sessionID = submitted.GetProperty("id").GetString(); var deadline = DateTime.UtcNow.AddMinutes(3); Connection? conn = null;
            while (DateTime.UtcNow < deadline)
            {
                ct.ThrowIfCancellationRequested(); var res = await identity.Api.Send("sessions/" + sessionID, ct: ct); var session = res.GetProperty("session");
                state = session.GetProperty("state").GetString()!; leaseUntil = identity.Api.LocalDeadline(session.GetProperty("lease_until").GetDateTime());
                if (state == "CLOSED" || state == "STOPPING") throw new InvalidOperationException(session.GetProperty("end_reason").GetString());
                if (state == "CONNECTING") { conn = res.GetProperty("connection").Deserialize<Connection>(Wire.Json); break; }
                await Task.Delay(1500, ct);
            }
            if (conn == null) throw new TimeoutException("Node preparation timed out");
            await kernel!.Start(conn, ct); probe = await TunnelProbe.Run(conn, kernel.SocksPort, ct); state = "ACTIVE"; var lastProbe = DateTime.UtcNow; var failedProbes = 0;
            while (!ct.IsCancellationRequested)
            {
                if (!kernel.Running) throw new InvalidOperationException("Network kernel stopped");
                if (DateTime.UtcNow - lastProbe > TimeSpan.FromSeconds(30))
                {
                    try { probe = await TunnelProbe.Run(conn, kernel.SocksPort, ct); state = "ACTIVE"; failedProbes = 0; }
                    catch (Exception ex) when (ex is not OperationCanceledException) { state = "DEGRADED"; error = "Tunnel probe failed"; if (++failedProbes >= 3) throw new IOException("Tunnel recovery probes failed", ex); }
                    lastProbe = DateTime.UtcNow;
                }
                try { var renewed = await identity.Api.Send("sessions/" + sessionID + "/heartbeat", HttpMethod.Post, new { state, probe }, ct: ct); leaseUntil = identity.Api.LocalDeadline(renewed.GetProperty("lease_until").GetDateTime()); error = ""; }
                catch (Exception ex) when (!ct.IsCancellationRequested && (ex is HttpRequestException || ex is OperationCanceledException)) { error = "Control service unreachable; local tunnel stops at lease expiry"; }
                if (DateTime.UtcNow >= leaseUntil) throw new TimeoutException("Session lease expired");
                await Task.Delay(TimeSpan.FromSeconds(10), ct);
            }
        }
        catch (OperationCanceledException) { state = "STOPPING"; }
        catch (Exception e) { state = "FAILED"; error = e.Message; }
        finally
        {
            kernel?.Stop();
            if (sessionID != null) { using var cleanup = new CancellationTokenSource(TimeSpan.FromSeconds(10)); try { await identity.Api.Send("sessions/" + sessionID + "/stop", HttpMethod.Post, ct: cleanup.Token); } catch { error += "; node authorization expires with its lease"; } }
            await gate.WaitAsync(); try { owner = null; sessionID = null; if (state != "FAILED") state = "IDLE"; } finally { gate.Release(); }
        }
    }
    static bool SocketSupportsIPv6() => System.Net.NetworkInformation.NetworkInterface.GetAllNetworkInterfaces().Any(n => n.GetIPProperties().UnicastAddresses.Any(a => a.Address.AddressFamily == System.Net.Sockets.AddressFamily.InterNetworkV6 && !a.Address.IsIPv6LinkLocal));
    public override async Task StopAsync(CancellationToken ct)
    {
        running?.Cancel(); if (sessionTask != null) { try { await sessionTask.WaitAsync(TimeSpan.FromSeconds(15), ct); } catch { } }
        kernel?.Stop(); await base.StopAsync(ct);
    }
}
