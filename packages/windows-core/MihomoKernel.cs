using System.ComponentModel;
using System.Diagnostics;
using System.Net;
using System.Net.NetworkInformation;
using System.Net.Sockets;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;

namespace FlowLight.Core;

public sealed class MihomoKernel : IDisposable
{
    public const string ProbeInbound = "flowlight-probe";
    internal const string LogFileName = "kernel.log";
    internal const int OutputTailChars = 8 * 1024;
    internal const long LogFileLimitBytes = 256 * 1024;
    const string JournalFileName = "owned-kernel.json";

    readonly string binary;
    readonly string expectedHash;
    readonly string root;
    readonly object outputGate = new();
    readonly StringBuilder outputTail = new();
    Process? process;
    string? configPath;
    string? secret;
    FileStream? log;
    Task kernelPumps = Task.CompletedTask;
    bool disposed;
    OwnedProcessJob? ownedJob;

    public int SocksPort { get; private set; }
    public bool Running => process is { HasExited: false };

    // How many times Start polls the SOCKS listener, 100 ms apart.
    internal int ReadinessAttempts { get; init; } = 50;

    // Creating the TUN adapter can take several seconds on first start.
    internal TimeSpan ReadinessBudget { get; init; } = TimeSpan.FromSeconds(20);

    public MihomoKernel(string binary, string expectedHash, string root)
    {
        this.binary = Path.GetFullPath(binary);
        this.expectedHash = expectedHash;
        this.root = Path.GetFullPath(root);
        Directory.CreateDirectory(this.root);
        Recover();
    }

    string JournalPath => Path.Combine(root, JournalFileName);

    // A stale or torn journal, or a PID since reused by a protected process,
    // must never stop the service from starting.
    void Recover()
    {
        var journal = JournalPath;
        if (!File.Exists(journal))
        {
            return;
        }
        try
        {
            using var doc = JsonDocument.Parse(File.ReadAllText(journal));
            using var old = Process.GetProcessById(doc.RootElement.GetProperty("pid").GetInt32());
            var started = doc.RootElement.GetProperty("started").GetInt64();
            if (string.Equals(old.MainModule?.FileName, binary, StringComparison.OrdinalIgnoreCase) &&
                old.StartTime.ToUniversalTime().Ticks == started)
            {
                old.Kill(true);
                old.WaitForExit(5000);
            }
        }
        catch (Exception e) when (e is ArgumentException or InvalidOperationException or Win32Exception
                                      or JsonException or KeyNotFoundException or NotSupportedException)
        {
        }
        File.Delete(journal);
    }

    public static object BuildConfig(Connection conn, int port)
    {
        if ((conn.Profile.Processes?.Length ?? 0) + (conn.Profile.Paths?.Length ?? 0) == 0)
        {
            throw new ArgumentException("Game process rules are required");
        }

        // Tunnel probes come from FlowLight.Service.exe, which the process rule
        // below sends DIRECT. They use a dedicated inbound pinned to the
        // tunnel, so they measure the tunnel and never the direct path.
        var rules = new List<string>
        {
            $"IN-NAME,{ProbeInbound},FLOWLIGHT",
            "PROCESS-NAME,mihomo.exe,DIRECT",
            "PROCESS-NAME,FlowLight.Service.exe,DIRECT",
        };
        foreach (var value in conn.Profile.ExcludeDomains ?? [])
        {
            rules.Add(Rule("DOMAIN-SUFFIX", value, "DIRECT"));
        }
        foreach (var value in conn.Profile.Paths ?? [])
        {
            rules.Add(Rule("PROCESS-PATH", value, "FLOWLIGHT"));
        }
        foreach (var value in conn.Profile.Processes ?? [])
        {
            rules.Add(Rule("PROCESS-NAME", value, "FLOWLIGHT"));
        }
        foreach (var value in conn.Profile.Domains ?? [])
        {
            rules.Add(Rule("DOMAIN-SUFFIX", value, "FLOWLIGHT"));
        }
        foreach (var value in conn.Profile.Cidrs ?? [])
        {
            rules.Add(Rule(value.Contains(':') ? "IP-CIDR6" : "IP-CIDR", value, "FLOWLIGHT"));
        }
        rules.Add("MATCH,DIRECT");

