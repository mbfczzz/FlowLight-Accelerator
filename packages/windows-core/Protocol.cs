using System.ComponentModel;
using System.IO.Pipes;
using System.Runtime.InteropServices;
using System.Security.Principal;
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
// the service's own error text. A RetryableRefusalException is the same, for
// a refusal that clears by itself: the caller waits and tries again instead
// of showing the player a dead end.
public sealed class ServiceErrorException(string message) : InvalidOperationException(message);

public sealed class RetryableRefusalException(string message, string code) : InvalidOperationException(message)
{
    public string Code { get; } = code;
}

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
    internal const string ServiceName = "FlowLightAccelerator";
    const int ConnectTimeoutMs = 3000;

    // Queries answer at once. Commands that change the session wait for the
    // service's own work: stop and logout wind the session down (kernel stop
    // plus the control-service release, up to ~20 s) and login makes two
    // control calls under the service's 30 s dispatch budget.
    const int QueryTimeoutMs = 10000;
    const int CommandTimeoutMs = 35000;

    static int ResponseTimeoutMs(string command) =>
        command is "login" or "start" or "stop" or "logout" ? CommandTimeoutMs : QueryTimeoutMs;

    public static readonly JsonSerializerOptions Json = new()
    {
        PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower,
        PropertyNameCaseInsensitive = true,
    };

    public static Task<JsonElement> CallAsync(string command, object? args = null, CancellationToken ct = default) =>
        CallAsync(ClientPipeName(), command, args, ct);

    // Development only. FLOWLIGHT_DEV_PIPE=1 already disables the service
    // identity check; FLOWLIGHT_PIPE_NAME then lets a stand-in service be
    // addressed while the real one keeps its pipe, so UI flows can be driven
    // end to end without stopping the installed service.
    static string ClientPipeName()
    {
        if (Environment.GetEnvironmentVariable("FLOWLIGHT_DEV_PIPE") == "1" &&
            Environment.GetEnvironmentVariable("FLOWLIGHT_PIPE_NAME") is { Length: > 0 } name)
        {
            return name;
        }
        return PipeName;
    }

    // The pipe name is a parameter so the checks can drive the failure paths
    // against a pipe that is certainly absent while the real service runs.
    internal static async Task<JsonElement> CallAsync(
        string pipeName,
        string command,
        object? args,
        CancellationToken ct)
    {
        // Connecting to a pipe nobody serves blocks for the whole connect
        // timeout; the namespace answers at once.
        if (!PipeExists(pipeName))
        {
            throw new ServiceUnavailableException("Local service is not running");
        }

        using var pipe = new NamedPipeClientStream(
            ".",
            pipeName,
            PipeDirection.InOut,
            PipeOptions.Asynchronous,
            TokenImpersonationLevel.Impersonation);
        try
        {
            await pipe.ConnectAsync(ConnectTimeoutMs, ct);
        }
        catch (TimeoutException e) when (PipeExists(pipeName))
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

        // A service that accepted the connection but never answers (hung, or
        // killed mid-request) must not stall the caller for good.
        using var deadline = CancellationTokenSource.CreateLinkedTokenSource(ct);
        var timeoutMs = ResponseTimeoutMs(command);
        deadline.CancelAfter(timeoutMs);
        string? line;
        try
        {
            using var writer = new StreamWriter(pipe, leaveOpen: true) { AutoFlush = true };
            using var reader = new StreamReader(pipe, leaveOpen: true);
            var request = JsonSerializer.Serialize(new { command, args }, Json);
            await writer.WriteLineAsync(request.AsMemory(), deadline.Token);
            line = await reader.ReadLineAsync(deadline.Token);
        }
        catch (OperationCanceledException e) when (!ct.IsCancellationRequested)
        {
            throw new ServiceUnavailableException(
                $"Local service did not respond within {timeoutMs / 1000} seconds",
                e);
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
            var message = string.IsNullOrWhiteSpace(error) ? "本地服务请求失败" : error;
            // The service passes the control service's retry code through,
            // so the window can wait it out rather than report a dead end.
            var code = root.TryGetProperty("code", out var tag) && tag.ValueKind == JsonValueKind.String
                ? tag.GetString()
                : null;
            if (code is { Length: > 0 } retryCode)
            {
                throw new RetryableRefusalException(message, retryCode);
            }
            throw new ServiceErrorException(message);
        }
        return root.GetProperty("data").Clone();
    }

    // The pipe must be served by the registered service, and the service must
    // be registered at its release location. The client runs as the signed-in
    // user and cannot open the SYSTEM service process to read its image path,
    // but the Service Control Manager answers both questions for any
    // authenticated user, as `sc.exe qc` does.
    internal static void VerifyServer(NamedPipeClientStream pipe)
    {
        if (Environment.GetEnvironmentVariable("FLOWLIGHT_DEV_PIPE") == "1")
        {
            return;
        }
        if (!GetNamedPipeServerProcessId(pipe.SafePipeHandle, out var pid))
        {
            throw new IOException("无法确认本地服务身份");
        }
        (uint ProcessId, string BinaryPath)? service;
        try
        {
            service = QueryService(ServiceName);
        }
        catch (Win32Exception e)
        {
            throw new IOException("无法确认本地服务身份", e);
        }
        if (service is not { } registered ||
            registered.ProcessId == 0 ||
            registered.ProcessId != pid ||
            !IsExpectedServicePath(registered.BinaryPath))
        {
            throw new IOException("本地服务不是正式安装版本，请从安装包重新安装");
        }
    }

    // Whether a binary path as registered with the Service Control Manager is
    // the release location of the service. The path may be quoted and
    // followed by arguments. An unquoted path is taken whole: the expected
    // path contains spaces, so unquoted arguments could not be told apart.
    internal static bool IsExpectedServicePath(string binaryPathFromScm)
    {
        var path = binaryPathFromScm.Trim();
        if (path.StartsWith('"'))
        {
            var close = path.IndexOf('"', 1);
            if (close < 0)
            {
                return false;
            }
            path = path[1..close];
        }
        if (path.Length == 0 || !Path.IsPathFullyQualified(path))
        {
            return false;
        }
        var expected = Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.ProgramFiles),
            "FlowLight Accelerator",
            "service",
            "FlowLight.Service.exe");
        try
        {
            return string.Equals(Path.GetFullPath(path), expected, StringComparison.OrdinalIgnoreCase);
        }
        catch (Exception e) when (e is ArgumentException or IOException)
        {
            return false;
        }
    }

    // What the Service Control Manager knows about service `name`: the
    // process running it (0 while it is stopped) and the binary path it was
    // registered with. Null when no such service is registered.
    internal static (uint ProcessId, string BinaryPath)? QueryService(string name)
    {
        using var manager = OpenSCManager(null, null, ScManagerConnect);
        if (manager.IsInvalid)
        {
            throw new Win32Exception();
        }
        using var service = OpenService(manager, name, ServiceQueryConfig | ServiceQueryStatus);
        if (service.IsInvalid)
        {
            var error = Marshal.GetLastWin32Error();
            if (error == ErrorServiceDoesNotExist)
            {
                return null;
            }
            throw new Win32Exception(error);
        }

        var status = new ServiceStatusProcess();
        var statusSize = (uint)Marshal.SizeOf<ServiceStatusProcess>();
        if (!QueryServiceStatusEx(service, ScStatusProcessInfo, ref status, statusSize, out _))
        {
            throw new Win32Exception();
        }

        // The configuration is variable-sized; the first call reports the size.
        if (QueryServiceConfig(service, IntPtr.Zero, 0, out var needed) ||
            Marshal.GetLastWin32Error() != ErrorInsufficientBuffer)
        {
            throw new Win32Exception();
        }
        var buffer = Marshal.AllocHGlobal((int)needed);
        try
        {
            if (!QueryServiceConfig(service, buffer, needed, out _))
            {
                throw new Win32Exception();
            }
            var config = Marshal.PtrToStructure<QueryServiceConfigW>(buffer);
            return (status.ProcessId, Marshal.PtrToStringUni(config.BinaryPathName) ?? "");
        }
        finally
        {
            Marshal.FreeHGlobal(buffer);
        }
    }

    const uint ScManagerConnect = 0x0001;
    const uint ServiceQueryConfig = 0x0001;
    const uint ServiceQueryStatus = 0x0004;
    const int ScStatusProcessInfo = 0;
    const int ErrorInsufficientBuffer = 122;
    const int ErrorServiceDoesNotExist = 1060;

    [StructLayout(LayoutKind.Sequential)]
    struct ServiceStatusProcess
    {
        public uint ServiceType;
        public uint CurrentState;
        public uint ControlsAccepted;
        public uint Win32ExitCode;
        public uint ServiceSpecificExitCode;
        public uint CheckPoint;
        public uint WaitHint;
        public uint ProcessId;
        public uint ServiceFlags;
    }

    [StructLayout(LayoutKind.Sequential)]
    struct QueryServiceConfigW
    {
        public uint ServiceType;
        public uint StartType;
        public uint ErrorControl;
        public IntPtr BinaryPathName;
        public IntPtr LoadOrderGroup;
        public uint TagId;
        public IntPtr Dependencies;
        public IntPtr ServiceStartName;
        public IntPtr DisplayName;
    }

    sealed class SafeServiceHandle() : SafeHandleZeroOrMinusOneIsInvalid(true)
    {
        protected override bool ReleaseHandle() => CloseServiceHandle(handle);
    }

    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool GetNamedPipeServerProcessId(SafePipeHandle pipe, out uint pid);

    [DllImport("advapi32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    static extern SafeServiceHandle OpenSCManager(string? machine, string? database, uint access);

    [DllImport("advapi32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    static extern SafeServiceHandle OpenService(SafeServiceHandle manager, string name, uint access);

    [DllImport("advapi32.dll", SetLastError = true)]
    static extern bool QueryServiceStatusEx(
        SafeServiceHandle service,
        int infoLevel,
        ref ServiceStatusProcess status,
        uint size,
        out uint needed);

    [DllImport("advapi32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    static extern bool QueryServiceConfig(SafeServiceHandle service, IntPtr config, uint size, out uint needed);

    [DllImport("advapi32.dll", SetLastError = true)]
    static extern bool CloseServiceHandle(IntPtr handle);
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
    string? ExpectedExit,
    // The exit's latency echo (on the exit's loopback, reached through the
    // tunnel); absent where the exit's node program predates it.
    string? ExitEcho = null);

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
