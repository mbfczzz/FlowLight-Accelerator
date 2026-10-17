import { useEffect, useRef, useState, type ReactNode } from "react";
import { Globe, LoaderCircle, MonitorCog, RotateCw, Server } from "lucide-react";
import { api, isSessionExpired, type Row } from "../api";
import { messageOf } from "../useSectionData";
import { Ago } from "../visuals";
import { Modal } from "./Modal";

type Measure = {
  ok: boolean;
  rtt_ms: number;
  min_ms: number;
  max_ms: number;
  loss: number;
  samples: number;
  error?: string;
};

type CheckNode = {
  role: string;
  endpoint_id: string;
  endpoint_name: string;
  kind: string;
  address: string;
  range_start: number;
  range_end: number;
  managed: boolean;
  node_name: string;
  online: boolean;
  last_seen: string | null;
  agent_version: string;
  listen: "pending" | "ok" | "failed" | "skipped";
  listen_error?: string;
  port?: number;
  release?: "pending" | "done" | "expired";
};

type CheckLink = {
  id: string;
  from: number;
  to: number;
  udp_expected: boolean;
  status: "pending" | "ok" | "warn" | "failed" | "skipped";
  tcp?: Measure;
  udp?: Measure;
  exit_ip?: string;
  note?: string;
};

type Check = {
  id: string;
  version: number;
  phase: "listen" | "dial" | "done";
  nodes: CheckNode[];
  links: CheckLink[];
  requested_ports?: Record<string, number> | null;
  created_at: string;
};

/**
 * A check's steps land as its nodes report them; the control service hands
 * nodes their work the moment it exists, so a second between looks keeps up.
 */
const POLL_MS = 1000;

const kindNames: Record<string, string> = { realm: "Realm 中转", ss2022: "SS2022 落地" };

const linkStates: Record<CheckLink["status"], [string, string]> = {
  pending: ["检测中", "tone-neutral"],
  ok: ["连通", "tone-good"],
  warn: ["连通 · 有丢包", "tone-warn"],
  failed: ["不通", "tone-bad"],
  skipped: ["未检测", "tone-neutral"],
};

function ms(v: number) {
  return (v < 10 ? v.toFixed(1) : Math.round(v)) + " ms";
}

/** "12 ms（10–15）", or what went wrong. */
function measure(name: string, m: Measure | undefined, expected: boolean): ReactNode {
  if (!expected) return <span className="muted">{name} 未检测</span>;
  if (!m) return <span className="muted">{name} 等待结果</span>;
  if (!m.ok && !m.rtt_ms)
    return (
      <span className="check-bad">
        {name} 不通{m.error ? "：" + m.error : ""}
      </span>
    );
  const spread = m.max_ms - m.min_ms >= 1 ? "（" + Math.round(m.min_ms) + "–" + Math.round(m.max_ms) + "）" : "";
  const loss = m.loss > 0 ? " · 丢包 " + Math.round(m.loss * 100) + "%" : "";
  return (
    <span className={m.ok ? undefined : "check-bad"}>
      {name} <b className="figure">{ms(m.rtt_ms)}</b>
      {spread}
      {loss}
      {m.error && m.loss > 0 ? <span className="muted">：{m.error}</span> : null}
    </span>
  );
}

/**
 * The connectivity of a line, hop by hop: whether each node is online and
 * running its test listener, and for each link from the control service to
 * the entry, between nodes, and from the exit to the internet, whether TCP
 * and UDP get through and how long a round trip takes.
 */
