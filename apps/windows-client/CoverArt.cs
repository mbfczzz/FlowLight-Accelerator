using System.IO;
using System.Net.Http;
using System.Security.Cryptography;
using System.Windows.Media.Imaging;

namespace FlowLight.Client;

// Cover pictures uploaded in the admin. The control service serves each
// one under the SHA-256 of its bytes, so a download is checked against
// its name and then kept on disk for good; a picture that cannot be had
// leaves the generated cover in place.
static class CoverArt
{
    static readonly HttpClient Http = new()
    {
        Timeout = TimeSpan.FromSeconds(20),
        MaxResponseContentBufferSize = 4 << 20,
    };
    static readonly string Folder = Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
        "FlowLightAccelerator",
        "covers");
    static readonly Dictionary<string, Task<BitmapSource?>> Loads = new();
    // Downloads wait until the service has said where the control service is.
    static readonly TaskCompletionSource<Uri> ServerKnown = new(TaskCreationOptions.RunContinuationsAsynchronously);
    static Uri? server;

    // The same rule as the service's API client: HTTPS, or HTTP to this machine.
    public static void SetServer(Uri control)
    {
        if (control.Scheme != Uri.UriSchemeHttps && !(control.Scheme == Uri.UriSchemeHttp && control.IsLoopback))
        {
            return;
        }
        server = new Uri(control.AbsoluteUri.TrimEnd('/') + "/");
        ServerKnown.TrySetResult(server);
    }

    public static bool IsName(string? name) =>
        name is { Length: 64 } && name.All(c => c is >= '0' and <= '9' or >= 'a' and <= 'f');

    // A card decodes at a card's size; the game page banner at full size.
    public static Task<BitmapSource?> Load(string name, bool full)
    {
        var key = name + (full ? "/full" : "/card");
        lock (Loads)
        {
            // A failed attempt is tried again the next time the cover is shown.
            if (!Loads.TryGetValue(key, out var load) || load.IsCompletedSuccessfully && load.Result is null)
            {
                load = Fetch(name, full ? 0 : 640);
                Loads[key] = load;
            }
            return load;
        }
    }

    static async Task<BitmapSource?> Fetch(string name, int width)
    {
        try
        {
            var bytes = await Cached(name) ?? await Download(name);
            return bytes is null ? null : await Task.Run(() => Decode(bytes, width));
        }
        catch (Exception)
        {
            return null;
        }
    }

    static string PathOf(string name) => Path.Combine(Folder, name + ".jpg");

    static bool Matches(byte[] bytes, string name) =>
        Convert.ToHexStringLower(SHA256.HashData(bytes)) == name;

    static async Task<byte[]?> Cached(string name)
    {
        var path = PathOf(name);
        if (!File.Exists(path))
        {
            return null;
        }
        var bytes = await File.ReadAllBytesAsync(path);
        if (Matches(bytes, name))
        {
            return bytes;
        }
        File.Delete(path);
        return null;
    }

    static async Task<byte[]?> Download(string name)
    {
        var root = server ?? await ServerKnown.Task;
        var bytes = await Http.GetByteArrayAsync(new Uri(root, "api/client/v1/covers/" + name));
        if (!Matches(bytes, name))
        {
            return null;
        }
        Directory.CreateDirectory(Folder);
        var temp = PathOf(name) + "." + Environment.ProcessId + ".tmp";
        await File.WriteAllBytesAsync(temp, bytes);
        File.Move(temp, PathOf(name), overwrite: true);
        return bytes;
    }

    static BitmapSource Decode(byte[] bytes, int width)
    {
        var image = new BitmapImage();
        image.BeginInit();
        image.CacheOption = BitmapCacheOption.OnLoad;
        image.CreateOptions = BitmapCreateOptions.IgnoreColorProfile;
        image.StreamSource = new MemoryStream(bytes);
        if (width > 0)
        {
            image.DecodePixelWidth = width;
        }
        image.EndInit();
        image.Freeze();
        return image;
    }
}
