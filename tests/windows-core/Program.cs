using System.Diagnostics;
using System.IO.Pipes;
using System.Net;
using System.Net.Sockets;
using System.Reflection;
using System.Text;
using System.Text.Json;
using FlowLight.Core;

if (args.Contains("--owned-child"))
{
    Thread.Sleep(Timeout.Infinite);
    return;
}
if (Environment.GetEnvironmentVariable(FakeKernel.ModeVariable) is { } fakeMode && args.Length >= 2 && args[^2] == "-f")
{
    FakeKernel.Run(fakeMode, args);
    return;
}

static void Check(bool value, string message)
{
    if (!value)
    {
        throw new Exception(message);
    }
}

try
{
    using var bad = new ApiClient("http://public.example");
    throw new Exception("insecure API accepted");
}
catch (ArgumentException)
{
}

var profile = new GameProfile(["VALORANT-Win64-Shipping.exe"], [], [], [], [], true);
var conn = new Connection(
    "node.example",
    20000,
    "2022-blake3-aes-128-gcm",
    "test-only",
    true,
    profile,
    "https://probe.example/probe",
    "probe.example:9081",
    null);
using (var data = JsonDocument.Parse(JsonSerializer.Serialize(conn, Wire.Json)))
{
    Check(data.RootElement.GetProperty("probe_url").GetString() == conn.ProbeUrl, "contract casing changed");
    Check(data.RootElement.GetProperty("profile").GetProperty("udp").GetBoolean(), "UDP requirement lost");
}

try
{
    MihomoKernel.BuildConfig(conn with { Profile = profile with { Processes = ["bad,rule"] } }, 10900);
    throw new Exception("rule injection accepted");
}
catch (ArgumentException)
{
}

using (var kernelConfig = JsonDocument.Parse(JsonSerializer.Serialize(MihomoKernel.BuildConfig(conn, 10900))))
{
    var root = kernelConfig.RootElement;
    var rules = root.GetProperty("rules").EnumerateArray().Select(r => r.GetString()).ToArray();
    var inbound = root.GetProperty("listeners")[0];
    Check(
        rules[0] == $"IN-NAME,{MihomoKernel.ProbeInbound},FLOWLIGHT" &&
        Array.IndexOf(rules, "PROCESS-NAME,FlowLight.Service.exe,DIRECT") > 0,
        "tunnel probe can match the service DIRECT rule");
    Check(
        inbound.GetProperty("name").GetString() == MihomoKernel.ProbeInbound &&
        inbound.GetProperty("proxy").GetString() == "FLOWLIGHT" &&
        inbound.GetProperty("port").GetInt32() == 10900 &&
        inbound.GetProperty("listen").GetString() == "127.0.0.1",
        "probe inbound not pinned to the tunnel");
    Check(
        !root.TryGetProperty("mixed-port", out _) && !root.GetProperty("ipv6").GetBoolean(),
        "unverified IPv6 or shared local proxy enabled");
}

var tornRoot = Directory.CreateTempSubdirectory("flowlight-kernel-").FullName;
File.WriteAllText(Path.Combine(tornRoot, "owned-kernel.json"), "{\"pid\":");
using (new MihomoKernel(Path.Combine(tornRoot, "mihomo.exe"), new string('0', 64), tornRoot))
{
}
Check(!File.Exists(Path.Combine(tornRoot, "owned-kernel.json")), "torn kernel journal not discarded");
Directory.Delete(tornRoot, true);

