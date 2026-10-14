using System.Net;
using System.Net.Sockets;
using System.Text;
using System.Text.Json;
using FlowLight.Core;
using System.Diagnostics;
if (args.Contains("--owned-child")) { Thread.Sleep(Timeout.Infinite); return; }
static void Check(bool value, string message) { if (!value) throw new Exception(message); }
try { using var bad = new ApiClient("http://public.example"); throw new Exception("insecure API accepted"); } catch (ArgumentException) { }
var profile = new GameProfile(["VALORANT-Win64-Shipping.exe"], [], [], [], [], true);
var conn = new Connection("node.example", 20000, "2022-blake3-aes-128-gcm", "test-only", true, profile, "https://probe.example/probe", "probe.example:9081", null);
using (var data = JsonDocument.Parse(JsonSerializer.Serialize(conn, Wire.Json)))
{
    Check(data.RootElement.GetProperty("probe_url").GetString() == conn.ProbeUrl, "contract casing changed");
    Check(data.RootElement.GetProperty("profile").GetProperty("udp").GetBoolean(), "UDP requirement lost");
}
try { MihomoKernel.BuildConfig(conn with { Profile = profile with { Processes = ["bad,rule"] } }, 10900); throw new Exception("rule injection accepted"); } catch (ArgumentException) { }
var listenerPort = new TcpListener(IPAddress.Loopback, 0); listenerPort.Start(); var port = ((IPEndPoint)listenerPort.LocalEndpoint).Port; listenerPort.Stop();
using var listener = new HttpListener(); listener.Prefixes.Add($"http://127.0.0.1:{port}/"); listener.Start();
var server = Task.Run(async () =>
{
    for (var i = 0; i < 6; i++)
    {
        var ctx = await listener.GetContextAsync(); var path = ctx.Request.Url!.AbsolutePath; string payload;
        if (i == 0) { Check(path.EndsWith("/auth/login"), "login path"); payload = """{"access_token":"first","refresh_token":"refresh","user":{"id":"owner"}}"""; }
        else if (i == 1) { Check(ctx.Request.Headers["Authorization"] == "Bearer first", "access token missing"); ctx.Response.StatusCode = 401; payload = """{"error":"expired"}"""; }
        else if (i == 2) { Check(path.EndsWith("/auth/refresh"), "refresh path"); payload = """{"access_token":"second","refresh_token":"rotated","user":{"id":"owner"}}"""; }
        else if (i == 3) { Check(ctx.Request.Headers["Authorization"] == "Bearer second", "refresh not applied"); payload = "[]"; }
        else if (i == 4) { Check(path.EndsWith("/sessions"), "session path"); Check(ctx.Request.Headers["Idempotency-Key"] == "request-stable-1", "idempotency key missing"); using var reader = new StreamReader(ctx.Request.InputStream); var body = await reader.ReadToEndAsync(); Check(body.Contains("game_id"), "session contract changed"); payload = """{"id":"session-1"}"""; }
        else { ctx.Response.StatusCode = 503; payload = "gateway unavailable"; }
        var bytes = Encoding.UTF8.GetBytes(payload); ctx.Response.ContentType = "application/json"; await ctx.Response.OutputStream.WriteAsync(bytes); ctx.Response.Close();
    }
});
using (var api = new ApiClient($"http://127.0.0.1:{port}", true))
{
    using var timeout = new CancellationTokenSource(TimeSpan.FromSeconds(15));
    await api.Login("tester", "fixture-password", timeout.Token);
    var games = await api.Send("games", ct: timeout.Token); Check(games.ValueKind == JsonValueKind.Array, "catalog decode");
    var session = await api.Send("sessions", HttpMethod.Post, new { game_id = "valorant" }, "request-stable-1", timeout.Token);
    Check(session.GetProperty("id").GetString() == "session-1", "session decode");
    try { await api.Send("games", ct: timeout.Token); throw new Exception("503 response accepted"); } catch (HttpRequestException e) { Check(e.StatusCode == HttpStatusCode.ServiceUnavailable, "503 not classified as transient"); }
}
await server.WaitAsync(TimeSpan.FromSeconds(15));
if (OperatingSystem.IsWindows())
{
    var start = new ProcessStartInfo(Environment.ProcessPath!) { UseShellExecute = false, CreateNoWindow = true };
    if (string.Equals(Path.GetFileNameWithoutExtension(Environment.ProcessPath), "dotnet", StringComparison.OrdinalIgnoreCase)) start.ArgumentList.Add(System.Reflection.Assembly.GetExecutingAssembly().Location);
    start.ArgumentList.Add("--owned-child");
    using var child = Process.Start(start)!;
    try { using (var job = new OwnedProcessJob()) { job.Add(child); } Check(child.WaitForExit(5000), "owned network process survived job disposal"); }
    finally { if (!child.HasExited) child.Kill(true); }
}
Console.WriteLine("PASS: HTTPS restriction, wire contract, rule validation, token rotation, stable idempotency, transient gateway errors, Windows child lifetime.");
