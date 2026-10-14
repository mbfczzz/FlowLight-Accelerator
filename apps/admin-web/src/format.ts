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
  player: "玩家",
  admin: "管理员",
  operator: "配置运维",
  publisher: "发布运维",
  viewer: "只读运维",
};

/** Columns whose values render as a badge rather than plain text. */
export const badgeColumns = ["state", "status", "enabled", "mode"];

/** Display form of one cell value. */
export function text(v: any, key = "") {
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
  return stateNames[v] || String(v);
}