// Token rotation, idempotency and transient error classification.
var port = FreeTcpPort();
using (var listener = new HttpListener())
{
    listener.Prefixes.Add($"http://127.0.0.1:{port}/");
    listener.Start();
    var server = Task.Run(async () =>
    {
        for (var i = 0; i < 6; i++)
        {
            var ctx = await listener.GetContextAsync();
            var path = ctx.Request.Url!.AbsolutePath;
            string payload;
            if (i == 0)
            {
                Check(path.EndsWith("/auth/login"), "login path");
                payload = """{"access_token":"first","refresh_token":"refresh","user":{"id":"owner"}}""";
            }
            else if (i == 1)
            {
                Check(ctx.Request.Headers["Authorization"] == "Bearer first", "access token missing");
                ctx.Response.StatusCode = 401;
                payload = """{"error":"expired"}""";
            }
            else if (i == 2)
            {
                Check(path.EndsWith("/auth/refresh"), "refresh path");
                payload = """{"access_token":"second","refresh_token":"rotated","user":{"id":"owner"}}""";
            }
            else if (i == 3)
            {
                Check(ctx.Request.Headers["Authorization"] == "Bearer second", "refresh not applied");
                payload = "[]";
            }
            else if (i == 4)
            {
                Check(path.EndsWith("/sessions"), "session path");
                Check(ctx.Request.Headers["Idempotency-Key"] == "request-stable-1", "idempotency key missing");
                using var reader = new StreamReader(ctx.Request.InputStream);
                var body = await reader.ReadToEndAsync();
                Check(body.Contains("game_id"), "session contract changed");
                payload = """{"id":"session-1"}""";
            }
            else
            {
                ctx.Response.StatusCode = 503;
                payload = "gateway unavailable";
            }
            var bytes = Encoding.UTF8.GetBytes(payload);
            ctx.Response.ContentType = "application/json";
            await ctx.Response.OutputStream.WriteAsync(bytes);
            ctx.Response.Close();
        }
    });
    using (var api = new ApiClient($"http://127.0.0.1:{port}", true))
    {
        using var timeout = new CancellationTokenSource(TimeSpan.FromSeconds(15));
        await api.Login("tester", "fixture-password", timeout.Token);
        var games = await api.Send("games", ct: timeout.Token);
        Check(games.ValueKind == JsonValueKind.Array, "catalog decode");
        var session = await api.Send(
            "sessions",
            HttpMethod.Post,
            new { game_id = "valorant" },
            "request-stable-1",
            timeout.Token);
        Check(session.GetProperty("id").GetString() == "session-1", "session decode");
        try
        {
            await api.Send("games", ct: timeout.Token);
            throw new Exception("503 response accepted");
        }
        catch (HttpRequestException e)
        {
            Check(e.StatusCode == HttpStatusCode.ServiceUnavailable, "503 not classified as transient");
        }
    }
    await server.WaitAsync(TimeSpan.FromSeconds(15));
}

// UDP accounting: the verdict and the loss come from five samples.
for (var replies = 0; replies <= TunnelProbe.UdpSamples; replies++)
{
    var accounted = new ProbeResult();
    TunnelProbe.ApplyUdp(accounted, replies);
    Check(accounted.Udp == (replies >= 4), $"UDP verdict wrong for {replies}/5 replies");
    Check(Math.Abs(accounted.Loss - (5 - replies) / 5.0) < 1e-9, $"UDP loss wrong for {replies}/5 replies");
}

