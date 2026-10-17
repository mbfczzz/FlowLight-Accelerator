using System.Net;
using System.Net.NetworkInformation;
using System.Net.Sockets;
using System.Runtime.InteropServices;
using Microsoft.Win32;

namespace FlowLight.Core;

// Another proxy found on this machine: the port it takes connections on, and
// the process behind that port, whose own connections the kernel sends
// straight out.
public sealed record UpstreamProxy(int Port, string Process);

// Another proxy running on this machine (Clash, v2rayN, sing-box), found so
// the kernel can hand it the traffic that is not the game's. Our own TUN
// carries the whole machine once a session runs, so without this every other
// application would be sent straight out and the other proxy would stop
// working the moment a game is accelerated.
public static class LocalProxy
{
    // Where the other proxy usually listens, most likely first: Clash Verge's
    // mixed port, Clash's HTTP and SOCKS ports, then the common alternatives.
    static readonly int[] Ports = [7897, 7890, 7891, 1080, 10808, 2080, 7899, 8889, 10809, 20171];

    // A proxy in internet settings is the surest hint; the port list is the
    // fallback for proxies that only run in TUN mode, which most do — Clash
    // Verge keeps its mixed port even with the system proxy switched off.
    //
    // sid is the player's Windows account, whose settings the service reads:
    // the service runs as SYSTEM, so its own "current user" is SYSTEM and
    // would never show the player's proxy. Null falls back to the service's
    // own account, which is all a console run has.
    //
    // A proxy is only used when its process can be named too. Its own
    // connections must go straight out — handed to it, they would be handed
    // back again, around and around — and that rule needs the name. Without
    // one, the start is refused as it was before coexisting existed.
    public static UpstreamProxy? Find(string? sid = null)
    {
        if (!OperatingSystem.IsWindows())
        {
            return null;
        }
        var listening = Listening();
        var port = Configured(sid) is { } configured && listening.Contains(configured) && SpeaksSocks5(configured)
            ? configured
            : Find(Ports.Where(listening.Contains));
        if (port is not { } found || Owner(found) is not { } owner || !MihomoKernel.Writable(owner))
        {
            return null;
        }
        return new UpstreamProxy(found, owner);
    }

    // The ports are a parameter so the search can be checked without the
    // machine's own proxies deciding the answer.
    internal static int? Find(IEnumerable<int> ports)
    {
        foreach (var port in ports)
        {
            if (SpeaksSocks5(port))
            {
                return port;
            }
        }
        return null;
    }

    // The loopback ports something listens on. Only those are tried: Windows
    // retries a refused loopback connect for about two seconds, so trying a
    // closed port costs that much, and every start runs this search.
    static HashSet<int> Listening()
    {
        try
        {
            return IPGlobalProperties.GetIPGlobalProperties().GetActiveTcpListeners()
                .Where(e => IPAddress.IsLoopback(e.Address) || e.Address.Equals(IPAddress.Any) || e.Address.Equals(IPAddress.IPv6Any))
                .Select(e => e.Port)
                .ToHashSet();
        }
        catch (NetworkInformationException)
        {
            return [];
        }
    }

    // The port in the given account's proxy settings (the service's own
    // account when sid is null). The value is either host:port or a
    // per-scheme list ("http=host:port;https=host:port").
    static int? Configured(string? sid)
    {
        // The platform test sits here rather than in the caller so the
        // registry APIs are provably guarded.
        if (!OperatingSystem.IsWindows())
        {
            return null;
        }
        try
        {
            using var key = Registrations(sid)?.OpenSubKey(
                @"Software\Microsoft\Windows\CurrentVersion\Internet Settings");
            if (key?.GetValue("ProxyEnable") is not int enabled || enabled == 0)
            {
                return null;
            }
            if (key.GetValue("ProxyServer") is not string server)
            {
                return null;
            }
            foreach (var part in server.Split(';'))
            {
                var value = part.Contains('=') ? part[(part.IndexOf('=') + 1)..] : part;
                var cut = value.LastIndexOf(':');
                if (cut > 0 && int.TryParse(value[(cut + 1)..], out var port) && IsLocal(value[..cut]))
                {
                    return port;
                }
            }
        }
        catch (Exception e) when (e is System.Security.SecurityException or UnauthorizedAccessException or IOException)
        {
        }
        return null;
    }

    // A player's settings live under their own hive; the service's own
    // settings live under the current user. A hive that is not loaded (the
    // player never signed in on this boot) reads as no setting.
    static RegistryKey? Registrations(string? sid)
    {
        // Guarded here as well: the analyzer does not follow the caller's.
        if (!OperatingSystem.IsWindows())
        {
            return null;
        }
        if (string.IsNullOrEmpty(sid))
        {
            return Registry.CurrentUser;
        }
        try
        {
            return Registry.Users.OpenSubKey(sid);
        }
        catch (Exception e) when (e is System.Security.SecurityException or UnauthorizedAccessException or IOException)
        {
            return null;
        }
    }

    // Only this machine's proxies are usable: a proxy elsewhere would be
    // reached through the tunnel and change what the player's traffic does.
    static bool IsLocal(string host)
    {
        var name = host.Trim().Trim('[', ']');
        return name is "localhost" or "127.0.0.1" or "::1" || name.EndsWith(".localhost");
    }

