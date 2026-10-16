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

    // What the player reads when the control service cannot be reached.
    public const string Unreachable = "无法连接服务器，请检查网络后重试";

    public ApiClient(string baseUrl, bool allowLocal = false)
    {
        var uri = new Uri(baseUrl);
        if (uri.Scheme != "https" && !(allowLocal && uri.IsLoopback && uri.Scheme == "http"))
        {
            throw new ArgumentException("控制服务地址必须使用 HTTPS");
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

        HttpResponseMessage sent;
        try
        {
            sent = await http.SendAsync(req, ct);
        }
        catch (HttpRequestException e) when (e.StatusCode is null)
        {
            // .NET names the host in these messages, and they reach the
            // player; the address is not theirs to see.
            throw new HttpRequestException(Unreachable, e);
        }
        using var res = sent;
        if (res.Headers.Date is { } serverDate)
        {
            Interlocked.Exchange(ref serverOffsetTicks, (serverDate.UtcDateTime - DateTime.UtcNow).Ticks);
        }
        if ((int)res.StatusCode >= 500)
        {
            throw new HttpRequestException("控制服务暂时不可用，请稍后重试", null, res.StatusCode);
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
                : null;
            // A refusal the control service marked retryable clears by
            // itself within a minute or two — a session that is still
            // closing, or the port it still holds. Waiting is better than
            // telling the player no.
            var code = res.StatusCode == HttpStatusCode.Conflict &&
                doc.RootElement.TryGetProperty("code", out var tag) && tag.ValueKind == JsonValueKind.String
                ? tag.GetString()
                : null;
            if (code is { Length: > 0 } retryCode)
            {
                throw new RetryableRefusalException(Localize(message), retryCode);
            }
            throw new InvalidOperationException(Localize(message));
        }
        return doc.RootElement.Clone();
    }

    // The control service answers in short English phrases; players see
    // these, so the ones a client can trigger are translated here. Anything
    // unknown is shown as received rather than hidden.
    static readonly Dictionary<string, string> Messages = new(StringComparer.Ordinal)
    {
        ["invalid credentials"] = "账号或密码错误",
        ["too many login attempts"] = "登录尝试过于频繁，请稍后再试",
        ["too many failed attempts for this account; try again later"] = "该账号登录失败次数过多，请稍后再试",
        ["refresh token expired or reused"] = "登录已过期，请重新登录",
        ["authentication required"] = "登录已过期，请重新登录",
        ["account unavailable"] = "账号已被停用",
        ["insufficient role"] = "当前账号没有权限执行此操作",
        ["resource not found"] = "请求的内容不存在",
        ["internal request failure"] = "控制服务内部错误，请稍后重试",
        ["multiple JSON objects are not accepted"] = "请求格式错误",
        ["device identifier and name required"] = "设备信息不完整",
        ["device belongs to a different account"] = "此设备已绑定到其他账号",
        ["register this device first"] = "设备尚未注册，请重新登录",
        ["device already has a session"] = "此设备已有一个加速会话在运行",
        ["account concurrency limit reached"] = "账号同时加速的设备数已达上限",
        ["game profile or region not published"] = "该游戏或区服尚未开放",
        ["line unavailable or unauthorized"] = "该线路不可用或未授权给当前账号",
        ["line is not verified and published"] = "该线路尚未通过验证发布",
        ["line does not cover game/region"] = "该线路不支持所选游戏或区服",
        ["device cannot use IPv6-only ingress"] = "此设备无法使用仅 IPv6 的线路",
        ["line capacity exhausted"] = "该线路已满员，请选择其他线路",
        ["line access was revoked"] = "该线路的使用权限已被收回",
        ["egress access was revoked"] = "出口的使用权限已被收回",
        ["egress belongs to another account"] = "该出口属于其他账号",
        ["egress binding not verified"] = "出口绑定尚未验证",
        ["egress not available to this account"] = "该出口不对当前账号开放",
        ["selected line does not terminate at this egress"] = "所选线路与出口不匹配",
        ["operator has not enabled external egress registration"] = "运营方未开放自备出口注册",
        ["external egress must be a public address or hostname"] = "自备出口必须是公网地址或域名",
        ["external egress limit reached; remove one first"] = "自备出口数量已达上限，请先删除一个",
        ["node offline"] = "线路节点离线，请稍后重试或更换线路",
        ["node unavailable"] = "线路节点不可用，请稍后重试或更换线路",
        ["node port pool exhausted"] = "线路节点端口已耗尽，请更换线路",
        ["managed node must be online and support leased runtimes"] = "线路节点离线，请稍后重试或更换线路",
        ["session is closing"] = "加速会话正在关闭",
        ["session lease expired"] = "加速会话已过期",
        ["invalid session transition"] = "加速会话状态异常",
        ["idempotency key reused with different input"] = "请求重复提交，请重试",
        ["Idempotency-Key of 8-128 characters required"] = "请求缺少幂等标识",
    };

    public static string Localize(string? message)
    {
        if (string.IsNullOrWhiteSpace(message))
        {
            return "控制服务请求失败";
        }
        if (Messages.TryGetValue(message, out var local))
        {
            return local;
        }
        if (message.StartsWith("invalid JSON request:", StringComparison.Ordinal))
        {
            return "请求格式错误，请更新客户端";
        }
        return message;
    }

    public void Dispose()
    {
        http.Dispose();
        refreshGate.Dispose();
    }
}
