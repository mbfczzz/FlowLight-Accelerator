using System.Diagnostics;
using System.Net;
using System.Net.Sockets;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
namespace FlowLight.Core;

public static class TunnelProbe
{
    public static async Task<ProbeResult> Run(Connection conn, int port, CancellationToken ct)
    {
        using var handler = new SocketsHttpHandler { Proxy = new WebProxy($"socks5://127.0.0.1:{port}"), UseProxy = true, AllowAutoRedirect = false };
        using var http = new HttpClient(handler) { Timeout = TimeSpan.FromSeconds(10) };
        var times = new List<double>(); var result = new ProbeResult();
        for (var i = 0; i < 3; i++)
        {
            var watch = Stopwatch.StartNew(); using var response = await http.GetAsync(conn.ProbeUrl, ct); response.EnsureSuccessStatusCode();
            using var json = JsonDocument.Parse(await response.Content.ReadAsStringAsync(ct));
            var ip = json.RootElement.GetProperty("ip").GetString() ?? "";
            if (!IPAddress.TryParse(ip, out _)) throw new InvalidOperationException("Probe did not return the exit address");
            if (!string.IsNullOrWhiteSpace(conn.ExpectedExit) && conn.ExpectedExit != ip) throw new InvalidOperationException("Unexpected exit address");
            result.ExitIp = ip; times.Add(watch.Elapsed.TotalMilliseconds);
        }
        times.Sort(); result.Tcp = true; result.RttMs = times[1]; result.JitterMs = times[2] - times[0]; result.Samples = 3;
        if (conn.Udp) { await ProbeUdp(port, conn.ProbeUdp, ct); result.Udp = true; }
        return result;
    }
    static async Task ProbeUdp(int socksPort, string target, CancellationToken ct)
    {
        using var deadline = CancellationTokenSource.CreateLinkedTokenSource(ct); deadline.CancelAfter(TimeSpan.FromSeconds(5)); ct = deadline.Token;
        using var tcp = new TcpClient(); await tcp.ConnectAsync(IPAddress.Loopback, socksPort, ct); var stream = tcp.GetStream();
        await stream.WriteAsync(new byte[] { 5, 1, 0 }, ct); var greeting = new byte[2]; await stream.ReadExactlyAsync(greeting, ct);
        if (greeting[0] != 5 || greeting[1] != 0) throw new IOException("SOCKS handshake rejected");
        await stream.WriteAsync(new byte[] { 5, 3, 0, 1, 0, 0, 0, 0, 0, 0 }, ct); var header = new byte[4]; await stream.ReadExactlyAsync(header, ct);
        if (header[1] != 0) throw new IOException("SOCKS UDP association rejected");
        var host = await Address(stream, header[3], ct); var p = new byte[2]; await stream.ReadExactlyAsync(p, ct); var relayPort = (p[0] << 8) | p[1];
        if (IPAddress.TryParse(host, out var ip) && (ip.Equals(IPAddress.Any) || ip.Equals(IPAddress.IPv6Any))) host = "127.0.0.1";
        using var udp = new UdpClient(); udp.Connect(host, relayPort); var uri = new Uri("udp://" + target); var domain = Encoding.ASCII.GetBytes(uri.Host);
        if (domain.Length > 253 || uri.Port < 1) throw new IOException("Invalid UDP probe target");
        var data = Encoding.ASCII.GetBytes("FLA1").Concat(RandomNumberGenerator.GetBytes(24)).ToArray();
        var packet = new byte[] { 0, 0, 0, 3, (byte)domain.Length }.Concat(domain).Concat(new byte[] { (byte)(uri.Port >> 8), (byte)uri.Port }).Concat(data).ToArray();
        await udp.SendAsync(packet, ct); var res = await udp.ReceiveAsync(ct); var b = res.Buffer;
        if (b.Length < 4 || b[2] != 0) throw new IOException("Malformed UDP response");
        using var body = new MemoryStream(b, 4, b.Length - 4); await Address(body, b[3], ct);
        var rest = new byte[body.Length - body.Position]; await body.ReadExactlyAsync(rest, ct);
        if (rest.Length < 2 || !rest.AsSpan(2).SequenceEqual(data)) throw new IOException("UDP nonce verification failed");
    }
    static async Task<string> Address(Stream stream, byte type, CancellationToken ct)
    {
        var n = type == 1 ? 4 : type == 4 ? 16 : 0; if (type == 3) { var l = new byte[1]; await stream.ReadExactlyAsync(l, ct); n = l[0]; }
        if (n == 0) throw new IOException("Unsupported SOCKS address");
        var bytes = new byte[n]; await stream.ReadExactlyAsync(bytes, ct);
        return type == 3 ? Encoding.ASCII.GetString(bytes) : new IPAddress(bytes).ToString();
    }
}