// SOCKS5 UDP framing and nonce verification against a local relay that
// echoes datagrams, dropping or corrupting the ones a scenario picks.
var shortReply = TimeSpan.FromMilliseconds(400);
await using (var socks = new FakeSocks())
{
    using var timeout = new CancellationTokenSource(TimeSpan.FromSeconds(30));
    Task<int> Sample() => TunnelProbe.ProbeUdp(socks.Port, conn.ProbeUdp, shortReply, timeout.Token);

    Check(await Sample() == 5, "clean UDP relay lost samples");
    Check(socks.Datagrams == 5, "UDP probe did not send five samples");
    Check(socks.DistinctNonces == 5, "UDP samples reuse a nonce");
    Check(socks.LastTarget == "probe.example:9081", "UDP probe addressed the wrong target");

    socks.Reply = (index, packet) => index == 2 ? null : packet;
    Check(await Sample() == 4, "single lost datagram miscounted");

    socks.Reply = (index, packet) => index is 1 or 3 ? null : packet;
    Check(await Sample() == 3, "two lost datagrams miscounted");

    // A reply whose payload differs by one byte is not a round trip.
    socks.Reply = (index, packet) =>
    {
        if (index is not (0 or 4))
        {
            return packet;
        }
        var corrupted = (byte[])packet.Clone();
        corrupted[^1] ^= 0xFF;
        return corrupted;
    };
    Check(await Sample() == 3, "corrupted nonce accepted");

    // Relays may answer with an IPv4 source address instead of the domain.
    socks.Reply = (_, packet) => FakeSocks.WithIPv4Header(packet);
    Check(await Sample() == 5, "IPv4-addressed UDP reply rejected");

    var payload = "FLA1"u8.ToArray().Concat(new byte[24]).ToArray();
    Check(!TunnelProbe.IsEcho([0, 0, 0, 1, 127, 0, 0, 1, 0], payload), "truncated UDP reply accepted");
    Check(!TunnelProbe.IsEcho([0, 0, 1, 1, 127, 0, 0, 1, 0, 53, .. payload], payload), "fragmented UDP reply accepted");
    Check(TunnelProbe.IsEcho([0, 0, 0, 1, 127, 0, 0, 1, 0, 53, .. payload], payload), "valid UDP reply rejected");
    Check(
        !TunnelProbe.IsEcho([0, 0, 0, 1, 127, 0, 0, 1, 0, 53, .. payload, 0], payload),
        "UDP reply with trailing data accepted");
}

// The whole probe: HTTP samples on one warm SOCKS connection, then UDP.
var probePort = FreeTcpPort();
using (var probeServer = new HttpListener())
{
    probeServer.Prefixes.Add($"http://127.0.0.1:{probePort}/");
    probeServer.Start();
    using var serving = new CancellationTokenSource();
    var serve = Task.Run(async () =>
    {
        while (!serving.IsCancellationRequested)
        {
            HttpListenerContext ctx;
            try
            {
                ctx = await probeServer.GetContextAsync();
            }
            catch (Exception) when (serving.IsCancellationRequested)
            {
                return;
            }
            var bytes = """{"ip":"203.0.113.9"}"""u8.ToArray();
            ctx.Response.ContentType = "application/json";
            await ctx.Response.OutputStream.WriteAsync(bytes);
            ctx.Response.Close();
        }
    });

    // Every tunnel connection costs 400 ms to set up. A sample that paid for
    // it would show up in the RTT or the jitter.
    await using (var socks = new FakeSocks { ConnectDelay = TimeSpan.FromMilliseconds(400) })
    {
        using var timeout = new CancellationTokenSource(TimeSpan.FromSeconds(60));
        var probeConn = conn with { ProbeUrl = $"http://127.0.0.1:{probePort}/probe", Udp = false };
        var tcpOnly = await TunnelProbe.Run(probeConn, socks.Port, shortReply, timeout.Token);
        Check(tcpOnly.Tcp && !tcpOnly.Udp && tcpOnly.Samples == 3, "TCP probe result incomplete");
        Check(tcpOnly.Loss == 0, "TCP-only probe reports loss");
        Check(tcpOnly.ExitIp == "203.0.113.9", "probe exit address lost");
        Check(socks.Connects == 1, $"probe opened {socks.Connects} tunnel connections instead of reusing one");
        Check(
            tcpOnly.RttMs < 200 && tcpOnly.JitterMs < 200,
            $"probe timed connection setup (rtt {tcpOnly.RttMs:0} ms, jitter {tcpOnly.JitterMs:0} ms)");

        socks.Reply = (index, packet) => index == 2 ? null : packet;
        var tolerated = await TunnelProbe.Run(probeConn with { Udp = true }, socks.Port, shortReply, timeout.Token);
        Check(tolerated.Udp && Math.Abs(tolerated.Loss - 0.2) < 1e-9, "one lost UDP sample failed the probe");

        socks.Reply = (index, packet) => index is 0 or 3 ? null : packet;
        try
        {
            await TunnelProbe.Run(probeConn with { Udp = true }, socks.Port, shortReply, timeout.Token);
            throw new Exception("probe with 3/5 UDP replies reported success");
        }
        catch (TunnelProbeException e)
        {
            Check(e.Result.Tcp && !e.Result.Udp, "3/5 UDP replies not reported as a UDP failure");
            Check(Math.Abs(e.Result.Loss - 0.4) < 1e-9, "UDP failure does not carry the measured loss");
        }
    }
    serving.Cancel();
    probeServer.Stop();
    await serve.WaitAsync(TimeSpan.FromSeconds(5));
}

