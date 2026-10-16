using System.Net;
using System.Net.NetworkInformation;
using System.Net.Sockets;
using System.Runtime.InteropServices;

namespace FlowLight.Core;

// Another proxy's TUN mode (Clash, sing-box, v2rayN...) takes the default
// route and answers DNS with fake addresses from 198.18.0.0/15. The kernel's
// connection to the line's entry then resolves to a fake address and is
// captured by the other proxy, so the tunnel never reaches the line and the
// start fails with an unexplained probe error. It is found before a session
// is created, and named.
public static class NetworkConflict
{
    // What is known of one network adapter: whether Windows routes internet
    // traffic through it, and its addresses, gateways and DNS servers.
    public sealed record Adapter(string Name, string Description, bool CarriesInternet, IReadOnlyList<IPAddress> Addresses);

    // Public addresses whose route shows which adapter carries the internet.
    static readonly IPAddress[] RouteTargets = [IPAddress.Parse("223.5.5.5"), IPAddress.Parse("8.8.8.8")];

    // Adapter names and descriptions proxy software gives its TUN, and the
    // name players know it by. Our own kernel's adapter is FlowLightTun.
    static readonly (string Pattern, string Software)[] Known =
    [
        ("Meta Tunnel", "Clash"),
        ("Clash", "Clash"),
        ("mihomo", "Clash"),
        ("Verge", "Clash"),
        ("FlClash", "Clash"),
        ("sing-box", "sing-box / v2rayN"),
        ("singbox", "sing-box / v2rayN"),
        ("sing_box", "sing-box / v2rayN"),
        ("v2ray", "v2rayN"),
        ("xray", "v2rayN"),
        ("nekoray", "NekoRay"),
        ("throne", "NekoRay"),
        ("hiddify", "Hiddify"),
        ("tun2socks", "代理软件"),
        ("Wintun", "代理软件"),
    ];

    // The fake-IP range Clash and sing-box answer DNS with by default.
    public static bool IsFakeIp(IPAddress a)
    {
        if (a.AddressFamily != AddressFamily.InterNetwork)
        {
            return false;
        }
        var b = a.GetAddressBytes();
        return b[0] == 198 && (b[1] & 0xFE) == 18;
    }

    // The player-facing reason the start cannot work, or null.
    public static string? Find(IEnumerable<Adapter> adapters)
    {
        foreach (var a in adapters)
        {
            if (!a.CarriesInternet || Ours(a))
            {
                continue;
            }
            var software = Known
                .Where(k => a.Name.Contains(k.Pattern, StringComparison.OrdinalIgnoreCase) ||
                            a.Description.Contains(k.Pattern, StringComparison.OrdinalIgnoreCase))
                .Select(k => k.Software)
                .FirstOrDefault();
            if (software is null && !a.Addresses.Any(IsFakeIp))
            {
                continue;
            }
            software ??= "代理软件";
            return $"检测到 {software} 的 TUN 模式（虚拟网卡「{a.Name}」）正在接管本机网络，会和加速器冲突：" +
                   $"请在 {software} 里关闭 TUN 模式（或先退出它），再开始加速";
        }
        return null;
    }

    static bool Ours(Adapter a) => a.Name.Contains("FlowLight", StringComparison.OrdinalIgnoreCase);

    // Find, on this machine.
    public static string? Detect()
    {
        if (!OperatingSystem.IsWindows())
        {
            return null;
        }
        var carrying = new HashSet<int>();
        foreach (var target in RouteTargets)
        {
            if (GetBestInterface(BitConverter.ToUInt32(target.GetAddressBytes(), 0), out var index) == 0)
            {
                carrying.Add((int)index);
            }
        }
        var adapters = new List<Adapter>();
        foreach (var n in NetworkInterface.GetAllNetworkInterfaces())
        {
            if (n.OperationalStatus != OperationalStatus.Up || n.NetworkInterfaceType == NetworkInterfaceType.Loopback)
            {
                continue;
            }
            try
            {
                var p = n.GetIPProperties();
                var index = p.GetIPv4Properties()?.Index;
                var addresses = p.UnicastAddresses.Select(u => u.Address)
                    .Concat(p.GatewayAddresses.Select(g => g.Address))
                    .Concat(p.DnsAddresses)
                    .ToList();
                adapters.Add(new Adapter(n.Name, n.Description, index is { } i && carrying.Contains(i), addresses));
            }
            catch (NetworkInformationException)
            {
                // An adapter without IPv4 carries no IPv4 route.
            }
        }
        return Find(adapters);
    }

    [DllImport("iphlpapi.dll")]
    static extern int GetBestInterface(uint destination, out uint index);
}
