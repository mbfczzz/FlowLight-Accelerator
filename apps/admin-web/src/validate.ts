import type { Row } from "./api";

/*
 * The same rules the control service applies when a row is saved, checked
 * before the request so the form can say what to fix. The service stays the
 * authority: anything missed here still comes back as its error.
 */

export const PLAYER_PASSWORD_MIN = 8;
export const STAFF_PASSWORD_MIN = 14;
const PASSWORD_MAX_BYTES = 72;
const NAME_MAX = 100;

const chars = (s: string) => Array.from(s).length;
const bytes = (s: string) => new TextEncoder().encode(s).length;
const str = (v: unknown) => (typeof v === "string" ? v : "");
const isInt = (v: unknown) => typeof v === "number" && Number.isInteger(v);

function name(v: unknown, field: string) {
  const s = str(v);
  return !s.trim() || chars(s) > NAME_MAX
    ? `请填写${field}（不超过 ${NAME_MAX} 个字符）`
    : "";
}

function host(v: unknown, field: string) {
  const s = str(v);
  return !s || s.length >= 254 || /[\s/\\,"']/.test(s)
    ? `${field}不能为空，且不能包含空格、斜杠、引号或逗号`
    : "";
}

function range(v: unknown, lo: number, hi: number, message: string) {
  return isInt(v) && (v as number) >= lo && (v as number) <= hi ? "" : message;
}

/** Label of the password field for a user being created or edited. */
export function passwordLabel(role: unknown, editing: boolean) {
  if (editing) return "新密码（留空保留原密码）";
  return role === "player"
    ? `密码（至少 ${PLAYER_PASSWORD_MIN} 位）`
    : `密码（管理和运维账号至少 ${STAFF_PASSWORD_MIN} 位）`;
}

function password(body: Row, initial: Row) {
  const pw = str(body.password);
  const staff = body.role !== "player";
  if (!initial.id && !pw) return "新用户需要设置密码";
  if (pw) {
    if (chars(pw) < PLAYER_PASSWORD_MIN)
      return `密码至少 ${PLAYER_PASSWORD_MIN} 位`;
    if (staff && chars(pw) < STAFF_PASSWORD_MIN)
      return `管理和运维账号的密码至少 ${STAFF_PASSWORD_MIN} 位`;
    if (bytes(pw) > PASSWORD_MAX_BYTES)
      return "密码过长：最多 72 字节（约 72 个英文字符或 24 个汉字）";
  } else if (staff && initial.id && initial.role === "player") {
    return `把玩家改为管理或运维角色时，请同时设置至少 ${STAFF_PASSWORD_MIN} 位的新密码`;
  }
  return "";
}

/** A problem and the field it belongs to. */
export type Problem = { field: string; message: string };

/** First problem with `body`, or null when it can be sent. */
export function validate(kind: string, body: Row, initial: Row): Problem | null {
  const checks: [string, string][] = [];
  if (kind === "games") {
    const draft = body.draft || {};
    checks.push(
      ["name", name(body.name, "游戏名称")],
      ["draft.processes", (draft.processes?.length || 0) + (draft.paths?.length || 0)
        ? ""
        : "请至少填写一个游戏进程名或完整程序路径"],
    );
  }
  if (kind === "nodes")
    checks.push(["name", name(body.name, "节点名称")], ["address", host(body.address, "节点地址")]);
  if (kind === "endpoints")
    checks.push(
      ["name", name(body.name, "端点名称")],
      ["node_id", body.node_id ? "" : "请选择所属节点"],
      ["host", host(body.host, "服务地址")],
      ["port", range(body.port, 1, 65535, "起始端口须在 1–65535 之间")],
      ["port", body.capabilities?.managed && body.port < 1024
        ? "平台管理的端点，起始端口不能低于 1024"
        : ""],
      ["port_end", range(
        body.port_end,
        Number(body.port) || 1,
        65535,
        "结束端口须不小于起始端口，且不超过 65535",
      )],
    );
  if (kind === "lines") {
    const hops: string[] = body.endpoint_ids || [];
    checks.push(
      ["name", name(body.name, "线路名称")],
      ["capacity", range(body.capacity, 1, 10000, "并发容量须在 1–10000 之间")],
      ["priority", range(body.priority, -1000000, 1000000, "优先级须为整数")],
      ["endpoint_ids", hops.some((h) => !h) ? "线路路径里有未选择的服务端点" : ""],
      ["endpoint_ids", hops.length > 5 ? "一条线路最多 5 个服务端点" : ""],
    );
  }
  if (kind === "users") {
    const username = str(body.username).trim();
    checks.push(
      ["username", name(username, "账号")],
      ["username", /\s/.test(username) ? "账号不能包含空格" : ""],
      ["role", body.role ? "" : "请选择权限"],
      ["max_sessions", range(body.max_sessions, 1, 20, "并发会话上限须在 1–20 之间")],
      ["password", password(body, initial)],
    );
  }
  if (kind === "bindings")
    checks.push(
      ["user_id", body.user_id ? "" : "请选择用户"],
      ["endpoint_id", body.endpoint_id ? "" : "请选择落地端点"],
      ["kind", body.kind ? "" : "请选择来源（共享、独享或自带落地）"],
    );
  if (kind === "releases") {
    let https = false;
    try {
      const u = new URL(str(body.url));
      https = u.protocol === "https:" && !!u.host;
    } catch {
      https = false;
    }
    checks.push(
      ["version", name(body.version, "版本号")],
      ["url", https ? "" : "下载地址必须是 https:// 开头的完整地址"],
      ["sha256", /^[0-9a-fA-F]{64}$/.test(str(body.sha256))
        ? ""
        : "安装包 SHA256 应为 64 位十六进制字符"],
      ["signature", str(body.signature).trim() ? "" : "请填写可信发布清单签名"],
    );
  }
  const first = checks.find(([, message]) => message);
  return first ? { field: first[0], message: first[1] } : null;
}