        var dns = NetworkInterface.GetAllNetworkInterfaces()
            .Where(n => n.OperationalStatus == OperationalStatus.Up &&
                        !n.Name.Contains("FlowLight", StringComparison.OrdinalIgnoreCase))
            .SelectMany(n => n.GetIPProperties().DnsAddresses)
            .Where(a => !IPAddress.IsLoopback(a))
            .Select(a => a.ToString())
            .Distinct()
            .ToArray();
        if (dns.Length == 0)
        {
            throw new InvalidOperationException("No system DNS resolver available");
        }

        var probeListener = new Dictionary<string, object>
        {
            { "name", ProbeInbound },
            { "type", "socks" },
            { "listen", "127.0.0.1" },
            { "port", port },
            { "udp", true },
            { "proxy", "FLOWLIGHT" },
        };
        var tun = new Dictionary<string, object>
        {
            { "enable", true },
            { "device", "FlowLightTun" },
            { "stack", "mixed" },
            { "auto-route", true },
            { "auto-detect-interface", true },
            { "dns-hijack", new[] { "any:53" } },
            { "strict-route", false },
        };
        var dnsConfig = new Dictionary<string, object>
        {
            { "enable", true },
            { "listen", "127.0.0.1:10553" },
            { "enhanced-mode", "fake-ip" },
            { "nameserver", dns },
            { "default-nameserver", dns },
        };
        var proxy = new
        {
            name = "FLOWLIGHT",
            type = "ss",
            server = conn.Host,
            port = conn.Port,
            cipher = conn.Cipher,
            password = conn.Password,
            udp = conn.Udp,
        };
        return new Dictionary<string, object>
        {
            { "allow-lan", false },
            { "mode", "rule" },
            { "find-process-mode", "always" },
            { "log-level", "warning" },
            { "ipv6", false },
            { "listeners", new[] { probeListener } },
            { "tun", tun },
            { "dns", dnsConfig },
            { "proxies", new[] { proxy } },
            { "rules", rules },
        };
    }

    static string Rule(string kind, string value, string target)
    {
        if (string.IsNullOrWhiteSpace(value) || value.IndexOfAny([',', '\r', '\n', '\0']) >= 0)
        {
            throw new ArgumentException("Invalid game rule");
        }
        return $"{kind},{value},{target}";
    }

    public async Task Start(Connection conn, CancellationToken ct)
    {
        if (Running)
        {
            throw new InvalidOperationException("A kernel is already running");
        }
        if (!OperatingSystem.IsWindows())
        {
            throw new PlatformNotSupportedException("Windows network service required");
        }

        // Release whatever a previous, self-terminated kernel left behind.
        Stop();
        BeginOutput(conn.Password);

        // A failed start must not leave a kernel, its journal or its
        // plaintext configuration behind, whatever the caller does next.
        try
        {
            await StartCore(conn, ct);
        }
        catch
        {
            Stop();
            throw;
        }
    }

    async Task StartCore(Connection conn, CancellationToken ct)
    {
        await VerifyBinary(ct);

        var listener = new TcpListener(IPAddress.Loopback, 0);
        listener.Start();
        try
        {
            SocksPort = ((IPEndPoint)listener.LocalEndpoint).Port;
        }
        finally
        {
            listener.Stop();
        }

        var config = Path.Combine(root, "session-" + Guid.NewGuid().ToString("N") + ".json");
        configPath = config;
        await File.WriteAllTextAsync(config, JsonSerializer.Serialize(BuildConfig(conn, SocksPort)), ct);

        var job = new OwnedProcessJob();
        ownedJob = job;
        await ValidateConfig(job, config, ct);

        var kernel = NewProcess();
        kernel.StartInfo.ArgumentList.Add("-f");
        kernel.StartInfo.ArgumentList.Add(config);
        try
        {
            kernel.Start();
        }
        catch
        {
            kernel.Dispose();
            throw;
        }
        process = kernel;
        job.Add(kernel);
        var pumps = PumpOutput(kernel);
        kernelPumps = pumps;

        var journal = new { pid = kernel.Id, started = kernel.StartTime.ToUniversalTime().Ticks };
        await File.WriteAllTextAsync(JournalPath, JsonSerializer.Serialize(journal), ct);

        // Windows retries a refused loopback connect for about two seconds,
        // so each attempt gets its own short timeout and the whole wait a
        // fixed budget; otherwise 50 attempts could take minutes.
        var readyBy = DateTime.UtcNow + ReadinessBudget;
        for (var i = 0; i < ReadinessAttempts && DateTime.UtcNow < readyBy; i++)
        {
            if (kernel.HasExited)
            {
                await Drain(pumps);
                throw new InvalidOperationException("Kernel exited; check service privileges" + OutputDiagnostics());
            }
            using var attempt = CancellationTokenSource.CreateLinkedTokenSource(ct);
            attempt.CancelAfter(TimeSpan.FromMilliseconds(250));
            try
            {
                using var test = new TcpClient();
                await test.ConnectAsync(IPAddress.Loopback, SocksPort, attempt.Token);
                return;
            }
            catch (Exception e) when (e is SocketException || (e is OperationCanceledException && !ct.IsCancellationRequested))
            {
                await Task.Delay(100, ct);
            }
        }
        throw new TimeoutException("Kernel listener did not become ready" + OutputDiagnostics());
    }

