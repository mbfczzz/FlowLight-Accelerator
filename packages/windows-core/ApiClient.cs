using System.Net;
using System.Net.Http.Headers;
using System.Text;
using System.Text.Json;

namespace FlowLight.Core;

public sealed class ApiClient : IDisposable
{
    readonly HttpClient http;
    readonly SemaphoreSlim refreshGate = new(1, 1);
    string refreshToken = "";
    long serverOffsetTicks;

    public ApiClient(string baseUrl, bool allowLocal = false)
    {
        var uri = new Uri(baseUrl);
        if (uri.Scheme != "https" && !(allowLocal && uri.IsLoopback && uri.Scheme == "http"))
        {
            throw new ArgumentException("Control service requires HTTPS");
        }
        http = new HttpClient
        {
            BaseAddress = new Uri(baseUrl.TrimEnd('/') + "/"),
            Timeout = TimeSpan.FromSeconds(20),
        };
    }

    public string AccessToken { get; private set; } = "";

    public DateTime LocalDeadline(DateTime serverDeadline)
    {
        var offset = TimeSpan.FromTicks(Interlocked.Read(ref serverOffsetTicks));
        return serverDeadline.ToUniversalTime() - offset - TimeSpan.FromSeconds(1);
    }

    public async Task<JsonElement> Login(string username, string password, CancellationToken ct)
    {
        var result = await Send("auth/login", HttpMethod.Post, new { username, password }, ct: ct, refresh: false);
        AccessToken = result.GetProperty("access_token").GetString()!;
        refreshToken = result.GetProperty("refresh_token").GetString()!;
        return result.GetProperty("user").Clone();
    }

    public async Task<JsonElement> Send(
        string path,
        HttpMethod? method = null,
        object? body = null,
        string? idempotency = null,
        CancellationToken ct = default,
        bool refresh = true)
    {
        var usedToken = AccessToken;
        using var req = new HttpRequestMessage(method ?? HttpMethod.Get, "api/client/v1/" + path);
        if (usedToken != "")
        {
            req.Headers.Authorization = new AuthenticationHeaderValue("Bearer", usedToken);
        }
        if (idempotency != null)
        {
            req.Headers.Add("Idempotency-Key", idempotency);
        }
        if (body != null)
        {
            var json = JsonSerializer.Serialize(body, Wire.Json);
            req.Content = new StringContent(json, Encoding.UTF8, "application/json");
        }

        using var res = await http.SendAsync(req, ct);
        if (res.Headers.Date is { } serverDate)
        {
            Interlocked.Exchange(ref serverOffsetTicks, (serverDate.UtcDateTime - DateTime.UtcNow).Ticks);
        }
        if ((int)res.StatusCode >= 500)
        {
            throw new HttpRequestException("Control service temporarily unavailable", null, res.StatusCode);
        }
        if (res.StatusCode == HttpStatusCode.Unauthorized && refresh && refreshToken != "")
        {
            await refreshGate.WaitAsync(ct);
            try
            {
                if (usedToken == AccessToken)
                {
                    var renewed = await Send(
                        "auth/refresh",
                        HttpMethod.Post,
                        new { refresh_token = refreshToken },
                        ct: ct,
                        refresh: false);
                    AccessToken = renewed.GetProperty("access_token").GetString()!;
                    refreshToken = renewed.GetProperty("refresh_token").GetString()!;
                }
            }
            finally
            {
                refreshGate.Release();
            }
            return await Send(path, method, body, idempotency, ct, false);
        }

        using var doc = JsonDocument.Parse(await res.Content.ReadAsStringAsync(ct));
        if (!res.IsSuccessStatusCode)
        {
            var message = doc.RootElement.TryGetProperty("error", out var error)
                ? error.GetString()
                : "Control request failed";
            throw new InvalidOperationException(message);
        }
        return doc.RootElement.Clone();
    }

    public void Dispose()
    {
        http.Dispose();
        refreshGate.Dispose();
    }
}