if (OperatingSystem.IsWindows())
{
    var start = new ProcessStartInfo(Environment.ProcessPath!) { UseShellExecute = false, CreateNoWindow = true };
    var host = Path.GetFileNameWithoutExtension(Environment.ProcessPath);
    if (string.Equals(host, "dotnet", StringComparison.OrdinalIgnoreCase))
    {
        start.ArgumentList.Add(Assembly.GetExecutingAssembly().Location);
    }
    start.ArgumentList.Add("--owned-child");
    using var child = Process.Start(start)!;
    try
    {
        using (var job = new OwnedProcessJob())
        {
            job.Add(child);
        }
        Check(child.WaitForExit(5000), "owned network process survived job disposal");
    }
    finally
    {
        if (!child.HasExited)
        {
            child.Kill(true);
        }
    }
}

// Kernel start failures, with this program standing in for mihomo.
var kernelChecks = "skipped (no apphost)";
if (OperatingSystem.IsWindows() && FakeKernel.Binary() is { } fakeBinary)
{
    var hash = Convert.ToHexString(System.Security.Cryptography.SHA256.HashData(File.ReadAllBytes(fakeBinary)));
    using var timeout = new CancellationTokenSource(TimeSpan.FromSeconds(90));

    async Task<(Exception Error, string Root)> FailedStart(string mode, string expectedHash, int attempts = 50)
    {
        var root = Directory.CreateTempSubdirectory("flowlight-kernel-").FullName;
        Environment.SetEnvironmentVariable(FakeKernel.ModeVariable, mode);
        try
        {
            using var kernel = new MihomoKernel(fakeBinary, expectedHash, root) { ReadinessAttempts = attempts };
            try
            {
                await kernel.Start(conn, timeout.Token);
            }
            catch (Exception e)
            {
                Check(!kernel.Running, $"{mode}: kernel still running after a failed start");
                Check(
                    !File.Exists(Path.Combine(root, "owned-kernel.json")),
                    $"{mode}: journal left after a failed start");
                Check(
                    Directory.GetFiles(root, "session-*.json").Length == 0,
                    $"{mode}: plaintext session config left behind");
                return (e, root);
            }
            throw new Exception($"{mode}: fake kernel start succeeded");
        }
        finally
        {
            Environment.SetEnvironmentVariable(FakeKernel.ModeVariable, null);
        }
    }

    string KernelLog(string root) => File.ReadAllText(Path.Combine(root, MihomoKernel.LogFileName));

    var (mismatch, mismatchRoot) = await FailedStart("exit", new string('0', 64));
    Check(
        mismatch is InvalidOperationException && mismatch.Message.StartsWith("网络内核校验失败"),
        "checksum mismatch not reported");
    Check(mismatch.Message.Contains(hash), "checksum mismatch hides the actual hash");
    Check(KernelLog(mismatchRoot).Contains(hash), "checksum mismatch not in kernel.log");

    var (rejected, rejectedRoot) = await FailedStart("reject", hash);
    Check(rejected.Message.StartsWith("网络内核配置校验失败"), "config rejection not reported");
    Check(rejected.Message.Contains("fake-kernel: configuration rejected"), "config rejection hides kernel output");
    Check(
        KernelLog(rejectedRoot).Contains("fake-kernel: configuration rejected"),
        "config rejection not in kernel.log");

    var (exited, exitedRoot) = await FailedStart("exit", hash);
    Check(exited.Message.StartsWith("网络内核已退出，请检查服务权限"), "kernel exit not reported");
    Check(exited.Message.Contains("fake-kernel: cannot open TUN"), "kernel exit hides kernel output");
    Check(
        exited.Message.Contains("[redacted]") && !exited.Message.Contains(conn.Password),
        "tunnel key leaked into diagnostics");
    Check(!KernelLog(exitedRoot).Contains(conn.Password), "tunnel key leaked into kernel.log");

    // Validation and the kernel each print their pid; the last one is the
    // kernel that never opened its listener.
    var (hung, hungRoot) = await FailedStart("hang", hash, attempts: 3);
    Check(
        hung is TimeoutException && hung.Message.StartsWith("网络内核监听未就绪"),
        "listener timeout not reported");
    var pids = hung.Message.Split("fake-kernel pid=").Skip(1).Select(p => int.Parse(p.Split(' ')[0])).ToArray();
    Check(pids.Length == 2, "listener timeout hides kernel output");
    var hungPid = pids[^1];
    try
    {
        using var survivor = Process.GetProcessById(hungPid);
        Check(survivor.WaitForExit(5000), "kernel left running after the listener timed out");
    }
    catch (ArgumentException)
    {
    }

    var (flooded, floodRoot) = await FailedStart("flood", hash);
    Check(flooded.Message.Contains("fake-kernel: flood end"), "latest kernel output missing from diagnostics");
    Check(flooded.Message.Length < MihomoKernel.OutputTailChars + 512, "kernel diagnostics are not bounded");
    var log = new FileInfo(Path.Combine(floodRoot, MihomoKernel.LogFileName));
    Check(log.Length <= MihomoKernel.LogFileLimitBytes, "kernel.log is not bounded");
    Check(File.Exists(log.FullName + ".1"), "kernel.log was not rotated");

    foreach (var root in new[] { mismatchRoot, rejectedRoot, exitedRoot, hungRoot, floodRoot })
    {
        Directory.Delete(root, true);
    }
    kernelChecks = "verified";
}

