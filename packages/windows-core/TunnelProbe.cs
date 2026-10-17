using System.Diagnostics;
using System.Net;
using System.Net.Sockets;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;

namespace FlowLight.Core;

// Thrown when the tunnel answered but did not meet the probe's bar. Result
// carries what was measured so callers can report it.
public sealed class TunnelProbeException(string message, ProbeResult result) : IOException(message)
{
    public ProbeResult Result { get; } = result;
}

public static class TunnelProbe
{
    internal const int WarmupRequests = 1;
    internal const int RttSamples = 3;
    internal const int UdpSamples = 5;
    internal const int UdpRequired = 4;
    internal const int EchoSamples = 5;

    static readonly TimeSpan RequestTimeout = TimeSpan.FromSeconds(10);
    static readonly TimeSpan UdpProbeTimeout = TimeSpan.FromSeconds(15);
    static readonly TimeSpan UdpReplyTimeout = TimeSpan.FromSeconds(2);
    static readonly TimeSpan EchoReplyTimeout = TimeSpan.FromSeconds(5);
    static readonly byte[] NonceMagic = "FLA1"u8.ToArray();

    public static Task<ProbeResult> Run(Connection conn, int port, CancellationToken ct) =>
        Run(conn, port, UdpReplyTimeout, ct);

    internal static async Task<ProbeResult> Run(
        Connection conn,
        int port,
        TimeSpan udpReplyTimeout,
        CancellationToken ct)
    {
        try
        {
            using var handler = new SocketsHttpHandler
            {
                Proxy = new WebProxy($"socks5://127.0.0.1:{port}"),
                UseProxy = true,
                AllowAutoRedirect = false,
                MaxConnectionsPerServer = 1,
            };
            using var http = new HttpClient(handler) { Timeout = RequestTimeout };

            // The first request opens the path (SOCKS, TCP, TLS) and names
            // the exit. Everything after it runs at once, each leg on its own
            // connection: the timed requests on the one just opened, the echo
            // and the UDP samples on theirs. Each leg is a few tiny frames,
            // so they do not slow one another, and the probe takes as long as
            // its slowest leg rather than all three one after another.
            var result = new ProbeResult { ExitIp = await Fetch(http, conn, ct) };
            // The legs share a token: the first to fail cancels the others,
            // so a path that drops the timed requests is reported then, not
            // after the UDP leg has waited out its own timeouts.
            using var legs = CancellationTokenSource.CreateLinkedTokenSource(ct);
            var samples = Samples(http, conn, legs.Token);
            var echo = string.IsNullOrEmpty(conn.ExitEcho)
                ? Task.FromResult<List<double>?>(null)
                : ProbeEcho(port, conn.ExitEcho, EchoReplyTimeout, legs.Token);
            var udp = conn.Udp ? ProbeUdp(port, conn.ProbeUdp, udpReplyTimeout, legs.Token) : Task.FromResult(0);
            await FirstFailure(legs, samples, echo, udp);

            var times = samples.Result;
            result.Tcp = true;
            result.RttMs = times[times.Count / 2];
            result.JitterMs = times[^1] - times[0];
            result.Samples = times.Count;
            result.Loss = 0;
            // The round trip a game sees ends at the exit; the probe service
            // may be far beyond it. Without an echo to reach (an older node
            // program, or one that had to move it), the probe service's
            // round trip stands.
            if (echo.Result is { } toExit)
            {
                result.RttMs = toExit[toExit.Count / 2];
                result.JitterMs = toExit[^1] - toExit[0];
                result.Samples = toExit.Count;
            }
            if (conn.Udp)
            {
                ApplyUdp(result, udp.Result);
                if (!result.Udp)
                {
                    throw new TunnelProbeException(
                        $"隧道 UDP 探测失败：{udp.Result}/{UdpSamples} 个包有回包",
                        result);
                }
            }
            return result;
        }
        catch (OperationCanceledException e) when (!ct.IsCancellationRequested)
        {
            // An internal deadline is a probe failure, not a request to stop
            // the session; callers treat OperationCanceledException as the latter.
            throw new TimeoutException("隧道探测超时", e);
        }
        catch (HttpRequestException e)
        {
            // .NET's message names the probe host; players see this one.
            throw new IOException("隧道探测失败：经这条线路连不上检测服务，请换一条线路或稍后重试", e);
        }
    }

    // Waits for every leg; the moment one fails, cancels the rest, lets them
    // wind down, and rethrows that first failure.
    static async Task FirstFailure(CancellationTokenSource legs, params Task[] all)
    {
        var remaining = all.ToList();
        while (remaining.Count > 0)
        {
            var done = await Task.WhenAny(remaining);
            if (!done.IsCompletedSuccessfully)
            {
                legs.Cancel();
                try
                {
                    await Task.WhenAll(all);
                }
                catch (Exception)
                {
                    // The first failure is the one reported, below.
                }
                await done;
            }
            remaining.Remove(done);
        }
    }