    async Task VerifyBinary(CancellationToken ct)
    {
        string hash;
        await using (var file = File.OpenRead(binary))
        {
            hash = Convert.ToHexString(await SHA256.HashDataAsync(file, ct));
        }
        if (expectedHash.Length != 64 || !hash.Equals(expectedHash, StringComparison.OrdinalIgnoreCase))
        {
            RecordOutput($"FlowLight: {binary} has SHA-256 {hash}; expected {expectedHash}{Environment.NewLine}");
            throw new InvalidOperationException("Kernel checksum mismatch" + OutputDiagnostics());
        }
    }

    async Task ValidateConfig(OwnedProcessJob job, string config, CancellationToken ct)
    {
        using var check = NewProcess();
        check.StartInfo.ArgumentList.Add("-t");
        check.StartInfo.ArgumentList.Add("-f");
        check.StartInfo.ArgumentList.Add(config);
        check.Start();
        try
        {
            job.Add(check);
        }
        catch
        {
            check.Kill(true);
            throw;
        }

        var pumps = PumpOutput(check);
        try
        {
            await check.WaitForExitAsync(ct);
            await pumps.WaitAsync(ct);
        }
        finally
        {
            if (!check.HasExited)
            {
                check.Kill(true);
                check.WaitForExit(5000);
            }
        }
        if (check.ExitCode != 0)
        {
            throw new InvalidOperationException("Kernel configuration validation failed" + OutputDiagnostics());
        }
    }

    Process NewProcess() => new()
    {
        StartInfo = new ProcessStartInfo(binary)
        {
            UseShellExecute = false,
            CreateNoWindow = true,
            RedirectStandardError = true,
            RedirectStandardOutput = true,
            WorkingDirectory = root,
        },
    };

    // Kernel output feeds a bounded in-memory tail, used in error messages,
    // and kernel.log in the kernel root, which outlives the session.
    Task PumpOutput(Process kernel) =>
        Task.WhenAll(Pump(kernel.StandardOutput), Pump(kernel.StandardError));

    async Task Pump(StreamReader reader)
    {
        try
        {
            while (await reader.ReadLineAsync() is { } line)
            {
                RecordOutput(line + Environment.NewLine);
            }
        }
        catch (Exception e) when (e is IOException or ObjectDisposedException or InvalidOperationException)
        {
            // The pipe closed underneath us because the process was disposed.
        }
    }

    static void WaitQuietly(Task pumps)
    {
        try
        {
            pumps.Wait(TimeSpan.FromSeconds(2));
        }
        catch (AggregateException)
        {
        }
    }

    // The pipes close once the process exits; wait briefly so the last lines
    // the kernel wrote before dying are in the diagnostics.
    static async Task Drain(Task pumps)
    {
        try
        {
            await pumps.WaitAsync(TimeSpan.FromSeconds(2));
        }
        catch (TimeoutException)
        {
        }
    }

    void BeginOutput(string password)
    {
        lock (outputGate)
        {
            // The tunnel key must not reach kernel.log or an error message
            // shown to the user; a very short value would mangle the text.
            secret = password is { Length: >= 4 } ? password : null;
            outputTail.Clear();
            AppendLog($"{Environment.NewLine}==== kernel start {DateTime.UtcNow:O} ===={Environment.NewLine}");
        }
    }

