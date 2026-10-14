using System.Diagnostics;
using System.Net;
using System.Net.NetworkInformation;
using System.Net.Sockets;
using System.Security.Cryptography;
using System.Text.Json;
namespace FlowLight.Core;

public sealed class MihomoKernel : IDisposable
{
    readonly string binary, expectedHash, root; Process? process; string? configPath; OwnedProcessJob? ownedJob;
    public int SocksPort { get; private set; }
    public bool Running => process is { HasExited: false };
    public MihomoKernel(string binary, string expectedHash, string root)
    {
        this.binary = Path.GetFullPath(binary); this.expectedHash = expectedHash; this.root = Path.GetFullPath(root);
        Directory.CreateDirectory(this.root); Recover();
    }
    void Recover()
    {
        var journal = Path.Combine(root, "owned-kernel.json"); if (!File.Exists(journal)) return;
        using var doc = JsonDocument.Parse(File.ReadAllText(journal));
        try
        {
            var old = Process.GetProcessById(doc.RootElement.GetProperty("pid").GetInt32());
            if (old.MainModule?.FileName == binary && old.StartTime.ToUniversalTime().Ticks == doc.RootElement.GetProperty("started").GetInt64()) { old.Kill(true); old.WaitForExit(5000); }
        }
        catch (ArgumentException) { }
        File.Delete(journal);
    }
    public static object BuildConfig(Connection conn, int port)
    {
        if ((conn.Profile.Processes?.Length ?? 0) + (conn.Profile.Paths?.Length ?? 0) == 0) throw new ArgumentException("Game process rules are required");
        var rules = new List<string> { "PROCESS-NAME,mihomo.exe,DIRECT", "PROCESS-NAME,FlowLight.Service.exe,DIRECT" };
        foreach (var value in conn.Profile.ExcludeDomains ?? []) rules.Add(Rule("DOMAIN-SUFFIX", value, "DIRECT"));
        foreach (var value in conn.Profile.Paths ?? []) rules.Add(Rule("PROCESS-PATH", value, "FLOWLIGHT"));
        foreach (var value in conn.Profile.Processes ?? []) rules.Add(Rule("PROCESS-NAME", value, "FLOWLIGHT"));
        foreach (var value in conn.Profile.Domains ?? []) rules.Add(Rule("DOMAIN-SUFFIX", value, "FLOWLIGHT"));
        foreach (var value in conn.Profile.Cidrs ?? []) rules.Add(Rule(value.Contains(':') ? "IP-CIDR6" : "IP-CIDR", value, "FLOWLIGHT"));
        rules.Add("MATCH,DIRECT");
        var dns = NetworkInterface.GetAllNetworkInterfaces().Where(n => n.OperationalStatus == OperationalStatus.Up && !n.Name.Contains("FlowLight", StringComparison.OrdinalIgnoreCase)).SelectMany(n => n.GetIPProperties().DnsAddresses).Where(a => !IPAddress.IsLoopback(a)).Select(a => a.ToString()).Distinct().ToArray();
        if (dns.Length == 0) throw new InvalidOperationException("No system DNS resolver available");
        return new Dictionary<string, object>{
   {"mixed-port",port},{"allow-lan",false},{"bind-address","127.0.0.1"},{"mode","rule"},{"find-process-mode","always"},{"log-level","warning"},{"ipv6",true},
   {"tun",new Dictionary<string,object>{{"enable",true},{"device","FlowLightTun"},{"stack","mixed"},{"auto-route",true},{"auto-detect-interface",true},{"dns-hijack",new[]{"any:53"}},{"strict-route",false}}},
   {"dns",new Dictionary<string,object>{{"enable",true},{"listen","127.0.0.1:10553"},{"enhanced-mode","fake-ip"},{"nameserver",dns},{"default-nameserver",dns}}},
   {"proxies",new[]{new{name="FLOWLIGHT",type="ss",server=conn.Host,port=conn.Port,cipher=conn.Cipher,password=conn.Password,udp=conn.Udp}}},{"rules",rules}
  };
    }
    static string Rule(string kind, string value, string target)
    {
        if (string.IsNullOrWhiteSpace(value) || value.IndexOfAny([',', '\r', '\n', '\0']) >= 0) throw new ArgumentException("Invalid game rule");
        return $"{kind},{value},{target}";
    }
    public async Task Start(Connection conn, CancellationToken ct)
    {
        if (Running) throw new InvalidOperationException("A kernel is already running");
        if (!OperatingSystem.IsWindows()) throw new PlatformNotSupportedException("Windows network service required");
        ownedJob?.Dispose(); ownedJob = new OwnedProcessJob();
        await using (var f = File.OpenRead(binary))
        {
            var hash = Convert.ToHexString(await SHA256.HashDataAsync(f, ct));
            if (expectedHash.Length != 64 || !hash.Equals(expectedHash, StringComparison.OrdinalIgnoreCase)) throw new InvalidOperationException("Kernel checksum mismatch");
        }
        var listener = new TcpListener(IPAddress.Loopback, 0); listener.Start(); SocksPort = ((IPEndPoint)listener.LocalEndpoint).Port; listener.Stop();
        configPath = Path.Combine(root, "session-" + Guid.NewGuid().ToString("N") + ".json");
        await File.WriteAllTextAsync(configPath, JsonSerializer.Serialize(BuildConfig(conn, SocksPort)), ct);
        using var check = NewProcess(); check.StartInfo.ArgumentList.Add("-t"); check.StartInfo.ArgumentList.Add("-f"); check.StartInfo.ArgumentList.Add(configPath);
        check.Start(); try { ownedJob.Add(check); } catch { check.Kill(true); throw; }
        var stderr = check.StandardError.ReadToEndAsync(ct); var stdout = check.StandardOutput.ReadToEndAsync(ct);
        try { await check.WaitForExitAsync(ct); await Task.WhenAll(stderr, stdout); }
        finally { if (!check.HasExited) { check.Kill(true); check.WaitForExit(5000); } }
        if (check.ExitCode != 0) throw new InvalidOperationException("Kernel configuration validation failed");
        process = NewProcess(); process.StartInfo.ArgumentList.Add("-f"); process.StartInfo.ArgumentList.Add(configPath); process.Start(); ownedJob.Add(process);
        _ = process.StandardOutput.ReadToEndAsync(); _ = process.StandardError.ReadToEndAsync();
        await File.WriteAllTextAsync(Path.Combine(root, "owned-kernel.json"), JsonSerializer.Serialize(new { pid = process.Id, started = process.StartTime.ToUniversalTime().Ticks }), ct);
        for (var i = 0; i < 50; i++)
        {
            if (process.HasExited) throw new InvalidOperationException("Kernel exited; check service privileges");
            try { using var test = new TcpClient(); await test.ConnectAsync(IPAddress.Loopback, SocksPort, ct); return; }
            catch (SocketException) { await Task.Delay(100, ct); }
        }
        throw new TimeoutException("Kernel listener did not become ready");
    }
    Process NewProcess() => new() { StartInfo = new ProcessStartInfo(binary) { UseShellExecute = false, CreateNoWindow = true, RedirectStandardError = true, RedirectStandardOutput = true, WorkingDirectory = root } };
    public void Stop()
    {
        ownedJob?.Dispose(); ownedJob = null;
        if (process is not null) { try { if (!process.HasExited) { process.Kill(true); process.WaitForExit(5000); } } finally { process.Dispose(); process = null; } }
        if (configPath != null && File.Exists(configPath)) File.Delete(configPath);
        var journal = Path.Combine(root, "owned-kernel.json"); if (File.Exists(journal)) File.Delete(journal);
    }
    public void Dispose() => Stop();
}