    internal static void ApplyUdp(ProbeResult result, int replies)
    {
        result.Udp = replies >= UdpRequired;
        result.Loss = (double)(UdpSamples - replies) / UdpSamples;
    }

    // One request to the probe service through the tunnel: the exit address
    // it reports, checked against the one the line expects. The first call
    // pays for the SOCKS, TCP and TLS handshakes; later ones reuse that
    // pooled connection and measure the tunnel's round trip alone.
    static async Task<string> Fetch(HttpClient http, Connection conn, CancellationToken ct)
    {
        using var response = await http.GetAsync(conn.ProbeUrl, ct);
        response.EnsureSuccessStatusCode();
        var body = await response.Content.ReadAsStringAsync(ct);
        using var json = JsonDocument.Parse(body);
        var ip = json.RootElement.GetProperty("ip").GetString() ?? "";
        if (!IPAddress.TryParse(ip, out _))
        {
            throw new InvalidOperationException("探测未返回出口地址");
        }
        if (!string.IsNullOrWhiteSpace(conn.ExpectedExit) && conn.ExpectedExit != ip)
        {
            throw new InvalidOperationException("出口地址与预期不符");
        }
        return ip;
    }

    // RttSamples timed requests on the warm connection, sorted.
    static async Task<List<double>> Samples(HttpClient http, Connection conn, CancellationToken ct)
    {
        var times = new List<double>(RttSamples);
        for (var i = 0; i < RttSamples; i++)
        {
            var watch = Stopwatch.StartNew();
            await Fetch(http, conn, ct);
            times.Add(watch.Elapsed.TotalMilliseconds);
        }
        times.Sort();
        return times;
    }

    // Bounces frames off the exit's latency echo over one SOCKS5 connection:
    // one untimed, which opens the path through the line, then EchoSamples
    // timed. Returns the sorted round trips, or null if the echo did not
    // answer.
    internal static async Task<List<double>?> ProbeEcho(
        int socksPort,
        string target,
        TimeSpan replyTimeout,
        CancellationToken ct)
    {
        if (!IPEndPoint.TryParse(target, out var echo) || echo.Port < 1)
        {
            return null;
        }
        try
        {
            using var tcp = new TcpClient { NoDelay = true };
            await tcp.ConnectAsync(IPAddress.Loopback, socksPort, ct);
            var stream = tcp.GetStream();

            await stream.WriteAsync(new byte[] { 5, 1, 0 }, ct);
            var greeting = new byte[2];
            await stream.ReadExactlyAsync(greeting, ct);
            if (greeting[0] != 5 || greeting[1] != 0)
            {
                return null;
            }
            byte type = echo.AddressFamily == AddressFamily.InterNetworkV6 ? (byte)4 : (byte)1;
            byte[] request = [5, 1, 0, type, .. echo.Address.GetAddressBytes(), (byte)(echo.Port >> 8), (byte)echo.Port];
            await stream.WriteAsync(request, ct);
            var header = new byte[4];
            await stream.ReadExactlyAsync(header, ct);
            if (header[1] != 0)
            {
                return null;
            }
            await ReadAddress(stream, header[3], ct);
            await stream.ReadExactlyAsync(new byte[2], ct);

            var frame = new byte[NonceMagic.Length + 24];
            NonceMagic.CopyTo(frame, 0);
            var reply = new byte[frame.Length];
            var times = new List<double>(EchoSamples);
            for (var i = 0; i <= EchoSamples; i++)
            {
                RandomNumberGenerator.Fill(frame.AsSpan(NonceMagic.Length));
                using var wait = CancellationTokenSource.CreateLinkedTokenSource(ct);
                wait.CancelAfter(replyTimeout);
                var watch = Stopwatch.StartNew();
                await stream.WriteAsync(frame, wait.Token);
                await stream.ReadExactlyAsync(reply, wait.Token);
                var elapsed = watch.Elapsed.TotalMilliseconds;
                if (!reply.AsSpan().SequenceEqual(frame))
                {
                    return null;
                }
                if (i > 0)
                {
                    times.Add(elapsed);
                }
            }
            times.Sort();
            return times;
        }
        catch (Exception e) when (!ct.IsCancellationRequested && e is IOException or SocketException or OperationCanceledException)
        {
            return null;
        }
    }

