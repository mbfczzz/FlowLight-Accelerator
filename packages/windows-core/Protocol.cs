using System.IO.Pipes;
using System.Runtime.InteropServices;
using System.Security.Principal;
using System.Text;
using System.Text.Json;
using Microsoft.Win32.SafeHandles;

namespace FlowLight.Core;

// The local service could not be reached: it is not running, not installed,
// or the pipe broke before it answered. Busy means the service is running but
// every pipe instance was in use until the connect timed out.
public sealed class ServiceUnavailableException(string message, Exception? inner = null, bool busy = false)
    : IOException(message, inner)
{
    public bool Busy { get; } = busy;
}

// The local service answered and reported that the request failed. Message is
// the service's own error text.
public sealed class ServiceErrorException(string message) : InvalidOperationException(message);

public static class Wire
{
    // A pipe name exists while any server instance does, busy or not.
    // Enumerating the pipe namespace never connects, unlike opening the path,
    // which would take one of the instances it is trying to detect.
    internal static bool PipeExists(string name = PipeName)
    {
        try
        {
            return Directory.EnumerateFiles(@"\\.\pipe\")
                .Any(p => string.Equals(Path.GetFileName(p), name, StringComparison.OrdinalIgnoreCase));
        }
        catch (Exception e) when (e is IOException or UnauthorizedAccessException)
        {
            return false;
        }
    }

    public const string PipeName = "FlowLight.Accelerator.v1";
    const int ConnectTimeoutMs = 3000;

    public static readonly JsonSerializerOptions Json = new()
    {
        PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower,
        PropertyNameCaseInsensitive = true,
    };

    public static async Task<JsonElement> CallAsync(string command, object? args = null, CancellationToken ct = default)
    {
        using var pipe = new NamedPipeClientStream(
            ".",
            PipeName,
            PipeDirection.InOut,
            PipeOptions.Asynchronous,
            TokenImpersonationLevel.Impersonation);
        try
        {
            await pipe.ConnectAsync(ConnectTimeoutMs, ct);
        }
        catch (TimeoutException e) when (PipeExists())
        {
            throw new ServiceUnavailableException("Local service is busy", e, busy: true);
        }
        catch (TimeoutException e)
        {
            throw new ServiceUnavailableException("Local service is not running", e);
        }
        catch (IOException e)
        {
            throw new ServiceUnavailableException("Local service is not reachable", e);
        }

        // Identity failures are not "service unavailable": they mean something
        // else owns the pipe, and the caller must see that message.
        VerifyServer(pipe);

        string? line;
        try
        {
            using var writer = new StreamWriter(pipe, leaveOpen: true) { AutoFlush = true };
            using var reader = new StreamReader(pipe, leaveOpen: true);
            await writer.WriteLineAsync(JsonSerializer.Serialize(new { command, args }, Json));
            line = await reader.ReadLineAsync(ct);
        }
        catch (IOException e)
        {
            throw new ServiceUnavailableException("Local service connection was interrupted", e);
        }
        if (line is null)
        {
            throw new ServiceUnavailableException("Local service did not respond");
        }

        using var doc = JsonDocument.Parse(line);
        var root = doc.RootElement;
        if (!root.GetProperty("success").GetBoolean())
        {
            var error = root.TryGetProperty("error", out var text) && text.ValueKind == JsonValueKind.String
                ? text.GetString()
                : null;
            throw new ServiceErrorException(string.IsNullOrWhiteSpace(error) ? "Local service request failed" : error);
        }
        return root.GetProperty("data").Clone();
    }

    static void VerifyServer(NamedPipeClientStream pipe)
    {
        if (Environment.GetEnvironmentVariable("FLOWLIGHT_DEV_PIPE") == "1")
        {
            return;
        }
        if (!GetNamedPipeServerProcessId(pipe.SafePipeHandle, out var pid))
        {
            throw new IOException("Cannot verify local service identity");
        }
        using var handle = OpenProcess(0x1000, false, pid);
        var path = new StringBuilder(32768);
        var length = (uint)path.Capacity;
        if (handle.IsInvalid || !QueryFullProcessImageName(handle, 0, path, ref length))
        {
            throw new IOException("Cannot verify local service executable");
        }
        var expected = Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.ProgramFiles),
            "FlowLight Accelerator",
            "service",
            "FlowLight.Service.exe");
        if (!Path.GetFullPath(path.ToString()).Equals(expected, StringComparison.OrdinalIgnoreCase))
        {
            throw new IOException("Untrusted local service; install the verified release in Program Files");
        }
    }

    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool GetNamedPipeServerProcessId(SafePipeHandle pipe, out uint pid);

    [DllImport("kernel32.dll", SetLastError = true)]
    static extern SafeProcessHandle OpenProcess(uint access, bool inherit, uint pid);

    [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    static extern bool QueryFullProcessImageName(
        SafeProcessHandle process,
        uint flags,
        StringBuilder path,
        ref uint size);
}

public sealed record GameProfile(
    string[] Processes,
    string[] Paths,
    string[] Domains,
    string[] Cidrs,
    string[] ExcludeDomains,
    bool Udp);

public sealed record Connection(
    string Host,
    int Port,
    string Cipher,
    string Password,
    bool Udp,
    GameProfile Profile,
    string ProbeUrl,
    string ProbeUdp,
    string? ExpectedExit);

public sealed class ProbeResult
{
    public bool Tcp { get; set; }
    public bool Udp { get; set; }
    public double RttMs { get; set; }
    public double JitterMs { get; set; }
    public double Loss { get; set; }
    public int Samples { get; set; }
    public string ExitIp { get; set; } = "";
    public DateTime ObservedAt { get; set; } = DateTime.UtcNow;
}
