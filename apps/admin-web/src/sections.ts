import type { Row } from "./api";
import {
  Activity,
  Box,
  Cable,
  Clock,
  Gamepad2,
  Layers,
  Network,
  Server,
  Settings2,
  Shield,
  Users,
} from "lucide-react";

/** Sidebar entries: id, display name, icon. */
export const sections = [
  ["overview", "运行总览", Activity],
  ["games", "游戏与规则", Gamepad2],
  ["lines", "加速线路", Network],
  ["nodes", "转发节点", Server],
  ["endpoints", "服务端点", Cable],
  ["bindings", "落地绑定", Layers],
  ["users", "用户与权限", Users],
  ["devices", "连接设备", Box],
  ["sessions", "加速会话", Activity],
  ["jobs", "部署任务", Clock],
  ["releases", "客户端版本", Settings2],
  ["audits", "操作审计", Shield],
] as const;

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
  sessions: ["game_id", "line_id", "state", "lease_until", "end_reason"],
  jobs: ["node_id", "kind", "status", "attempts", "result"],
  releases: ["version", "channel", "published", "created_at"],
  audits: ["actor", "action", "resource", "created_at"],
};

/** Per-section description under the page title. */
export function sectionDescription(section: string) {
  if (section === "overview")
    return "配置、连接与节点运行状态。所有数据来自控制服务。";
  if (section === "games") return "规则先保存为草稿，发布后供新会话使用。";
  if (section === "lines")
    return "独立配置入口、中转和落地，发布前完成整条路径探测。";
  return "查看并维护平台资源，操作会记录到审计日志。";
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
