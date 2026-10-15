import type { Row } from "./api";
import {
  Cable,
  Gamepad2,
  History,
  LayoutDashboard,
  ChartColumn,
  ListChecks,
  MonitorSmartphone,
  Package,
  Route,
  Server,
  ShieldCheck,
  Users,
  Waypoints,
  Zap,
} from "lucide-react";

/** Sidebar entries: id, display name, icon, group. */
export const sections = [
  ["overview", "运行总览", LayoutDashboard, ""],
  ["games", "游戏与规则", Gamepad2, "配置"],
  ["lines", "加速线路", Route, "配置"],
  ["nodes", "节点服务器", Server, "配置"],
  ["endpoints", "服务端点", Cable, "配置"],
  ["bindings", "落地绑定", Waypoints, "配置"],
  ["users", "用户与权限", Users, "账号"],
  ["devices", "连接设备", MonitorSmartphone, "账号"],
  ["sessions", "加速会话", Zap, "运行"],
  ["traffic", "流量统计", ChartColumn, "运行"],
  ["jobs", "部署任务", ListChecks, "运行"],
  ["releases", "客户端版本", Package, "运行"],
  ["audits", "操作审计", History, "运行"],
] as const;

export const sectionIcon = (section: string) =>
  sections.find((s) => s[0] === section)?.[2] || ShieldCheck;

/** Column order per list section; also drives the editor's field sections. */
export const columns: Record<string, string[]> = {
  games: ["name", "regions", "published_version", "enabled"],
  lines: [
    "name",
    "regions",
    "mode",
    "capacity",
    "published_version",
    "draining",
  ],
  nodes: ["name", "region", "address", "last_seen", "enabled"],
  endpoints: ["name", "node_id", "kind", "host", "port", "port_end", "enabled"],
  bindings: ["user_id", "endpoint_id", "kind", "verified_at", "enabled"],
  users: ["username", "role", "max_sessions", "enabled"],
  devices: ["name", "user_id", "last_seen"],
  sessions: ["user_id", "game_id", "line_id", "state", "bytes", "duration", "created_at", "end_reason"],
  jobs: ["node_id", "kind", "status", "attempts", "result"],
  releases: ["version", "channel", "published", "created_at"],
  audits: ["actor", "action", "resource", "created_at"],
};

/** Per-section description under the page title. */
const descriptions: Record<string, string> = {
  overview: "平台的配置进度和运行状态，数据来自控制服务。",
  games: "游戏的区服与分流规则。保存是草稿，发布后客户端才会显示。",
  lines: "由入口、中转和落地组成的加速路径，发布并通过探测后开放。",
  nodes: "装了节点 Agent 的服务器。它本身不转发流量，入口、中转和落地服务都作为端点开在它上面。",
  endpoints: "开在节点服务器上的入口、中转或落地服务，线路按顺序把它们串起来。",
  bindings: "把落地端点分配给指定用户，共享或独享。",
  users: "玩家和运维人员的账号、角色与并发上限。",
  devices: "登录过客户端的设备。",
  sessions: "正在进行和最近结束的加速，以及发布时的验证会话。流量为落地服务器实测（上传与下载合计）。",
  traffic: "落地服务器实测的加速流量（上传与下载合计），以及用量最多的玩家、线路、游戏和服务器。",
  jobs: "控制服务下发给节点的准备、探测和回收任务。",
  releases: "客户端新版本的下载地址与校验信息。",
  audits: "后台的每一次修改：谁、做了什么、改了哪项。",
};

export function sectionDescription(section: string) {
  return descriptions[section] || "";
}

export const sectionTitle = (section: string) =>
  sections.find((s) => s[0] === section)?.[1] || "";

/** What the signed-in role may do; mirrors the control API's role checks. */
export type Permissions = {
  canEdit: boolean;
  canPublish: boolean;
};

export function permissionsFor(user: Row): Permissions {
  return {
    canEdit: ["admin", "operator", "publisher"].includes(user.role),
    canPublish: ["admin", "publisher"].includes(user.role),
  };
}

const editableSections = [
  "games",
  "lines",
  "nodes",
  "endpoints",
  "bindings",
  "users",
  "releases",
];

/** Whether rows in `section` can be created and edited by `user`. */
export function isEditable(section: string, user: Row) {
  return (
    editableSections.includes(section) &&
    permissionsFor(user).canEdit &&
    !(["users", "bindings"].includes(section) && user.role !== "admin")
  );
}

/** Default body for a new row of the given section. */
export function defaults(kind: string): Row {
  if (kind === "games")
    return {
      name: "",
      regions: [],
      draft: {
        processes: [],
        paths: [],
        domains: [],
        cidrs: [],
        exclude_domains: [],
        udp: true,
      },
      enabled: true,
    };
  if (kind === "nodes")
    return {
      name: "",
      address: "",
      region: "",
      enabled: true,
      capabilities: {
        tcp: true,
        udp: true,
        ipv4: true,
        ipv6: false,
        managed: true,
      },
    };
  if (kind === "endpoints")
    return {
      name: "",
      node_id: "",
      kind: "ss2022",
      host: "",
      port: 20000,
      port_end: 20100,
      cipher: "2022-blake3-aes-128-gcm",
      enabled: true,
      capabilities: {
        tcp: true,
        udp: true,
        ipv4: true,
        ipv6: false,
        managed: true,
      },
    };
  if (kind === "lines")
    return {
      name: "",
      regions: [],
      game_ids: [],
      user_ids: [],
      endpoint_ids: [],
      mode: "managed",
      capacity: 100,
      priority: 0,
    };
  if (kind === "users")
    return { username: "", role: "player", enabled: true, max_sessions: 1 };
  if (kind === "bindings")
    return {
      user_id: "",
      endpoint_id: "",
      kind: "shared",
      fixed_ip: false,
      enabled: true,
    };
  return {
    version: "",
    channel: "stable",
    url: "",
    sha256: "",
    signature: "",
    required: false,
    published: false,
  };
}
