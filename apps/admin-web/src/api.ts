const key = "flowlight-admin-session";
export type Row = Record<string, any>;

/** Rows per page for the list sections; matches the backend default. */
export const PAGE_SIZE = 200;
/** Largest page the backend accepts. */
export const MAX_PAGE_SIZE = 1000;

let auth: Row | null = (() => {
  try {
    return JSON.parse(sessionStorage.getItem(key) || "null");
  } catch {
    return null;
  }
})();

export const currentUser = () => auth?.user;

export function clearAuth() {
  auth = null;
  sessionStorage.removeItem(key);
}

/** A failed request. `status` is 0 when no response arrived. */
export class ApiError extends Error {
  status: number;
  constructor(message: string, status = 0) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

/**
 * The stored session can no longer be renewed. By the time this is thrown the
 * credentials are cleared and the expiry listeners have run, so callers only
 * need to stop; they should not show it as an error.
 */
export class SessionExpiredError extends ApiError {
  constructor() {
    super("登录已过期，请重新登录", 401);
    this.name = "SessionExpiredError";
  }
}

export const isSessionExpired = (e: unknown) => e instanceof SessionExpiredError;

const expiredListeners = new Set<() => void>();

/**
 * Run `listener` whenever the session expires. It fires from inside the
 * transport, so the shell returns to the login screen no matter which request
 * discovered the expiry.
 */
export function onSessionExpired(listener: () => void) {
  expiredListeners.add(listener);
  return () => {
    expiredListeners.delete(listener);
  };
}

function expire(): never {
  clearAuth();
  for (const listener of expiredListeners) listener();
  throw new SessionExpiredError();
}

async function send(url: string, init: RequestInit) {
  try {
    return await fetch(url, init);
  } catch {
    throw new ApiError("无法连接控制服务，请检查网络后重试");
  }
}

/**
 * Describe a body that is not the JSON the control service sends. A gateway
 * error page or an empty body would otherwise surface as a JSON syntax error
 * that says nothing about what happened.
 */
function unreadable(status: number, empty: boolean) {
  const what = empty ? "服务返回了空响应" : "服务返回了无法识别的响应";
  const hint = status >= 500 ? "，控制服务或网关暂时不可用，请稍后重试" : "";
  return what + "（HTTP " + status + "）" + hint;
}

async function parseBody(res: Response, fallback = "请求失败"): Promise<any> {
  if (res.status === 204) return null;
  let raw = "";
  try {
    raw = await res.text();
  } catch {
    throw new ApiError("读取响应失败（HTTP " + res.status + "）", res.status);
  }
  if (raw.trim() === "") throw new ApiError(unreadable(res.status, true), res.status);
  let body: any;
  try {
    body = JSON.parse(raw);
  } catch {
    throw new ApiError(unreadable(res.status, false), res.status);
  }
  if (!res.ok)
    throw new ApiError(
      (typeof body?.error === "string" && body.error) ||
        fallback + "（HTTP " + res.status + "）",
      res.status,
    );
  return body;
}

export async function login(username: string, password: string) {
  const res = await send("/api/admin/v1/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password }),
  });
  const body = await parseBody(res, "登录失败");
  if (!body?.user) throw new ApiError("登录失败", res.status);
  if (body.user.role === "player") throw new Error("此账号没有管理后台权限");
  auth = body;
  sessionStorage.setItem(key, JSON.stringify(body));
  return body.user;
}

let refreshing: Promise<void> | null = null;

/**
 * Renew the access token. Only a 401 means the refresh token was rejected;
 * any other failure (an unreachable service, a gateway error page) keeps the
 * stored session so the next attempt can still succeed.
 */
function refresh() {
  if (!refreshing)
    refreshing = (async () => {
      if (!auth?.refresh_token) expire();
      const res = await send("/api/admin/v1/auth/refresh", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refresh_token: auth.refresh_token }),
      });
      if (res.status === 401) expire();
      const next = await parseBody(res);
      auth = next;
      sessionStorage.setItem(key, JSON.stringify(next));
    })().finally(() => {
      refreshing = null;
    });
  return refreshing;
}

/** Call the admin API. A Blob (an uploaded file) is sent as it is. */
export async function api(
  path: string,
  method = "GET",
  data?: unknown,
  retry = true,
): Promise<any> {
  const file = data instanceof Blob ? data : null;
  const res = await send("/api/admin/v1" + path, {
    method,
    headers: {
      "Content-Type": file ? file.type || "application/octet-stream" : "application/json",
      Authorization: "Bearer " + auth?.access_token,
    },
    body: file ?? (data === undefined ? undefined : JSON.stringify(data)),
  });
  if (res.status === 401) {
    if (!retry) expire();
    await refresh();
    return api(path, method, data, false);
  }
  return parseBody(res);
}

/**
 * One page of a list endpoint. The backend answers with a plain array in its
 * usual order; parameters it does not understand are ignored, which the
 * caller detects by the page being longer than requested.
 */
export async function listPage(
  kind: string,
  limit: number,
  offset: number,
): Promise<Row[]> {
  const query = "?limit=" + limit + "&offset=" + offset;
  const body = await api("/" + kind + query);
  if (!Array.isArray(body)) throw new ApiError("服务返回的列表格式不正确");
  return body;
}
