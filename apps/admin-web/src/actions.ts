import type { Row } from "./api";

/**
 * A state-changing request that must be confirmed before it is sent.
 * `lock` names the row it acts on: while one request for a row is in flight,
 * every confirmable action on that row is disabled.
 */
export type Confirmable = {
  lock: string;
  /** Which action on the row this is, for its in-progress label. */
  action: string;
  title: string;
  message: string;
  confirmLabel: string;
  /** Interrupts traffic or players; rendered with the danger style. */
  danger?: boolean;
  path: string;
  /** POST unless given. */
  method?: "POST" | "DELETE";
  body?: unknown;
  /** Notice shown once the request succeeds. */
  success: string;
  /** The row's display name, for what the action opens afterwards. */
  subject?: string;
};

const DONE = "操作已完成";

export const rowLock = (kind: string, id: string) => kind + ":" + id;

const nameOf = (row: Row) => row.name || row.username || row.id;

export function publishGame(row: Row): Confirmable {
  return {
    lock: rowLock("games", row.id),
    action: "publish",
    title: "发布游戏规则",
    message:
      "将把「" + nameOf(row) + "」的当前草稿发布为新版本，发布后供新会话使用。",
    confirmLabel: "发布规则",
    path: "/games/" + row.id + "/profiles/publish",
    success: DONE,
  };
}

export function publishLine(row: Row): Confirmable {
  return {
    lock: rowLock("lines", row.id),
    action: "publish",
    title: "发布线路",
    message:
      "将为「" +
      nameOf(row) +
      "」创建隔离的验证会话并探测整条路径。探测通过后新版本才会生效，失败时保留之前的已发布版本。",
    confirmLabel: "开始发布",
    path: "/lines/" + row.id + "/publish",
    success: "线路验证已开始，结果以实际探测为准",
  };
}

export function drainLine(row: Row): Confirmable {
  const draining = !row.draining;
  return {
    lock: rowLock("lines", row.id),
    action: "drain",
    title: draining ? "排空线路" : "恢复接收新会话",
    message: draining
      ? "「" + nameOf(row) + "」将停止接收新会话，已有会话继续运行直至结束。"
      : "「" + nameOf(row) + "」将重新接收新会话。",
    confirmLabel: draining ? "排空" : "恢复接收",
    danger: draining,
    path: "/lines/" + row.id + "/drain",
    body: { draining },
    success: DONE,
  };
}

export function terminateLine(row: Row): Confirmable {
  return {
    lock: rowLock("lines", row.id),
    action: "terminate",
    title: "终止线路",
    message:
      "「" +
      nameOf(row) +
      "」将停止接收新会话，并立即回收该线路上所有未关闭的会话，正在加速的玩家会断开连接。",
    confirmLabel: "终止线路",
    danger: true,
    path: "/lines/" + row.id + "/terminate",
    success: DONE,
  };
}

export function rollbackLine(
  lineId: string,
  lineName: string,
  version: Row,
): Confirmable {
  return {
    lock: rowLock("lines", lineId),
    action: "rollback",
    title: "切换线路版本",
    message:
      "「" +
      lineName +
      "」将切换到已验证的版本 " +
      version.number +
      "，之后创建的会话会使用该版本。",
    confirmLabel: "切换到版本 " + version.number,
    path: "/lines/" + lineId + "/rollback",
    body: { version_id: version.id },
    success: DONE,
  };
}

export function stopSession(row: Row): Confirmable {
  return {
    lock: rowLock("sessions", row.id),
    action: "stop",
    title: "停止会话",
    message: "将停止会话 " + row.id + " 并回收其资源，玩家的加速连接会断开。",
    confirmLabel: "停止会话",
    danger: true,
    path: "/sessions/" + row.id + "/stop",
    success: DONE,
  };
}

/** What deleting a row takes with it, and what has to go first. */
const deletion: Record<string, [string, (name: string) => string]> = {
  nodes: [
    "节点服务器",
    (n) =>
      "将删除节点服务器「" + n + "」。上面还有服务端点时不能删除，需要先删除那些端点。" +
      "删除后这台服务器上的节点程序无法再连接控制服务；不再使用的话，在服务器上运行 sudo systemctl disable --now flowlight-node 停掉它。",
  ],
  endpoints: [
    "服务端点",
    (n) =>
      "将删除服务端点「" + n + "」和它保存的密钥。仍被线路（含已发布的版本）、落地绑定或进行中的会话使用时不能删除。",
  ],
  lines: [
    "线路",
    (n) =>
      "将删除线路「" + n + "」，玩家不再看到它；历史会话和流量记录会保留。还有进行中的会话时不能删除，可以先在“更多”里终止线路。",
  ],
  games: [
    "游戏",
    (n) =>
      "将删除游戏「" + n + "」和它的全部规则版本、封面，玩家不再看到它；历史会话和流量记录会保留。仍被线路限定或有玩家正在加速时不能删除。",
  ],
  bindings: [
    "落地绑定",
    () => "将删除这个落地绑定。还有进行中的会话使用它时不能删除。",
  ],
  releases: [
    "客户端版本",
    (n) =>
      "将删除客户端版本 " + n + "。已安装的客户端不受影响；它是最新版本的话，客户端会改为提示上一个已发布的版本。",
  ],
};

export const deletable = (section: string) => section in deletion;

export function removeRow(section: string, row: Row): Confirmable {
  const [kind, message] = deletion[section];
  return {
    lock: rowLock(section, row.id),
    action: "delete",
    title: "删除" + kind,
    message: message(section === "releases" ? row.version : nameOf(row)) + "删除后不能恢复。",
    confirmLabel: "删除",
    danger: true,
    method: "DELETE",
    path: "/" + section + "/" + row.id,
    success: "已删除",
  };
}

export function regenerateEnrollment(row: Row): Confirmable {
  return {
    lock: rowLock("nodes", row.id),
    action: "enroll",
    subject: nameOf(row),
    title: row.agent_version ? "重新生成接入命令" : "生成接入命令",
    message:
      "将为「" +
      nameOf(row) +
      "」生成一条带一次性凭据的接入命令，在服务器上运行它即可安装节点程序并完成注册。此前生成且尚未使用的凭据会立即失效。" +
      (row.agent_version ? "这台服务器已经注册过；只是更新节点程序时，用不带凭据的同一命令即可，不需要重新生成。" : ""),
    confirmLabel: "生成命令",
    path: "/nodes/" + row.id + "/enrollment",
    success: "已生成接入命令",
  };
}