if (OperatingSystem.IsWindows())
{
    // Busy-versus-missing detection must see a live pipe without connecting.
    var pipeName = "FlowLight.Checks." + Guid.NewGuid().ToString("N");
    Check(!Wire.PipeExists(pipeName), "absent pipe reported as present");

    // A pipe nobody serves fails at once, not after the connect timeout.
    var watch = Stopwatch.StartNew();
    try
    {
        await Wire.CallAsync(pipeName, "status", null, CancellationToken.None);
        throw new Exception("call to an absent pipe succeeded");
    }
    catch (ServiceUnavailableException e)
    {
        Check(!e.Busy, "absent pipe reported as busy");
    }
    Check(watch.ElapsedMilliseconds < 1500, $"absent pipe took {watch.ElapsedMilliseconds} ms to fail");

    using var server = new NamedPipeServerStream(
        pipeName, PipeDirection.InOut, 1, PipeTransmissionMode.Byte, PipeOptions.Asynchronous);
    var waiting = server.WaitForConnectionAsync();
    Check(Wire.PipeExists(pipeName), "running pipe server not detected");
    Check(!waiting.IsCompleted, "pipe detection consumed a server instance");

    // A pipe served by this process is not the installed service, whether or
    // not that service is registered on this machine.
    if (Environment.GetEnvironmentVariable("FLOWLIGHT_DEV_PIPE") != "1")
    {
        using var client = new NamedPipeClientStream(".", pipeName, PipeDirection.InOut, PipeOptions.Asynchronous);
        await client.ConnectAsync(5000);
        await waiting;
        try
        {
            Wire.VerifyServer(client);
            throw new Exception("pipe served by the checks accepted as the service");
        }
        catch (IOException e)
        {
            Check(e.Message.Contains("本地服务"), "verification failure does not name the local service");
        }
    }

    // Service identity comes from the Service Control Manager, which answers
    // without elevation. EventLog always runs, hosted by svchost.
    var eventLog = Wire.QueryService("EventLog");
    Check(eventLog is { ProcessId: > 0 }, "EventLog service status not read from the SCM");
    Check(
        eventLog!.Value.BinaryPath.Contains("svchost.exe", StringComparison.OrdinalIgnoreCase),
        $"EventLog binary path unexpected: {eventLog.Value.BinaryPath}");
    using (var host = Process.GetProcessById((int)eventLog.Value.ProcessId))
    {
        Check(
            host.ProcessName.Equals("svchost", StringComparison.OrdinalIgnoreCase),
            $"EventLog pid {eventLog.Value.ProcessId} is {host.ProcessName}, not svchost");
    }
    Check(
        Wire.QueryService("FlowLight.Checks.NoSuchService." + Guid.NewGuid().ToString("N")) is null,
        "unregistered service not reported as absent");

    // The registered binary path, as `sc.exe qc` shows it, must be the
    // release location: quoted or not, with or without arguments.
    var release = Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.ProgramFiles),
        "FlowLight Accelerator",
        "service",
        "FlowLight.Service.exe");
    Check(Wire.IsExpectedServicePath(release), "unquoted release path rejected");
    Check(Wire.IsExpectedServicePath($"\"{release}\""), "quoted release path rejected");
    Check(Wire.IsExpectedServicePath($"\"{release}\" --service --verbose"), "release path with arguments rejected");
    Check(Wire.IsExpectedServicePath($"  \"{release.ToUpperInvariant()}\"  "), "release path compared by case");
    Check(!Wire.IsExpectedServicePath(""), "empty service path accepted");
    Check(!Wire.IsExpectedServicePath("\""), "unterminated quote accepted");
    Check(!Wire.IsExpectedServicePath("FlowLight.Service.exe"), "relative service path accepted");
    var profileCopy = Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.UserProfile),
        "FlowLight Accelerator",
        "service",
        "FlowLight.Service.exe");
    Check(!Wire.IsExpectedServicePath($"\"{profileCopy}\""), "service path under the user profile accepted");
    var tempCopy = Path.Combine(Path.GetTempPath(), "FlowLight.Service.exe");
    Check(!Wire.IsExpectedServicePath($"\"{tempCopy}\""), "service path in a temp directory accepted");
    var otherFile = Path.Combine(Path.GetDirectoryName(release)!, "FlowLight.Client.exe");
    Check(!Wire.IsExpectedServicePath($"\"{otherFile}\""), "different file name accepted");
    Check(!Wire.IsExpectedServicePath($"\"{release}.bak\""), "file name with a suffix accepted");
    var traversal = Path.Combine(Path.GetDirectoryName(release)!, "..", "..", "..", "Users", "FlowLight.Service.exe");
    Check(!Wire.IsExpectedServicePath($"\"{traversal}\""), "path leaving the release directory accepted");
}