    // Opens one SOCKS5 UDP association and sends UdpSamples datagrams, each
    // with its own nonce. Returns how many came back with their exact payload.
    internal static async Task<int> ProbeUdp(
        int socksPort,
        string target,
        TimeSpan replyTimeout,
        CancellationToken ct)
    {
        using var deadline = CancellationTokenSource.CreateLinkedTokenSource(ct);
        deadline.CancelAfter(UdpProbeTimeout);
        ct = deadline.Token;

        var uri = new Uri("udp://" + target);
        var domain = Encoding.ASCII.GetBytes(uri.Host);
        if (domain.Length > 253 || uri.Port < 1)
        {
            throw new IOException("UDP 探测目标无效");
        }

        using var tcp = new TcpClient();
        await tcp.ConnectAsync(IPAddress.Loopback, socksPort, ct);
        var stream = tcp.GetStream();

        await stream.WriteAsync(new byte[] { 5, 1, 0 }, ct);
        var greeting = new byte[2];
        await stream.ReadExactlyAsync(greeting, ct);
        if (greeting[0] != 5 || greeting[1] != 0)
        {
            throw new IOException("本地代理握手被拒绝");
        }

        await stream.WriteAsync(new byte[] { 5, 3, 0, 1, 0, 0, 0, 0, 0, 0 }, ct);
        var header = new byte[4];
        await stream.ReadExactlyAsync(header, ct);
        if (header[1] != 0)
        {
            throw new IOException("本地代理拒绝 UDP 关联");
        }
        var host = await ReadAddress(stream, header[3], ct);
        var portBytes = new byte[2];
        await stream.ReadExactlyAsync(portBytes, ct);
        var relayPort = (portBytes[0] << 8) | portBytes[1];
        if (IPAddress.TryParse(host, out var ip) && (ip.Equals(IPAddress.Any) || ip.Equals(IPAddress.IPv6Any)))
        {
            host = "127.0.0.1";
        }

        using var udp = new UdpClient();
        udp.Connect(host, relayPort);
        var prefix = new byte[] { 0, 0, 0, 3, (byte)domain.Length }
            .Concat(domain)
            .Concat(new byte[] { (byte)(uri.Port >> 8), (byte)uri.Port })
            .ToArray();

        var verified = 0;
        for (var i = 0; i < UdpSamples; i++)
        {
            if (await SampleUdp(udp, prefix, replyTimeout, ct))
            {
                verified++;
            }
        }
        return verified;
    }

    static async Task<bool> SampleUdp(UdpClient udp, byte[] prefix, TimeSpan timeout, CancellationToken ct)
    {
        var payload = NonceMagic.Concat(RandomNumberGenerator.GetBytes(24)).ToArray();
        using var wait = CancellationTokenSource.CreateLinkedTokenSource(ct);
        wait.CancelAfter(timeout);
        try
        {
            await udp.SendAsync(prefix.Concat(payload).ToArray(), wait.Token);
            while (true)
            {
                var response = await udp.ReceiveAsync(wait.Token);
                // A late reply to an earlier sample carries a different nonce,
                // fails verification and is skipped rather than counted.
                if (IsEcho(response.Buffer, payload))
                {
                    return true;
                }
            }
        }
        catch (OperationCanceledException) when (!ct.IsCancellationRequested)
        {
            return false;
        }
        catch (SocketException)
        {
            // Windows reports an ICMP port-unreachable on a later receive as a
            // connection reset; it means this datagram was lost.
            return false;
        }
    }

    // SOCKS5 UDP reply: RSV(2) FRAG(1) ATYP(1) DST.ADDR DST.PORT(2) DATA. The
    // data must be exactly the payload that was sent.
    internal static bool IsEcho(byte[] packet, ReadOnlySpan<byte> payload)
    {
        if (packet.Length < 4 || packet[2] != 0)
        {
            return false;
        }
        int offset;
        switch (packet[3])
        {
            case 1:
                offset = 4 + 4;
                break;
            case 4:
                offset = 4 + 16;
                break;
            case 3:
                if (packet.Length < 5 || packet[4] == 0)
                {
                    return false;
                }
                offset = 5 + packet[4];
                break;
            default:
                return false;
        }
        offset += 2;
        return packet.Length >= offset && packet.AsSpan(offset).SequenceEqual(payload);
    }

    static async Task<string> ReadAddress(Stream stream, byte type, CancellationToken ct)
    {
        var length = type switch { 1 => 4, 4 => 16, _ => 0 };
        if (type == 3)
        {
            var lengthByte = new byte[1];
            await stream.ReadExactlyAsync(lengthByte, ct);
            length = lengthByte[0];
        }
        if (length == 0)
        {
            throw new IOException("本地代理返回了不支持的地址类型");
        }
        var bytes = new byte[length];
        await stream.ReadExactlyAsync(bytes, ct);
        return type == 3 ? Encoding.ASCII.GetString(bytes) : new IPAddress(bytes).ToString();
    }
}
