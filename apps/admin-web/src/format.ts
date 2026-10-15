export const labels: Record<string, string> = {
  id: "标识",
  name: "名称",
  username: "账号",
  role: "角色",
  enabled: "启用",
  regions: "区服",
  published_version: "发布版本",
  mode: "模式",
  capacity: "容量",
  priority: "优先级",
  address: "地址",
  region: "地区",
  last_seen: "最近心跳",
  kind: "类型",
  host: "服务地址",
  port: "起始端口",
  port_end: "结束端口",
  node_id: "所属节点",
  user_id: "所属用户",
  endpoint_id: "落地端点",
  device_id: "设备",
  game_id: "游戏",
  line_id: "线路",
  state: "状态",
  purpose: "用途",
  created_at: "创建时间",
  updated_at: "更新时间",
  lease_until: "租约到期",
  end_reason: "结束原因",
  attempts: "尝试次数",
  status: "执行状态",
  result: "结果",
  action: "操作",
  actor: "操作人",
  resource: "资源",
  detail: "详情",
  version: "版本",
  channel: "渠道",
  url: "下载地址",
  sha256: "SHA256",
  signature: "清单签名",
  required: "必须更新",
  published: "已发布",
  max_sessions: "并发上限",
  agent_version: "Agent 版本",
  fixed_ip: "固定出口",
  expected_exit: "预期出口 IP",
  draining: "排空中",
  verified_at: "验证时间",
  error: "失败原因",
};

/** Enum values that appear in rows and need Chinese display names. */
export const stateNames: Record<string, string> = {
  PREPARING: "准备中",
  CONNECTING: "等待客户端连接",
  ACTIVE: "运行中",
  DEGRADED: "连接异常",
  RECONNECTING: "重新连接",
  STOPPING: "正在回收",
  CLOSED: "已关闭",
  pending: "待执行",
  running: "执行中",
  succeeded: "成功",
  failed: "失败",
  cancelled: "已取消",
  ready: "已验证",
  validating: "验证中",
  managed: "平台托管",
  imported: "导入线路",
  hybrid: "平台入口＋自带落地",
  shared: "共享",
  dedicated: "独享",
  byo: "自带落地",
  ss2022: "SS2022 落地",
  realm: "Realm 中转",
  prepare: "准备转发",
  probe: "线路探测",
  revoke: "回收资源",
  game: "玩家加速",
  validation: "发布验证",
  stable: "稳定版",
  beta: "测试版",
  player: "玩家",
  admin: "管理员",
  operator: "配置运维",
  publisher: "发布运维",
  viewer: "只读运维",
};

/** Why a session ended, as the control service records it. */
const endReasons: Record<string, string> = {
  requested: "按请求停止",
  "lease or preparation timeout": "准备或租约超时",
  "line terminated by operator": "线路被终止",
  "validation complete": "验证完成",
  runtime_failed: "转发进程异常退出",
  lease_expired: "节点租约过期",
};

/** Audit log actions. */
const auditActions: Record<string, string> = {
  "resource.save": "保存配置",
  "game.publish": "发布游戏规则",
  "game.profile.activate": "切换游戏规则版本",
  "line.publish.begin": "开始发布线路",
  "line.publish.ready": "线路验证通过",
  "line.rollback": "切换线路版本",
  "line.drain": "排空或恢复线路",
  "line.terminate": "终止线路",
  "node.enroll": "生成注册凭据",
  "node.register": "节点注册",
  "node.certificate.renew": "节点证书续期",
  "egress.register": "注册自备落地",
  "session.create": "创建会话",
  "session.stop": "停止会话",
  "session.closed": "会话关闭",
};

function endReason(v: string) {
  if (endReasons[v]) return endReasons[v];
  const failed = /^node task failed: (\w+)$/.exec(v);
  if (failed) return "节点任务失败：" + (stateNames[failed[1]] || failed[1]);
  return v;
}

/** Columns whose values are enumerations with Chinese display names. */
const enumColumns = ["state", "status", "mode", "role", "kind", "purpose", "channel"];

/** Columns whose values render as a badge rather than plain text. */
export const badgeColumns = ["state", "status", "enabled", "mode"];

/** Columns that hold the id of another row. */
const referenceColumns = [
  "actor",
  "user_id",
  "node_id",
  "endpoint_id",
  "line_id",
  "game_id",
  "device_id",
  "resource",
];

/** Display form of one cell value; `names` maps ids to display names. */
export function text(v: any, key = "", names: Record<string, string> = {}) {
  if (referenceColumns.includes(key) && typeof v === "string" && names[v])
    return names[v];
  if (key === "published_version")
    return !v ? "草稿" : typeof v === "number" ? "v" + v : "已发布";
  if (
    v === null ||
    v === undefined ||
    v === "" ||
    String(v).startsWith("0001-")
  )
    return "—";
  if (typeof v === "boolean") return v ? "是" : "否";
  if (Array.isArray(v)) return v.length ? v.join(" / ") : "待配置";
  if (typeof v === "object") return JSON.stringify(v);
  if (key.endsWith("_at") || key === "last_seen" || key === "lease_until")
    return new Date(v).toLocaleString("zh-CN", { hour12: false });
  if (key === "end_reason") return endReason(String(v));
  if (key === "action") return auditActions[v] || String(v);
  // Only enumerated columns are translated: a user named "admin" or a game
  // whose id is "probe" keeps its own name.
  if (!enumColumns.includes(key)) return String(v);
  return stateNames[v] || String(v);
}