Console.WriteLine(
    "PASS: HTTPS restriction, wire contract, rule validation, tunnel-pinned probe inbound, torn kernel journal, " +
    "token rotation, stable idempotency, transient gateway errors, UDP 4-of-5 sampling and loss, UDP nonce " +
    "framing, warm-connection TCP probe, Windows child lifetime, kernel failure diagnostics and cleanup " +
    $"({kernelChecks}), absent-pipe fast fail, foreign pipe server rejected, SCM service lookup, release path " +
    "check.");

static int FreeTcpPort()
{
    var probe = new TcpListener(IPAddress.Loopback, 0);
    probe.Start();
    var port = ((IPEndPoint)probe.LocalEndpoint).Port;
    probe.Stop();
    return port;
}

// Stands in for mihomo when this program is started with "-f <config>".
static class FakeKernel
{
    public const string ModeVariable = "FLOWLIGHT_FAKE_KERNEL";

    // The apphost next to the assembly, so the kernel's arguments reach Main.
    public static string? Binary()
    {
        var path = Environment.ProcessPath;
        if (path is not null &&
            !string.Equals(Path.GetFileNameWithoutExtension(path), "dotnet", StringComparison.OrdinalIgnoreCase))
        {
            return path;
        }
        var apphost = Path.ChangeExtension(Assembly.GetExecutingAssembly().Location, ".exe");
        return File.Exists(apphost) ? apphost : null;
    }

