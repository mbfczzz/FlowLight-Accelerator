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