export function LineCheckPanel({ line, canRun, onClose }: { line: Row; canRun: boolean; onClose: () => void }) {
  const [check, setCheck] = useState<Check | null | undefined>(undefined);
  const [error, setError] = useState("");
  const [starting, setStarting] = useState(false);
  // Test ports typed per endpoint; empty is automatic.
  const [ports, setPorts] = useState<Record<string, string>>({});
  const typed = useRef(false);
  const alive = useRef(true);
  useEffect(
    () => () => {
      alive.current = false;
    },
    [],
  );

  /** start: POST, with the ports chosen here, or none to keep the last ones. */
  async function run(start: boolean, chosen?: Record<string, number>) {
    setError("");
    if (start) setStarting(true);
    try {
      const result = await api(
        "/lines/" + line.id + "/check",
        start ? "POST" : "GET",
        start ? (chosen ? { ports: chosen } : {}) : undefined,
      );
      if (alive.current) setCheck(result);
    } catch (e) {
      if (!isSessionExpired(e) && alive.current) setError(messageOf(e));
    } finally {
      if (alive.current) setStarting(false);
    }
  }

  // Opening the panel is asking for a check now; a viewer sees the latest.
  useEffect(() => {
    void run(canRun);
  }, []);

  // Show the ports the check used until the admin types their own.
  useEffect(() => {
    if (typed.current || !check) return;
    const kept: Record<string, string> = {};
    for (const [id, port] of Object.entries(check.requested_ports || {})) kept[id] = String(port);
    setPorts(kept);
  }, [check?.id]);

  const running = !!check && check.phase !== "done";
  // After the result, until each node says its test port is closed.
  const releasing = !!check && check.nodes.some((n) => n.release === "pending");
  useEffect(() => {
    if (!running && !releasing) return;
    const t = setTimeout(() => void run(false), POLL_MS);
    return () => clearTimeout(t);
  }, [check, running, releasing]);

  const hops = check?.nodes || [];
  const linkTo = (to: number) => check?.links.find((l) => l.to === to);
  const failed = check?.links.filter((l) => l.status === "failed") || [];
  const lossy = check?.links.filter((l) => l.status === "warn") || [];
  const measured = check?.links.filter((l) => l.status !== "skipped") || [];
  const choosable = hops.filter((n) => n.managed);
  const portError = (n: CheckNode) => {
    const v = (ports[n.endpoint_id] || "").trim();
    if (!v) return "";
    const p = Number(v);
    return Number.isInteger(p) && p >= n.range_start && p <= n.range_end
      ? ""
      : "须在 " + n.range_start + "–" + n.range_end + " 之间";
  };
  const portsValid = choosable.every((n) => !portError(n));
  function rerun() {
    if (!portsValid || running || starting) return;
    const chosen: Record<string, number> = {};
    for (const n of choosable) {
      const v = (ports[n.endpoint_id] || "").trim();
      if (v) chosen[n.endpoint_id] = Number(v);
    }
    void run(true, chosen);
  }
  const stopName = (i: number) =>
    i < 0 ? "控制服务" : i >= hops.length ? "互联网" : hops[i].role + "「" + (hops[i].endpoint_name || "已删除的端点") + "」";

  return (
    <Modal
      drawer
      title="连通性检查"
      subtitle={
        (line.name || line.id) +
        (check ? (check.version ? " · 已发布版本 v" + check.version : " · 草稿路径（还没有发布）") : "")
      }
      close={onClose}
    >
      <div className="check-summary" role="status">
        {check === undefined && !error && (
          <p className="muted">
            <LoaderCircle size={15} className="spin" /> 正在读取…
          </p>
        )}
        {check === null && <p className="muted">这条线路还没有做过连通性检查。</p>}
        {running && (
          <div className="check-banner tone-neutral">
            <LoaderCircle size={16} className="spin" />
            <div>
              <b>{check?.phase === "listen" ? "正在让各节点开启检测端口…" : "正在测量每一段…"}</b>
              <span>节点每 2 秒领取一次任务，通常 10 秒内完成。</span>
            </div>
          </div>
        )}
        {check && !running && (
          <div className={"check-banner " + (failed.length ? "tone-bad" : lossy.length ? "tone-warn" : "tone-good")}>
            <div>
              <b>
                {failed.length
                  ? failed.length + " 段不通"
                  : lossy.length
                    ? "全部连通，" + lossy.length + " 段有丢包"
                    : "全部 " + measured.length + " 段连通"}
              </b>
              {failed.length > 0 && (
                <span>{failed.map((l) => stopName(l.from) + " → " + stopName(l.to)).join("；")}</span>
              )}
            </div>
          </div>
        )}
        {error && (
          <div className="alert" role="alert">
            <span>{error}</span>
          </div>
        )}
        {canRun && choosable.length > 0 && (
          <div className="check-ports">
            <div className="check-ports-head">
              <b>检测端口</b>
              <span className="muted">
                留空时用范围里最小的空闲端口，也就是下一个玩家会分到的端口；前置转发或防火墙只放行了部分端口时，可以在这里指定。
              </span>
            </div>
            {choosable.map((n) => {
              const id = "check-port-" + n.endpoint_id;
              const problem = portError(n);
              return (
                <div className="check-port" key={n.endpoint_id}>
                  <label htmlFor={id}>
                    {n.role}「{n.endpoint_name || "已删除的端点"}」
                  </label>
                  <input
                    id={id}
                    inputMode="numeric"
                    placeholder="自动"
                    value={ports[n.endpoint_id] || ""}
                    aria-invalid={problem ? true : undefined}
                    aria-describedby={id + "-hint"}
                    onChange={(e) => {
                      typed.current = true;
                      setPorts((p) => ({ ...p, [n.endpoint_id]: e.target.value }));
                    }}
                    onKeyDown={(e) => {
                      if (e.key === "Enter") rerun();
                    }}
                  />
                  <small id={id + "-hint"} className={problem ? "check-bad" : "muted"}>
                    {problem || "范围 " + n.range_start + "–" + n.range_end}
                  </small>
                </div>
              );
            })}
          </div>
        )}
        {check && (
          <div className="check-meta">
            <span className="muted">
              检查于 <Ago at={check.created_at} />
            </span>
            {canRun && (
              <button className="secondary sm" disabled={running || starting || !portsValid} onClick={rerun}>
                <RotateCw size={14} />
                重新检查
              </button>
            )}
          </div>
        )}
      </div>

      {check && (
        <ol className="check-path">
          <Stop icon={<MonitorCog size={16} />} title="控制服务" detail="从控制服务所在的服务器出发，看入口能否从公网连上" />
          {hops.map((n, i) => (
            <LinkAndStop key={i} link={linkTo(i)}>
              <Stop
                icon={<Server size={16} />}
                tag={n.role}
                title={n.endpoint_name || "已删除的端点"}
                detail={
                  <>
                    <span>
                      {kindNames[n.kind] || n.kind} · <span className="mono">{n.address}</span>
                    </span>
                    <span>
                      节点服务器「{n.node_name || "已删除"}」 ·{" "}
                      {n.online ? (
                        <span className="check-good">在线</span>
                      ) : (
                        <span className="check-bad">
                          不在线
                          {n.last_seen && (
                            <>
                              （最后心跳 <Ago at={n.last_seen} />）
                            </>
                          )}
                        </span>
                      )}
                      {n.agent_version && " · 节点程序 " + n.agent_version}
                    </span>
                    <span>
                      {n.listen === "ok" &&
                        "检测端口 " +
                          n.port +
                          (check.requested_ports?.[n.endpoint_id] ? "（指定）" : "（自动选择）") +
                          (n.release === "done"
                            ? " · 检查结束，已关闭"
                            : n.release === "pending"
                              ? " · 检查结束，正在关闭…"
                              : n.release === "expired"
                                ? " · 节点没有确认关闭，开启约 90 秒后会自动关闭"
                                : " · 已开启")}
                      {n.listen === "pending" && "正在开启检测端口…"}
                      {n.listen === "skipped" && "不由平台运行：直接连接它自己的服务，只检测 TCP"}
                      {n.listen === "failed" && <span className="check-bad">检测端口没有开启：{n.listen_error}</span>}
                    </span>
                  </>
                }
              />
            </LinkAndStop>
          ))}
          <LinkAndStop link={linkTo(hops.length)}>
            <Stop
              icon={<Globe size={16} />}
              title="互联网"
              detail={
                linkTo(hops.length)?.exit_ip ? (
                  <span>
                    落地的出口 IP <span className="mono">{linkTo(hops.length)?.exit_ip}</span>
                  </span>
                ) : (
                  "落地访问探测服务，检查它能否正常出网"
                )
              }
            />
          </LinkAndStop>
        </ol>
      )}
    </Modal>
  );
}