    // The process listening on the upstream port, so the kernel can send its
    // own connections straight out. Without that rule the kernel would hand
    // the proxy's own traffic back to the proxy, which hands it back again.
    // Unknown names fall back to the list of proxy processes.
    static readonly string[] ProxyProcesses =
    [
        "verge-mihomo.exe", "mihomo.exe", "clash-meta.exe", "clash.exe", "clash-win64.exe",
        "sing-box.exe", "v2ray.exe", "v2rayN.exe", "xray.exe", "FlClash.exe", "FlClashHelperService.exe",
        "nekobox.exe", "nekoray.exe", "hiddify.exe", "hiddify-cli.exe", "clash-verge.exe",
    ];

    public static string? Owner(int port)
    {
        if (!OperatingSystem.IsWindows())
        {
            return null;
        }
        return ProcessName(port) ?? KnownRunning();
    }

    // The process holding the port, by asking Windows which TCP endpoint it
    // owns (the same table `netstat -ano` prints), then naming that process.
    static string? ProcessName(int port)
    {
        foreach (var row in TcpTable())
        {
            if (row.Port != port || row.Pid == 0)
            {
                continue;
            }
            try
            {
                using var process = System.Diagnostics.Process.GetProcessById(row.Pid);
                return process.ProcessName + ".exe";
            }
            catch (Exception e) when (e is ArgumentException or InvalidOperationException or System.ComponentModel.Win32Exception)
            {
                return null;
            }
        }
        return null;
    }

    // A proxy process known by name, used when the port's owner cannot be
    // named (a different user's process, or one that just exited).
    static string? KnownRunning()
    {
        foreach (var candidate in ProxyProcesses)
        {
            try
            {
                var name = System.IO.Path.GetFileNameWithoutExtension(candidate);
                if (System.Diagnostics.Process.GetProcessesByName(name).Length > 0)
                {
                    return candidate;
                }
            }
            catch (Exception e) when (e is InvalidOperationException or System.ComponentModel.Win32Exception)
            {
            }
        }
        return null;
    }

    // Local TCP listeners and the process behind each, as Windows reports
    // them. Only the port and owner are read; nothing is changed. The
    // unprivileged table is used, which is enough for a proxy this user runs.
    static List<(int Port, int Pid)> TcpTable()
    {
        var rows = new List<(int, int)>();
        var size = 0;
        // Sizing the table is a call that FAILS: with a null buffer the API
        // reports ERROR_INSUFFICIENT_BUFFER (122) and fills in the size it
        // needs. Requiring 0 here (as this first did) made every call return
        // an empty table, so the port argument was silently ignored and the
        // caller always fell back to scanning names.
        if (GetExtendedTcpTable(IntPtr.Zero, ref size, false, AfInet, TcpTableOwnerPidListener, 0) != InsufficientBuffer || size == 0)
        {
            return rows;
        }
        var buffer = Marshal.AllocHGlobal(size);
        try
        {
            if (GetExtendedTcpTable(buffer, ref size, false, AfInet, TcpTableOwnerPidListener, 0) != NoError)
            {
                return rows;
            }
            var count = Marshal.ReadInt32(buffer);
            var at = buffer + 4;
            var stride = Marshal.SizeOf<MibTcpRowOwnerPid>();
            for (var i = 0; i < count; i++)
            {
                var row = Marshal.PtrToStructure<MibTcpRowOwnerPid>(at + i * stride);
                // The port is stored in network byte order in the low half.
                var port = (int)((row.LocalPort >> 8) | ((row.LocalPort & 0xFF) << 8));
                rows.Add((port, (int)row.OwningPid));
            }
        }
        finally
        {
            Marshal.FreeHGlobal(buffer);
        }
        return rows;
    }

    const int AfInet = 2;
    const int TcpTableOwnerPidListener = 3;
    const uint NoError = 0;
    // Returned by the sizing call, where the buffer is null on purpose.
    const uint InsufficientBuffer = 122;

    [StructLayout(LayoutKind.Sequential)]
    struct MibTcpRowOwnerPid
    {
        public uint State;
        public uint LocalAddr;
        public uint LocalPort;
        public uint RemoteAddr;
        public uint RemotePort;
        public uint OwningPid;
    }

    [DllImport("iphlpapi.dll", SetLastError = true)]
    static extern uint GetExtendedTcpTable(IntPtr table, ref int size, bool order, int family, int type, uint reserved);

    // Whether something on this port answers a SOCKS5 greeting, which is what
    // the kernel needs to hand it a connection. A mixed port (Clash's) speaks
    // both, so this finds those too. A port that is closed, or served by
    // something else entirely, is not taken.
    static bool SpeaksSocks5(int port)
    {
        if (port is < 1 or > 65535)
        {
            return false;
        }
        try
        {
            using var client = new TcpClient();
            client.Connect(IPAddress.Loopback, port);
            using var stream = client.GetStream();
            stream.ReadTimeout = 500;
            stream.WriteTimeout = 500;
            // Only "no authentication" is offered, and only a proxy that
            // picks it is used. A proxy demanding a login would accept our
            // connections and then reject the traffic, since the kernel's
            // upstream entry has no credentials to give it; better to leave
            // it alone and keep everything going straight out.
            stream.Write([5, 1, 0]);
            var reply = new byte[2];
            var read = stream.Read(reply, 0, 2);
            return read == 2 && reply[0] == 5 && reply[1] == 0;
        }
        catch (Exception e) when (e is SocketException or IOException or ArgumentException or InvalidOperationException)
        {
            return false;
        }
    }
}
