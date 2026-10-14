const key = "flowlight-admin-session";
export type Row = Record<string, any>;
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
export async function login(username: string, password: string) {
  const res = await fetch("/api/admin/v1/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password }),
  });
  const body = await res.json();
  if (!res.ok) throw new Error(body.error || "登录失败");
  if (body.user.role === "player") throw new Error("此账号没有管理后台权限");
  auth = body;
  sessionStorage.setItem(key, JSON.stringify(body));
  return body.user;
}
let refreshing: Promise<void> | null = null;
async function refresh() {
  if (!refreshing)
    refreshing = (async () => {
      const res = await fetch("/api/admin/v1/auth/refresh", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refresh_token: auth?.refresh_token }),
      });
      if (!res.ok) {
        clearAuth();
        throw new Error("登录已过期，请重新登录");
      }
      auth = await res.json();
      sessionStorage.setItem(key, JSON.stringify(auth));
    })().finally(() => {
      refreshing = null;
    });
  return refreshing;
}
export async function api(
  path: string,
  method = "GET",
  data?: unknown,
  retry = true,
): Promise<any> {
  const res = await fetch("/api/admin/v1" + path, {
    method,
    headers: {
      "Content-Type": "application/json",
      Authorization: "Bearer " + auth?.access_token,
    },
    body: data === undefined ? undefined : JSON.stringify(data),
  });
  if (res.status === 401 && retry) {
    await refresh();
    return api(path, method, data, false);
  }
  const body = await res.json();
  if (!res.ok) throw new Error(body.error || "请求失败");
  return body;
}
