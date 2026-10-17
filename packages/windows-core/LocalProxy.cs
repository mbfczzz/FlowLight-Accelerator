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
            // SpeaksSocks5 connects over IPv4, so only listeners that take
            // that: the IPv4 loopback and any-address, and the IPv6
            // any-address, which Go programs open dual-stack by default. A
            // listener on ::1 alone would refuse and cost the two seconds.
            return IPGlobalProperties.GetIPGlobalProperties().GetActiveTcpListeners()
                .Where(e => e.Address.Equals(IPAddress.Loopback) || e.Address.Equals(IPAddress.Any) || e.Address.Equals(IPAddress.IPv6Any))
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
            // Both handles are released; disposing a base key is a no-op.
            using var hive = Registrations(sid);
            using var key = hive?.OpenSubKey(
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
    // A port whose owner cannot be named gives null, and the proxy is not
    // used: guessing from a list of proxy names (as this once did) could name
    // another proxy that happens to run, and the real owner's traffic would
    // go round in that loop.
    public static string? Owner(int port)
    {
        if (!OperatingSystem.IsWindows())
        {
            return null;
        }
        return ProcessName(port);
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

    // Local TCP listeners and the process behind each, as Windows reports
    // them (the same table `netstat -ano` prints), for IPv4 and for IPv6: a
    // dual-stack listener, which Go programs such as Clash open by default,
    // appears only in the IPv6 table. Only the port and owner are read;
    // nothing is changed.
    static List<(int Port, int Pid)> TcpTable()
    {
        var rows = new List<(int, int)>();
        // MIB_TCPROW_OWNER_PID: state, local address, local port, remote
        // address, remote port, pid; six DWORDs.
        Read(AfInet, 24, 8, 20, rows);
        // MIB_TCP6ROW_OWNER_PID: local address (16 bytes), scope, local port,
        // remote address (16), scope, remote port, state, pid.
        Read(AfInet6, 56, 20, 52, rows);
        return rows;
    }

    static void Read(int family, int stride, int portAt, int pidAt, List<(int, int)> rows)
    {
        var size = 0;
        // Sizing the table is a call that FAILS: with a null buffer the API
        // reports ERROR_INSUFFICIENT_BUFFER (122) and fills in the size it
        // needs. Requiring 0 here (as this first did) made every call return
        // an empty table, so the port argument was silently ignored.
        if (GetExtendedTcpTable(IntPtr.Zero, ref size, false, family, TcpTableOwnerPidListener, 0) != InsufficientBuffer || size == 0)
        {
            return;
        }
        var buffer = Marshal.AllocHGlobal(size);
        try
        {
            if (GetExtendedTcpTable(buffer, ref size, false, family, TcpTableOwnerPidListener, 0) != NoError)
            {
                return;
            }
            var count = Marshal.ReadInt32(buffer);
            var first = buffer + 4;
            for (var i = 0; i < count; i++)
            {
                var row = first + i * stride;
                // The port is stored in network byte order in the low half.
                var raw = Marshal.ReadInt32(row + portAt);
                rows.Add((((raw >> 8) & 0xFF) | ((raw & 0xFF) << 8), Marshal.ReadInt32(row + pidAt)));
            }
        }
        finally
        {
            Marshal.FreeHGlobal(buffer);
        }
    }

    const int AfInet = 2;
    const int AfInet6 = 23;
    const int TcpTableOwnerPidListener = 3;
    const uint NoError = 0;
    // Returned by the sizing call, where the buffer is null on purpose.
    const uint InsufficientBuffer = 122;

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
