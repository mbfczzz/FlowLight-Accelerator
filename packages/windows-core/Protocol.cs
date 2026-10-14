using System.IO.Pipes;
using System.Text.Json;
using System.Runtime.InteropServices;
using System.Text;
using Microsoft.Win32.SafeHandles;
namespace FlowLight.Core;

public static class Wire
{
    public const string PipeName = "FlowLight.Accelerator.v1";
    public static readonly JsonSerializerOptions Json = new() { PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower, PropertyNameCaseInsensitive = true };
    public static async Task<JsonElement> CallAsync(string command, object? args = null, CancellationToken ct = default)
    {
        using var pipe = new NamedPipeClientStream(".", PipeName, PipeDirection.InOut, PipeOptions.Asynchronous, System.Security.Principal.TokenImpersonationLevel.Impersonation);
        await pipe.ConnectAsync(3000, ct);
        VerifyServer(pipe);
        using var writer = new StreamWriter(pipe, leaveOpen: true) { AutoFlush = true };
        using var reader = new StreamReader(pipe, leaveOpen: true);
        await writer.WriteLineAsync(JsonSerializer.Serialize(new { command, args }, Json));
        var line = await reader.ReadLineAsync(ct) ?? throw new IOException("Local service did not respond");
        using var doc = JsonDocument.Parse(line); var root = doc.RootElement;
        if (!root.GetProperty("success").GetBoolean()) throw new InvalidOperationException(root.GetProperty("error").GetString());
        return root.GetProperty("data").Clone();
    }
    static void VerifyServer(NamedPipeClientStream pipe)
    {
        if (Environment.GetEnvironmentVariable("FLOWLIGHT_DEV_PIPE") == "1") return;
        if (!GetNamedPipeServerProcessId(pipe.SafePipeHandle, out var pid)) throw new IOException("Cannot verify local service identity");
        using var handle = OpenProcess(0x1000, false, pid);
        var path = new StringBuilder(32768); uint length = (uint)path.Capacity;
        if (handle.IsInvalid || !QueryFullProcessImageName(handle, 0, path, ref length)) throw new IOException("Cannot verify local service executable");
        var expected = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ProgramFiles), "FlowLight Accelerator", "service", "FlowLight.Service.exe");
        if (!Path.GetFullPath(path.ToString()).Equals(expected, StringComparison.OrdinalIgnoreCase)) throw new IOException("Untrusted local service; install the verified release in Program Files");
    }
    [DllImport("kernel32.dll", SetLastError = true)] static extern bool GetNamedPipeServerProcessId(SafePipeHandle pipe, out uint pid);
    [DllImport("kernel32.dll", SetLastError = true)] static extern SafeProcessHandle OpenProcess(uint access, bool inherit, uint pid);
    [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)] static extern bool QueryFullProcessImageName(SafeProcessHandle process, uint flags, StringBuilder path, ref uint size);
}
public sealed record GameProfile(string[] Processes, string[] Paths, string[] Domains, string[] Cidrs, string[] ExcludeDomains, bool Udp);
public sealed record Connection(string Host, int Port, string Cipher, string Password, bool Udp, GameProfile Profile, string ProbeUrl, string ProbeUdp, string? ExpectedExit);
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