    public static void Run(string mode, string[] args)
    {
        var validating = args.Contains("-t");
        Console.WriteLine($"fake-kernel pid={Environment.ProcessId} mode={mode}");
        Console.Out.Flush();
        if (validating)
        {
            if (mode == "reject")
            {
                Console.Error.WriteLine("fake-kernel: configuration rejected");
                Environment.Exit(1);
            }
            Environment.Exit(0);
        }
        switch (mode)
        {
            case "exit":
                Console.Error.WriteLine("fake-kernel: cannot open TUN with config " + File.ReadAllText(args[^1]));
                Environment.Exit(2);
                break;
            case "hang":
                Thread.Sleep(Timeout.Infinite);
                break;
            case "flood":
                var line = new string('x', 99);
                for (var i = 0; i < 3000; i++)
                {
                    Console.WriteLine(line);
                }
                Console.WriteLine("fake-kernel: flood end");
                Console.Out.Flush();
                Environment.Exit(3);
                break;
        }
        Environment.Exit(0);
    }
}

// A minimal SOCKS5 server: CONNECT relays TCP, UDP ASSOCIATE hands out a relay
// that echoes each datagram back (or not) as Reply decides.
sealed class FakeSocks : IAsyncDisposable
{
    readonly TcpListener listener = new(IPAddress.Loopback, 0);
    readonly UdpClient relay = new(new IPEndPoint(IPAddress.Loopback, 0));
    readonly CancellationTokenSource stop = new();
    readonly HashSet<string> nonces = [];
    readonly Task accepting;
    readonly Task relaying;
    int connects;
    int datagrams;
    int round;

    public FakeSocks()
    {
        listener.Start();
        Port = ((IPEndPoint)listener.LocalEndpoint).Port;
        accepting = Accept();
        relaying = Relay();
    }

    public int Port { get; }
    public int Connects => Volatile.Read(ref connects);
    public int Datagrams => Volatile.Read(ref datagrams);
    public int DistinctNonces
    {
        get
        {
            lock (nonces)
            {
                return nonces.Count;
            }
        }
    }
    public string LastTarget { get; private set; } = "";
    public TimeSpan ConnectDelay { get; init; }

    // Receives the datagram's index within the current association and the
    // datagram itself; returns the reply to send, or null to drop it.
    public Func<int, byte[], byte[]?> Reply { get; set; } = (_, packet) => packet;

    public static byte[] WithIPv4Header(byte[] packet)
    {
        var dataStart = 5 + packet[4] + 2;
        byte[] header = [0, 0, 0, 1, 127, 0, 0, 1, packet[dataStart - 2], packet[dataStart - 1]];
        return [.. header, .. packet.AsSpan(dataStart)];
    }

    async Task Accept()
    {
        while (!stop.IsCancellationRequested)
        {
            TcpClient client;
            try
            {
                client = await listener.AcceptTcpClientAsync(stop.Token);
            }
            catch (Exception) when (stop.IsCancellationRequested)
            {
                return;
            }
            _ = Serve(client);
        }
    }