function Stop({ icon, tag, title, detail }: { icon: ReactNode; tag?: string; title: string; detail: ReactNode }) {
  return (
    <li className="check-stop">
      <span className="check-icon">{icon}</span>
      <div>
        <div className="check-title">
          {tag && <span className="pill tone-neutral plain">{tag}</span>}
          <b>{title}</b>
        </div>
        <div className="check-detail">{detail}</div>
      </div>
    </li>
  );
}

/** The link that arrives at a stop, drawn on the line above it. */
function LinkAndStop({ link, children }: { link?: CheckLink; children: ReactNode }) {
  const [label, tone] = linkStates[link?.status || "pending"];
  return (
    <>
      <li className="check-link" data-status={link?.status || "pending"}>
        <span className="check-rail" aria-hidden />
        <div>
          <span className={"pill " + tone}>{label}</span>
          {link && link.status !== "skipped" && !(link.status === "failed" && !link.tcp) && (
            <span className="check-figures">
              {measure("TCP", link.tcp, true)}
              {measure("UDP", link.udp, link.udp_expected)}
            </span>
          )}
          {link?.note && (
            <span className={"check-note " + (link.status === "skipped" ? "muted" : "check-bad")}>{link.note}</span>
          )}
        </div>
      </li>
      {children}
    </>
  );
}
