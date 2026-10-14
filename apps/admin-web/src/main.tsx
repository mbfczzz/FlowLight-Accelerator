import React, { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import {
  Activity,
  ArrowDown,
  ArrowUp,
  Box,
  Cable,
  ChevronRight,
  Clock,
  Gamepad2,
  Layers,
  LogOut,
  Network,
  Plus,
  RefreshCw,
  Search,
  Server,
  Settings2,
  Shield,
  Users,
  X,
} from "lucide-react";
import { api, login, currentUser, clearAuth, type Row } from "./api";
import "./style.css";
const sections = [
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
const labels: Record<string, string> = {
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
const columns: Record<string, string[]> = {
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
const stateNames: Record<string, string> = {
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
function text(v: any, key = "") {
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
function App() {
  const [user, setUser] = useState<Row | null>(currentUser() || null);
  const [section, setSection] = useState("overview");
  const [rows, setRows] = useState<Row[]>([]);
  const [overview, setOverview] = useState<Row>({});
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [search, setSearch] = useState("");
  const [editing, setEditing] = useState<Row | null>(null);
  const [detail, setDetail] = useState<Row | null>(null);
  const [enroll, setEnroll] = useState<Row | null>(null);
  const [notice, setNotice] = useState("");
  const canEdit =
    !!user && ["admin", "operator", "publisher"].includes(user.role);
  const canPublish = !!user && ["admin", "publisher"].includes(user.role);
  async function refresh() {
    setBusy(true);
    setError("");
    if (section === "overview") setOverview({});
    try {
      if (section === "overview") setOverview(await api("/overview"));
      else setRows(await api("/" + section));
    } catch (e) {
      setError((e as Error).message);
      if (!currentUser()) setUser(null);
    } finally {
      setBusy(false);
    }
  }
  useEffect(() => {
    if (!user) return;
    let alive = true;
    setRows([]);
    if (section === "overview") setOverview({});
    setSearch("");
    setError("");
    setBusy(true);
    api("/" + section)
      .then((v) => {
        if (alive) {
          section === "overview" ? setOverview(v) : setRows(v);
        }
      })
      .catch((e) => {
        if (alive) setError(e.message);
      })
      .finally(() => {
        if (alive) setBusy(false);
      });
    return () => {
      alive = false;
    };
  }, [section, user]);
  async function action(path: string, body?: unknown) {
    setBusy(true);
    setError("");
    try {
      const result = await api(path, "POST", body);
      setNotice(
        path.startsWith("/lines/") && path.endsWith("/publish")
          ? "线路验证已开始，结果以实际探测为准"
          : "操作已完成",
      );
      await refresh();
      return result;
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  if (!user) return <Login onLogin={setUser} />;
  const title = sections.find((s) => s[0] === section)?.[1] || "";
  const editable =
    [
      "games",
      "lines",
      "nodes",
      "endpoints",
      "bindings",
      "users",
      "releases",
    ].includes(section) &&
    canEdit &&
    !(["users", "bindings"].includes(section) && user.role !== "admin");
  const filtered = rows.filter((r) =>
    JSON.stringify(r).toLowerCase().includes(search.toLowerCase()),
  );
  return (
    <div className="shell">
      <aside>
        <a
          className="brand"
          href="#"
          onClick={(e) => {
            e.preventDefault();
            setSection("overview");
          }}
        >
          <span className="brand-symbol">F</span>
          <span>
            FlowLight<strong>Accelerator</strong>
          </span>
        </a>
        <div className="workspace">
          流光加速器 <span>管理后台</span>
        </div>
        <nav aria-label="管理导航">
          {sections.map(([id, name, Icon]) => (
            <button
              key={id}
              className={section === id ? "selected" : ""}
              onClick={() => {
                setSection(id);
                setNotice("");
              }}
            >
              <Icon size={18} />
              {name}
              {section === id && <ChevronRight size={14} />}
            </button>
          ))}
        </nav>
        <div className="account">
          <span>
            {user.username}
            <small>{stateNames[user.role]}</small>
          </span>
          <button
            title="退出登录"
            aria-label="退出登录"
            onClick={async () => {
              await api("/auth/logout", "POST").catch(() => {});
              clearAuth();
              setUser(null);
            }}
          >
            <LogOut size={17} />
          </button>
        </div>
      </aside>
      <main>
        <header>
          <div className="breadcrumb">
            工作空间 <ChevronRight size={12} />
            {title}
          </div>
          <div className="header-label">FlowLight / Control</div>
        </header>
        <section className="content">
          <div className="page-head">
            <div>
              <h1>{title}</h1>
              <p>
                {section === "overview"
                  ? "配置、连接与节点运行状态。所有数据来自控制服务。"
                  : section === "games"
                    ? "规则先保存为草稿，发布后供新会话使用。"
                    : section === "lines"
                      ? "独立配置入口、中转和落地，发布前完成整条路径探测。"
                      : "查看并维护平台资源，操作会记录到审计日志。"}
              </p>
            </div>
            <div className="actions">
              <button className="secondary" onClick={refresh} disabled={busy}>
                <RefreshCw size={15} />
                刷新
              </button>
              {editable && (
                <button className="primary" onClick={() => setEditing({})}>
                  <Plus size={16} />
                  新增
                </button>
              )}
            </div>
          </div>
          {error && (
            <div className="alert" role="alert">
              {error}
              <button onClick={refresh}>重试</button>
            </div>
          )}
          {notice && (
            <div className="notice" role="status">
              {notice}
              <button aria-label="关闭提示" onClick={() => setNotice("")}>
                <X size={15} />
              </button>
            </div>
          )}
          {section === "overview" ? (
            <>
              <div className="metrics">
                {[
                  ["games", "游戏规则"],
                  ["active_sessions", "当前会话"],
                  ["online_nodes", "在线节点"],
                  ["verified_versions", "已验证线路版本"],
                ].map(([k, n]) => (
                  <div key={k}>
                    <span>{n}</span>
                    <strong>{busy ? "—" : (overview[k] ?? "—")}</strong>
                    <small>
                      {k === "online_nodes"
                        ? "依据最近心跳"
                        : k === "verified_versions"
                          ? "依据实际 TCP / UDP 探测"
                          : "控制服务记录"}
                    </small>
                  </div>
                ))}
              </div>
              <div className="onboarding">
                <div>
                  <Network size={28} />
                  <h2>从第一条可用线路开始</h2>
                  <p>
                    登记节点与服务端点，为游戏配置区服，组合线路并发布。节点完成执行与探测后，客户端才能选择该线路。
                  </p>
                  <button
                    className="primary"
                    onClick={() => setSection("nodes")}
                  >
                    管理转发节点
                    <ChevronRight size={16} />
                  </button>
                </div>
                <ol>
                  <li>
                    <b>01</b>
                    <span>
                      配置游戏规则
                      <small>VALORANT 与英雄联盟已提供初始草稿</small>
                    </span>
                  </li>
                  <li>
                    <b>02</b>
                    <span>
                      接入节点与端点
                      <small>按节点配置独立身份、协议和端口池</small>
                    </span>
                  </li>
                  <li>
                    <b>03</b>
                    <span>
                      发布并验证线路<small>实际探测通过后开放新会话</small>
                    </span>
                  </li>
                </ol>
              </div>
            </>
          ) : (
            <>
              <div className="table-toolbar">
                <label>
                  <Search size={16} />
                  <input
                    aria-label="搜索当前列表"
                    placeholder="搜索名称、状态或标识"
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                  />
                </label>
                <span>{filtered.length} 条记录</span>
              </div>
              <div className="table-wrap" aria-busy={busy}>
                <table>
                  <thead>
                    <tr>
                      {columns[section]?.map((c) => (
                        <th key={c}>{labels[c] || c}</th>
                      ))}
                      <th>操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    {filtered.map((row) => (
                      <tr key={row.id}>
                        {columns[section]?.map((col, i) => (
                          <td key={col}>
                            {["state", "status", "enabled", "mode"].includes(
                              col,
                            ) ? (
                              <span
                                className={
                                  "badge " + String(row[col]).toLowerCase()
                                }
                              >
                                {text(row[col], col)}
                              </span>
                            ) : (
                              <span className={i === 0 ? "cell-main" : ""}>
                                {text(row[col], col)}
                              </span>
                            )}
                          </td>
                        ))}
                        <td>
                          <div className="row-actions">
                            <button onClick={() => setDetail(row)}>详情</button>
                            {editable && (
                              <button onClick={() => setEditing(row)}>
                                编辑
                              </button>
                            )}
                            {section === "games" && canPublish && (
                              <button
                                onClick={() =>
                                  action(
                                    "/games/" + row.id + "/profiles/publish",
                                  )
                                }
                              >
                                发布规则
                              </button>
                            )}
                            {section === "lines" && canPublish && (
                              <>
                                <button
                                  onClick={() =>
                                    action("/lines/" + row.id + "/publish")
                                  }
                                >
                                  发布
                                </button>
                                <button
                                  onClick={() =>
                                    action("/lines/" + row.id + "/drain", {
                                      draining: !row.draining,
                                    })
                                  }
                                >
                                  {row.draining ? "接收新会话" : "排空"}
                                </button>
                                <button
                                  onClick={async () => {
                                    try {
                                      setDetail({
                                        title: "线路版本",
                                        versions: await api(
                                          "/lines/" + row.id + "/versions",
                                        ),
                                        line_id: row.id,
                                      });
                                    } catch (e) {
                                      setError((e as Error).message);
                                    }
                                  }}
                                >
                                  版本
                                </button>
                              </>
                            )}
                            {section === "nodes" && canEdit && (
                              <button
                                onClick={async () => {
                                  const result = await action(
                                    "/nodes/" + row.id + "/enrollment",
                                  );
                                  if (result) setEnroll(result);
                                }}
                              >
                                注册凭据
                              </button>
                            )}
                            {section === "sessions" &&
                              canEdit &&
                              row.state !== "CLOSED" && (
                                <button
                                  onClick={() =>
                                    action("/sessions/" + row.id + "/stop")
                                  }
                                >
                                  停止
                                </button>
                              )}
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                {!filtered.length && (
                  <div className="empty">
                    <Box size={28} />
                    <h3>{busy ? "正在读取资源…" : "暂无记录"}</h3>
                    <p>
                      {search
                        ? "试试其他关键词。"
                        : editable
                          ? "创建资源后，它会出现在这里。"
                          : "资源运行后会自动产生记录。"}
                    </p>
                  </div>
                )}
              </div>
            </>
          )}
        </section>
        <footer>
          FlowLight Accelerator <span>游戏流量由加速节点独立处理</span>
        </footer>
      </main>
      {editing && (
        <Editor
          kind={section}
          initial={editing}
          onClose={() => setEditing(null)}
          onSave={async (body) => {
            await api(
              "/" + section + (editing.id ? "/" + editing.id : ""),
              editing.id ? "PATCH" : "POST",
              body,
            );
            setEditing(null);
            setNotice("已保存。配置变更需要发布后供新会话使用。");
            await refresh();
          }}
        />
      )}
      {detail && (
        <Modal title={detail.title || "资源详情"} close={() => setDetail(null)}>
          <dl className="details">
            {Object.entries(detail)
              .filter(([k]) => k !== "title")
              .map(([k, v]) => (
                <div key={k}>
                  <dt>{labels[k] || k}</dt>
                  <dd>
                    {typeof v === "object" ? (
                      <pre>{JSON.stringify(v, null, 2)}</pre>
                    ) : (
                      text(v, k)
                    )}
                  </dd>
                </div>
              ))}
          </dl>
          {detail.versions && canPublish && (
            <div className="version-actions">
              {detail.versions
                .filter((v: Row) => v.status === "ready")
                .map((v: Row) => (
                  <button
                    key={v.id}
                    onClick={async () => {
                      await action("/lines/" + detail.line_id + "/rollback", {
                        version_id: v.id,
                      });
                      setDetail(null);
                    }}
                  >
                    切换到版本 {v.number}
                  </button>
                ))}
            </div>
          )}
        </Modal>
      )}
      {enroll && (
        <Modal title="节点一次性注册凭据" close={() => setEnroll(null)}>
          <p>交给对应节点的部署人员。注册成功后，此引导凭据自动失效。</p>
          <label>
            节点标识
            <input readOnly value={enroll.node_id} />
          </label>
          <label>
            引导凭据
            <input readOnly value={enroll.bootstrap_secret} />
          </label>
          <button
            className="primary"
            onClick={() =>
              navigator.clipboard
                .writeText(JSON.stringify(enroll, null, 2))
                .then(() => setNotice("已复制注册信息"))
            }
          >
            复制注册信息
          </button>
        </Modal>
      )}
    </div>
  );
}
function Login({ onLogin }: { onLogin: (u: Row) => void }) {
  const [username, setName] = useState("admin");
  const [password, setPass] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  return (
    <div className="login">
      <div className="login-art">
        <div className="brand">
          <span className="brand-symbol">F</span>
          <span>
            FlowLight<strong>Accelerator</strong>
          </span>
        </div>
        <h1>
          每条线路，
          <br />
          清晰可控。
        </h1>
        <p>
          连接游戏、节点与玩家，
          <br />
          管理流光加速器的每一次连接。
        </p>
        <div className="route-graphic">
          <i />
          <span />
          <i />
          <span />
          <i />
        </div>
      </div>
      <form
        onSubmit={async (e) => {
          e.preventDefault();
          setBusy(true);
          setError("");
          try {
            onLogin(await login(username, password));
          } catch (e) {
            setError((e as Error).message);
          } finally {
            setBusy(false);
          }
        }}
      >
        <h2>登录管理后台</h2>
        <p>使用管理员或运维账号继续。</p>
        <label>
          账号
          <input
            autoComplete="username"
            required
            value={username}
            onChange={(e) => setName(e.target.value)}
          />
        </label>
        <label>
          密码
          <input
            autoComplete="current-password"
            type="password"
            required
            value={password}
            onChange={(e) => setPass(e.target.value)}
          />
        </label>
        {error && (
          <p className="alert" role="alert">
            {error}
          </p>
        )}
        <button className="primary" disabled={busy}>
          {busy ? "正在登录…" : "登录"}
          <ChevronRight size={16} />
        </button>
        <small>首次账号由控制服务启动配置创建。</small>
      </form>
    </div>
  );
}
function Modal({
  title,
  children,
  close,
}: {
  title: string;
  children: React.ReactNode;
  close: () => void;
}) {
  const ref = React.useRef<HTMLDialogElement>(null);
  useEffect(() => {
    ref.current?.showModal();
  }, []);
  return (
    <dialog ref={ref} onCancel={close}>
      <div className="modal-head">
        <h2>{title}</h2>
        <button aria-label="关闭" onClick={close}>
          <X size={20} />
        </button>
      </div>
      {children}
    </dialog>
  );
}
function Editor({
  kind,
  initial,
  onClose,
  onSave,
}: {
  kind: string;
  initial: Row;
  onClose: () => void;
  onSave: (r: Row) => Promise<void>;
}) {
  const [d, set] = useState<Row>(() => ({
    ...defaults(kind),
    ...structuredClone(initial),
  }));
  const [refs, setRefs] = useState<Record<string, Row[]>>({});
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    let alive = true;
    Promise.all(
      ["nodes", "endpoints", "games", "users"].map(
        async (k) => [k, await api("/" + k).catch(() => [])] as const,
      ),
    ).then((p) => {
      if (alive) setRefs(Object.fromEntries(p));
    });
    return () => {
      alive = false;
    };
  }, []);
  function update(k: string, v: any) {
    set((prev) => {
      const next = structuredClone(prev);
      const bits = k.split(".");
      if (bits.length === 2) {
        next[bits[0]] ??= {};
        next[bits[0]][bits[1]] = v;
      } else next[k] = v;
      return next;
    });
  }
  function val(k: string): any {
    return k.split(".").reduce<any>((o, p) => o?.[p], d);
  }
  function field(
    k: string,
    label: string,
    type = "text",
    choices?: [string, string][],
  ) {
    return (
      <label key={k}>
        {label}
        {type === "check" ? (
          <input
            type="checkbox"
            checked={!!val(k)}
            onChange={(e) => update(k, e.target.checked)}
          />
        ) : type === "list" ? (
          <textarea
            value={(val(k) || []).join("\n")}
            onChange={(e) =>
              update(
                k,
                e.target.value
                  .split("\n")
                  .map((x) => x.trim())
                  .filter(Boolean),
              )
            }
            placeholder="每行一个值"
          />
        ) : choices ? (
          <select
            value={val(k) || ""}
            onChange={(e) => update(k, e.target.value)}
          >
            <option value="">请选择</option>
            {choices.map(([v, n]) => (
              <option value={v} key={v}>
                {n}
              </option>
            ))}
          </select>
        ) : (
          <input
            type={type}
            value={val(k) ?? ""}
            onChange={(e) =>
              update(
                k,
                type === "number" ? Number(e.target.value) : e.target.value,
              )
            }
            autoComplete={type === "password" ? "new-password" : "off"}
          />
        )}
      </label>
    );
  }
  const options = (key: string) =>
    refs[key]?.map(
      (r) => [r.id, r.name || r.username || r.id] as [string, string],
    ) || [];
  return (
    <Modal
      title={
        (initial.id ? "编辑" : "新增") +
        (sections.find((s) => s[0] === kind)?.[1] || "资源")
      }
      close={onClose}
    >
      <form
        onSubmit={async (e) => {
          e.preventDefault();
          setBusy(true);
          setError("");
          try {
            const body = structuredClone(d);
            delete body.id;
            delete body.created_at;
            delete body.updated_at;
            if (kind === "users") {
              delete body.password_hash;
            }
            await onSave(body);
          } catch (e) {
            setError((e as Error).message);
          } finally {
            setBusy(false);
          }
        }}
      >
        <div className="fields">
          {["games", "nodes", "endpoints", "lines"].includes(kind) &&
            field("name", "名称")}
          {kind === "games" && (
            <>
              {field("regions", "区服标识，例如 kr、jp", "list")}
              {field("draft.processes", "游戏进程名称", "list")}
              {field("draft.paths", "完整程序路径（可选）", "list")}
              {field("draft.domains", "游戏域名（可选）", "list")}
              {field("draft.cidrs", "目标网段（可选）", "list")}
              {field("draft.exclude_domains", "排除域名（可选）", "list")}
              {field("draft.udp", "需要 UDP", "check")}
              {field("enabled", "启用", "check")}
              <p className="hint">
                内置游戏规则是初始草稿，发布前请按实际安装和区服核对。
              </p>
            </>
          )}
          {kind === "nodes" && (
            <>
              {field("region", "节点地区")}
              {field("address", "节点地址")}
              {field("enabled", "启用节点", "check")}
              {["tcp", "udp", "ipv4", "ipv6", "managed"].map((k) =>
                field(
                  "capabilities." + k,
                  k === "managed" ? "支持托管进程" : k.toUpperCase(),
                  "check",
                ),
              )}
            </>
          )}
          {kind === "endpoints" && (
            <>
              {field("node_id", "所属节点", "text", options("nodes"))}
              {field("kind", "服务类型", "text", [
                ["ss2022", "SS2022 落地"],
                ["realm", "Realm 透明中转"],
              ])}
              {field("host", "客户端／上游可达地址")}
              {field("port", "起始端口", "number")}
              {field("port_end", "结束端口", "number")}
              {field("cipher", "加密方法", "text", [
                ["2022-blake3-aes-128-gcm", "SS2022 AES-128"],
                ["2022-blake3-aes-256-gcm", "SS2022 AES-256"],
              ])}
              {field(
                "password",
                "导入线路密钥（仅新增或更换时填写）",
                "password",
              )}
              {["tcp", "udp", "ipv4", "ipv6", "managed"].map((k) =>
                field(
                  "capabilities." + k,
                  k === "managed" ? "平台管理此端点" : k.toUpperCase(),
                  "check",
                ),
              )}
              {field("enabled", "启用", "check")}
            </>
          )}
          {kind === "lines" && (
            <>
              {field("regions", "可用区服", "list")}
              {field("mode", "运行模式", "text", [
                ["managed", "平台托管"],
                ["imported", "导入现成线路"],
                ["hybrid", "平台入口＋自带落地"],
              ])}
              {field("capacity", "并发容量", "number")}
              {field("priority", "优先级", "number")}
              {field("expected_exit", "预期出口 IP（可选）")}
              <div className="path-editor">
                <span>线路顺序 · 入口 → 中转 → 落地</span>
                {(d.endpoint_ids || []).map((id: string, i: number) => (
                  <div key={i}>
                    <b>{i + 1}</b>
                    <select
                      value={id}
                      onChange={(e) => {
                        const a = [...d.endpoint_ids];
                        a[i] = e.target.value;
                        update("endpoint_ids", a);
                      }}
                    >
                      <option value="">选择服务端点</option>
                      {options("endpoints").map(([v, n]) => (
                        <option key={v} value={v}>
                          {n}
                        </option>
                      ))}
                    </select>
                    <button
                      type="button"
                      aria-label="上移"
                      disabled={!i}
                      onClick={() => {
                        const a = [...d.endpoint_ids];
                        [a[i - 1], a[i]] = [a[i], a[i - 1]];
                        update("endpoint_ids", a);
                      }}
                    >
                      <ArrowUp size={14} />
                    </button>
                    <button
                      type="button"
                      aria-label="删除此跳"
                      onClick={() =>
                        update(
                          "endpoint_ids",
                          d.endpoint_ids.filter(
                            (_: string, j: number) => j !== i,
                          ),
                        )
                      }
                    >
                      <X size={14} />
                    </button>
                  </div>
                ))}
                <button
                  type="button"
                  onClick={() =>
                    update("endpoint_ids", [...(d.endpoint_ids || []), ""])
                  }
                >
                  添加端点
                </button>
              </div>
              {field("game_ids", "限定游戏（留空允许全部游戏）", "list")}
              {field("user_ids", "限定用户（留空允许已授权账号）", "list")}
              <p className="hint">
                发布会创建隔离的验证会话。探测失败时保留之前的已发布版本。
              </p>
            </>
          )}
          {kind === "bindings" && (
            <>
              {field("user_id", "用户", "text", options("users"))}
              {field("endpoint_id", "落地端点", "text", options("endpoints"))}
              {field("kind", "来源", "text", [
                ["shared", "共享"],
                ["dedicated", "独享"],
                ["byo", "自带落地"],
              ])}
              {field("expected_exit", "预期出口 IP")}
              {field("fixed_ip", "要求固定出口", "check")}
              {field("enabled", "启用", "check")}
              <p className="hint">
                关联线路通过实际出口探测后，绑定才会被标记为已验证。
              </p>
            </>
          )}
          {kind === "users" && (
            <>
              {field("username", "账号")}
              {field(
                "password",
                initial.id ? "新密码（留空保留）" : "密码（14–72 字节）",
                "password",
              )}
              {field(
                "role",
                "权限",
                "text",
                Object.entries(stateNames).filter(([k]) =>
                  [
                    "admin",
                    "operator",
                    "publisher",
                    "viewer",
                    "player",
                  ].includes(k),
                ),
              )}
              {field("max_sessions", "并发会话上限", "number")}
              {field("enabled", "启用", "check")}
            </>
          )}
          {kind === "releases" && (
            <>
              {field("version", "版本号")}
              {field("channel", "发布渠道", "text", [
                ["stable", "稳定版"],
                ["beta", "测试版"],
              ])}
              {field("url", "HTTPS 下载地址")}
              {field("sha256", "安装包 SHA256")}
              {field("signature", "可信发布清单签名")}
              {field("required", "必须更新", "check")}
              {field("published", "发布此版本", "check")}
            </>
          )}
        </div>
        {error && (
          <p className="alert" role="alert">
            {error}
          </p>
        )}
        <div className="form-actions">
          <button type="button" className="secondary" onClick={onClose}>
            取消
          </button>
          <button className="primary" disabled={busy}>
            {busy ? "正在保存…" : "保存"}
          </button>
        </div>
      </form>
    </Modal>
  );
}
function defaults(kind: string): Row {
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
createRoot(document.getElementById("root")!).render(<App />);
