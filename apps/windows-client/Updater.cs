using System.Diagnostics;
using System.IO;
using System.Net;
using System.Net.Http;
using System.Security.Cryptography;
using System.Text.Json;

namespace FlowLight.Client;

// A client release published in the admin's 客户端版本.
sealed record ClientRelease(string Version, Uri Url, string Sha256, bool Required);

// Finds, downloads and starts a newer client. The control service names the
// installer and its SHA-256; nothing is run unless the downloaded file
// matches. The installer replaces the client and the service and keeps the
// control address, so an update needs no choices from the player.
static class Updater
{
    static readonly HttpClient Http = new() { Timeout = Timeout.InfiniteTimeSpan };
    static readonly string Folder = Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
        "FlowLightAccelerator",
        "updates");

    // The newest published stable release, if it is newer than `current`.
    public static async Task<ClientRelease?> Check(Uri control, string current, CancellationToken ct)
    {
        using var timeout = CancellationTokenSource.CreateLinkedTokenSource(ct);
        timeout.CancelAfter(TimeSpan.FromSeconds(20));
        var uri = new Uri(control, "api/client/v1/releases/latest?current=" + Uri.EscapeDataString(current));
        using var response = await Http.GetAsync(uri, timeout.Token);
        if (response.StatusCode == HttpStatusCode.NotFound)
        {
            return null;
        }
        response.EnsureSuccessStatusCode();
        using var doc = JsonDocument.Parse(await response.Content.ReadAsStringAsync(timeout.Token));
        var root = doc.RootElement;
        var version = root.TryGetProperty("version", out var v) ? v.GetString() : null;
        var url = root.TryGetProperty("url", out var u) ? u.GetString() : null;
        var sha = root.TryGetProperty("sha256", out var s) ? s.GetString()?.ToLowerInvariant() : null;
        var required = root.TryGetProperty("required", out var r) && r.ValueKind == JsonValueKind.True;
        if (version is null || sha is not { Length: 64 } || !sha.All(Uri.IsHexDigit)
            || !Uri.TryCreate(url, UriKind.Absolute, out var link) || link.Scheme != Uri.UriSchemeHttps)
        {
            throw new InvalidDataException("控制服务返回的版本信息不完整");
        }
        return Compare(version, current) > 0 ? new ClientRelease(version, link, sha, required) : null;
    }

    // Dotted numeric versions, as the control service orders them: a leading
    // "v" and anything after "-" or "+" are ignored.
    public static int Compare(string a, string b)
    {
        static int[] Parts(string v)
        {
            v = v.Trim().TrimStart('v', 'V');
            var cut = v.IndexOfAny(['-', '+']);
            if (cut >= 0)
            {
                v = v[..cut];
            }
            return v.Split('.').Select(p => int.TryParse(p, out var n) ? n : 0).ToArray();
        }
        int[] x = Parts(a), y = Parts(b);
        for (var i = 0; i < Math.Max(x.Length, y.Length); i++)
        {
            var c = (i < x.Length ? x[i] : 0).CompareTo(i < y.Length ? y[i] : 0);
            if (c != 0)
            {
                return c;
            }
        }
        return 0;
    }

    static string PathOf(ClientRelease release) =>
        Path.Combine(Folder, "FlowLight-Accelerator-Setup-" + release.Sha256[..12] + ".exe");

    // Downloads the installer, reporting (received, total) bytes; total is
    // 0 when the server does not say. A file already downloaded and intact
    // is used as it is.
    public static async Task<string> Download(ClientRelease release, IProgress<(long Received, long Total)> progress, CancellationToken ct)
    {
        var path = PathOf(release);
        if (File.Exists(path) && await HashOf(path, ct) == release.Sha256)
        {
            return path;
        }
        Directory.CreateDirectory(Folder);
        foreach (var old in Directory.EnumerateFiles(Folder))
        {
            try { File.Delete(old); } catch (IOException) { } catch (UnauthorizedAccessException) { }
        }
        var temp = path + ".part";
        using (var response = await Http.GetAsync(release.Url, HttpCompletionOption.ResponseHeadersRead, ct))
        {
            response.EnsureSuccessStatusCode();
            var total = response.Content.Headers.ContentLength ?? 0;
            await using var source = await response.Content.ReadAsStreamAsync(ct);
            await using var target = new FileStream(temp, FileMode.Create, FileAccess.Write, FileShare.None, 1 << 16, true);
            var buffer = new byte[1 << 16];
            long received = 0, reported = 0;
            int read;
            while ((read = await ReadWithIdleLimit(source, buffer, ct)) > 0)
            {
                await target.WriteAsync(buffer.AsMemory(0, read), ct);
                received += read;
                if (received - reported >= 1 << 18)
                {
                    reported = received;
                    progress.Report((received, total));
                }
            }
            progress.Report((received, total));
        }
        if (await HashOf(temp, ct) != release.Sha256)
        {
            File.Delete(temp);
            throw new InvalidDataException("下载的安装包校验不一致，已删除。请稍后重试，或联系管理员核对版本信息。");
        }
        File.Move(temp, path, overwrite: true);
        return path;
    }

    // A stalled connection fails after a minute instead of hanging forever.
    static async Task<int> ReadWithIdleLimit(Stream source, byte[] buffer, CancellationToken ct)
    {
        using var idle = CancellationTokenSource.CreateLinkedTokenSource(ct);
        idle.CancelAfter(TimeSpan.FromSeconds(60));
        try
        {
            return await source.ReadAsync(buffer, idle.Token);
        }
        catch (OperationCanceledException) when (!ct.IsCancellationRequested)
        {
            throw new TimeoutException("下载超过 1 分钟没有进展，请检查网络后重试");
        }
    }

    static async Task<string> HashOf(string path, CancellationToken ct)
    {
        await using var file = new FileStream(path, FileMode.Open, FileAccess.Read, FileShare.Read, 1 << 16, true);
        return Convert.ToHexStringLower(await SHA256.HashDataAsync(file, ct));
    }

    // Starts the installer. The file is held open against writes from the
    // hash check until Windows has started it, so it cannot be swapped in
    // between. Setup asks for administrator rights itself — started that
    // way, what it reopens at the end runs as the player, not elevated —
    // and its launcher stays until the installation has ended, so the
    // caller can tell a declined prompt from an installation under way: the
    // former ends the launcher while this client is still here, the latter
    // closes this client before replacing its files.
    public static Process Launch(string path, string sha256)
    {
        using var held = new FileStream(path, FileMode.Open, FileAccess.Read, FileShare.Read);
        if (Convert.ToHexStringLower(SHA256.HashData(held)) != sha256)
        {
            throw new InvalidDataException("安装包在下载后被改动过，已停止更新。");
        }
        // /LAUNCH=1 has the installer open the new client when it is done.
        return Process.Start(new ProcessStartInfo(path, "/SILENT /SUPPRESSMSGBOXES /NORESTART /LAUNCH=1")
        {
            UseShellExecute = true,
            WorkingDirectory = Path.GetDirectoryName(path)!,
        }) ?? throw new InvalidOperationException("安装程序没有启动");
    }
}