    void RecordOutput(string text)
    {
        lock (outputGate)
        {
            if (secret is not null)
            {
                text = text.Replace(secret, "[redacted]", StringComparison.Ordinal);
            }
            outputTail.Append(text);
            if (outputTail.Length > OutputTailChars)
            {
                outputTail.Remove(0, outputTail.Length - OutputTailChars);
            }
            AppendLog(text);
        }
    }

    // Best effort and bounded: kernel.log is rotated to kernel.log.1 once it
    // would pass the limit, so at most two such files exist. It is created
    // with the kernel root's inherited ACL and can be read while the kernel
    // runs. Losing diagnostics must never take the tunnel down. Callers hold
    // outputGate.
    void AppendLog(string text)
    {
        if (disposed)
        {
            return;
        }
        var path = Path.Combine(root, LogFileName);
        var bytes = Encoding.UTF8.GetBytes(text);
        try
        {
            log ??= OpenLog(path);
            if (log.Length > 0 && log.Length + bytes.Length > LogFileLimitBytes)
            {
                log.Dispose();
                log = null;
                Rotate(path);
                log = OpenLog(path);
            }
            log.Write(bytes);
            log.Flush();
        }
        catch (Exception e) when (e is IOException or UnauthorizedAccessException)
        {
            log?.Dispose();
            log = null;
        }
    }

    static FileStream OpenLog(string path) =>
        new(path, FileMode.Append, FileAccess.Write, FileShare.ReadWrite | FileShare.Delete);

    // A viewer holding the log open can block the move; truncating in place
    // still keeps the file bounded.
    static void Rotate(string path)
    {
        try
        {
            File.Move(path, path + ".1", overwrite: true);
        }
        catch (Exception e) when (e is IOException or UnauthorizedAccessException)
        {
            using var truncate = new FileStream(
                path,
                FileMode.Truncate,
                FileAccess.Write,
                FileShare.ReadWrite | FileShare.Delete);
        }
    }

    void CloseLog()
    {
        lock (outputGate)
        {
            log?.Dispose();
            log = null;
        }
    }

    // The most recent kernel output (at most OutputTailChars), for callers
    // that report a kernel which stopped after a successful start.
    public string RecentOutput
    {
        get
        {
            lock (outputGate)
            {
                return outputTail.ToString().Trim();
            }
        }
    }

    string OutputDiagnostics()
    {
        var tail = RecentOutput;
        if (tail.Length == 0)
        {
            return $" (no kernel output; see {LogFileName})";
        }
        var nl = Environment.NewLine;
        return $"{nl}Kernel output (last {OutputTailChars / 1024} KB, see {LogFileName}):{nl}{tail}";
    }

    // Stop runs in the session's finally block, so it does not throw: a
    // cleanup failure must not mask why the session ended or skip the
    // remaining cleanup. The job handle closing kills every owned process.
    public void Stop()
    {
        var job = ownedJob;
        ownedJob = null;
        job?.Dispose();

        var kernel = process;
        process = null;
        var exited = true;
        if (kernel is not null)
        {
            try
            {
                if (!kernel.HasExited)
                {
                    kernel.Kill(true);
                }
                exited = kernel.WaitForExit(5000);
                if (exited)
                {
                    // The pipes reach EOF once the kernel is gone. Waiting
                    // keeps its last words and means no late line can reopen
                    // kernel.log after it is closed below.
                    WaitQuietly(kernelPumps);
                }
            }
            catch (Exception e) when (e is InvalidOperationException or Win32Exception or NotSupportedException
                                          or AggregateException)
            {
                exited = false;
            }
            finally
            {
                kernel.Dispose();
                kernelPumps = Task.CompletedTask;
            }
        }

        if (configPath is not null)
        {
            TryDelete(configPath);
            configPath = null;
        }
        // A kernel that may still be alive keeps its journal, so the next
        // service start can still find and stop it.
        if (exited)
        {
            TryDelete(JournalPath);
        }
        CloseLog();
    }

    static void TryDelete(string path)
    {
        try
        {
            File.Delete(path);
        }
        catch (Exception e) when (e is IOException or UnauthorizedAccessException)
        {
        }
    }

    public void Dispose()
    {
        Stop();
        lock (outputGate)
        {
            disposed = true;
        }
    }
}