    async Task Serve(TcpClient client)
    {
        using (client)
        {
            try
            {
                var stream = client.GetStream();
                var hello = new byte[2];
                await stream.ReadExactlyAsync(hello, stop.Token);
                await stream.ReadExactlyAsync(new byte[hello[1]], stop.Token);
                await stream.WriteAsync(new byte[] { 5, 0 }, stop.Token);

                var request = new byte[4];
                await stream.ReadExactlyAsync(request, stop.Token);
                var host = await ReadHost(stream, request[3]);
                var portBytes = new byte[2];
                await stream.ReadExactlyAsync(portBytes, stop.Token);
                var port = (portBytes[0] << 8) | portBytes[1];

                if (request[1] == 1)
                {
                    Interlocked.Increment(ref connects);
                    await Task.Delay(ConnectDelay, stop.Token);
                    using var upstream = new TcpClient();
                    await upstream.ConnectAsync(host, port, stop.Token);
                    await stream.WriteAsync(new byte[] { 5, 0, 0, 1, 0, 0, 0, 0, 0, 0 }, stop.Token);
                    var up = upstream.GetStream();
                    await Task.WhenAny(stream.CopyToAsync(up, stop.Token), up.CopyToAsync(stream, stop.Token));
                    return;
                }
                if (request[1] == 3)
                {
                    Volatile.Write(ref round, 0);
                    var relayPort = ((IPEndPoint)relay.Client.LocalEndPoint!).Port;
                    await stream.WriteAsync(
                        new byte[] { 5, 0, 0, 1, 127, 0, 0, 1, (byte)(relayPort >> 8), (byte)relayPort },
                        stop.Token);
                    // The association lives as long as this connection.
                    while (await stream.ReadAsync(new byte[1], stop.Token) > 0)
                    {
                    }
                    return;
                }
                await stream.WriteAsync(new byte[] { 5, 7, 0, 1, 0, 0, 0, 0, 0, 0 }, stop.Token);
            }
            catch (Exception e) when (e is IOException or SocketException or OperationCanceledException)
            {
            }
        }
    }

    async Task<string> ReadHost(Stream stream, byte type)
    {
        if (type == 3)
        {
            var length = new byte[1];
            await stream.ReadExactlyAsync(length, stop.Token);
            var name = new byte[length[0]];
            await stream.ReadExactlyAsync(name, stop.Token);
            return Encoding.ASCII.GetString(name);
        }
        var address = new byte[type == 4 ? 16 : 4];
        await stream.ReadExactlyAsync(address, stop.Token);
        return new IPAddress(address).ToString();
    }

    async Task Relay()
    {
        while (!stop.IsCancellationRequested)
        {
            UdpReceiveResult received;
            try
            {
                received = await relay.ReceiveAsync(stop.Token);
            }
            catch (Exception) when (stop.IsCancellationRequested)
            {
                return;
            }
            catch (SocketException)
            {
                continue;
            }
            var packet = received.Buffer;
            Interlocked.Increment(ref datagrams);
            if (packet.Length > 5 && packet[3] == 3)
            {
                var hostEnd = 5 + packet[4];
                var target = Encoding.ASCII.GetString(packet, 5, packet[4]);
                LastTarget = $"{target}:{(packet[hostEnd] << 8) | packet[hostEnd + 1]}";
                lock (nonces)
                {
                    nonces.Add(Convert.ToHexString(packet.AsSpan(hostEnd + 2)));
                }
            }
            var index = Interlocked.Increment(ref round) - 1;
            if (Reply(index, packet) is { } reply)
            {
                await relay.SendAsync(reply, received.RemoteEndPoint, stop.Token);
            }
        }
    }

    public async ValueTask DisposeAsync()
    {
        stop.Cancel();
        listener.Stop();
        relay.Dispose();
        await Task.WhenAll(accepting, relaying).WaitAsync(TimeSpan.FromSeconds(5));
        stop.Dispose();
    }
}
