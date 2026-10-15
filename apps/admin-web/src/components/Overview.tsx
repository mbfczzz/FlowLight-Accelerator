import { Check, ChevronRight, Gamepad2, Route, Server, Zap } from "lucide-react";
import type { ReactNode } from "react";
import type { Row } from "../api";

type Tile = {
  key: string;
  label: string;
  note: string;
  /** The count of what this tile is a part of, if any, and its unit. */
  of?: string;
  unit?: string;
  icon: ReactNode;
};

const tiles: Tile[] = [
  { key: "active_sessions", label: "当前会话", note: "包含发布时的验证会话", icon: <Zap size={17} /> },
  { key: "online_nodes", label: "在线节点", note: "45 秒内有心跳", of: "nodes", unit: "个节点", icon: <Server size={17} /> },
  { key: "verified_versions", label: "已验证线路版本", note: "通过实际 TCP / UDP 探测", of: "lines", unit: "条线路", icon: <Route size={17} /> },
  { key: "published_games", label: "已发布游戏", note: "客户端可见", of: "games", unit: "款游戏", icon: <Gamepad2 size={17} /> },
];

/**
 * A tile's tone reports state: green when there is some, amber when there
 * is something to have but none of it is working yet, grey otherwise.
 */
function tileTone(t: Tile, o: Row) {
  const n = count(o, t.key);
  if (n === undefined) return "tone-neutral";
  if (n > 0) return "tone-good";
  const whole = t.of ? count(o, t.of) : undefined;
  return whole ? "tone-warn" : "tone-neutral";
}

type Step = {
  key: string;
  title: string;
  done: (o: Row) => boolean | undefined;
  detail: (o: Row) => string;
  section: string;
  action: string;
};

const count = (o: Row, k: string) => (typeof o[k] === "number" ? (o[k] as number) : undefined);
const known = (...v: (number | undefined)[]) => v.every((x) => x !== undefined);

/**
 * Setup in the order it has to happen. Each step is done when the control
 * service reports at least one of what it needs.
 */
const steps: Step[] = [
  {
    key: "games",
    title: "发布游戏规则",
    done: (o) => (known(count(o, "published_games")) ? count(o, "published_games")! > 0 : undefined),
    detail: (o) =>
      known(count(o, "games"), count(o, "published_games"))
        ? `${count(o, "games")} 款游戏，已发布 ${count(o, "published_games")} 款。填好区服和进程后点“发布规则”。`
        : "填好区服和进程后点“发布规则”。",
    section: "games",
    action: "去发布",
  },
  {
    key: "nodes",
    title: "接入节点服务器",
    done: (o) => (known(count(o, "online_nodes")) ? count(o, "online_nodes")! > 0 : undefined),
    detail: (o) =>
      known(count(o, "nodes"), count(o, "online_nodes"))
        ? `已添加 ${count(o, "nodes")} 台，在线 ${count(o, "online_nodes")} 台。添加服务器后生成注册凭据，在服务器上运行节点 Agent。`
        : "添加服务器后生成注册凭据，在服务器上运行节点 Agent。",
    section: "nodes",
    action: "管理节点",
  },
  {
    key: "endpoints",
    title: "配置服务端点",
    done: (o) => (known(count(o, "endpoints")) ? count(o, "endpoints")! > 0 : undefined),
    detail: (o) =>
      known(count(o, "endpoints"))
        ? `已有 ${count(o, "endpoints")} 个端点。在节点服务器上开入口、中转或落地服务；每条线路至少需要一个 SS2022 落地端点。`
        : "在节点服务器上开入口、中转或落地服务；每条线路至少需要一个 SS2022 落地端点。",
    section: "endpoints",
    action: "管理端点",
  },
  {
    key: "lines",
    title: "发布并验证线路",
    done: (o) => (known(count(o, "verified_versions")) ? count(o, "verified_versions")! > 0 : undefined),
    detail: (o) =>
      known(count(o, "lines"), count(o, "verified_versions"))
        ? `${count(o, "lines")} 条线路，${count(o, "verified_versions")} 个版本通过验证。探测通过后玩家才能选择。`
        : "探测通过后玩家才能选择。",
    section: "lines",
    action: "管理线路",
  },
  {
    key: "players",
    title: "开通玩家账号",
    done: (o) => (known(count(o, "players")) ? count(o, "players")! > 0 : undefined),
    detail: (o) =>
      known(count(o, "players"))
        ? `已有 ${count(o, "players")} 个玩家账号。玩家用它登录客户端。`
        : "玩家用它登录客户端。",
    section: "users",
    action: "管理账号",
  },
];

export function Overview({
  overview,
  loading,
  onOpen,
}: {
  overview: Row;
  loading: boolean;
  onOpen: (section: string) => void;
}) {
  const states = steps.map((s) => (loading ? undefined : s.done(overview)));
  const done = states.filter(Boolean).length;
  const knownSteps = states.some((s) => s !== undefined);
  return (
    <>
      <div className="tiles">
        {tiles.map((t) => (
          <div className="tile" key={t.key}>
            <div className="tile-head">
              <span className={"tile-icon " + (loading ? "tone-neutral" : tileTone(t, overview))}>{t.icon}</span>
              {t.label}
            </div>
            <strong>{loading ? "—" : (overview[t.key] ?? "—")}</strong>
            <small>
              {!loading && t.of && count(overview, t.of) !== undefined
                ? `共 ${count(overview, t.of)} ${t.unit} · ${t.note}`
                : t.note}
            </small>
          </div>
        ))}
      </div>
      <section className="panel checklist">
        <div className="checklist-head">
          <h2>上线准备</h2>
          <span>{knownSteps ? `已完成 ${done} / ${steps.length}` : "—"}</span>
        </div>
        <div className="progress" aria-hidden="true">
          <i style={{ width: knownSteps ? `${(done / steps.length) * 100}%` : "0%" }} />
        </div>
        {steps.map((s, i) => {
          const isDone = states[i] === true;
          return (
            <div className={"step" + (isDone ? " done" : "")} key={s.key}>
              <span className="step-mark">{isDone ? <Check size={16} /> : i + 1}</span>
              <span>
                <b>{s.title}</b>
                <small>{s.detail(loading ? {} : overview)}</small>
              </span>
              <button className={isDone ? "ghost sm" : "secondary sm"} onClick={() => onOpen(s.section)}>
                {isDone ? "查看" : s.action}
                <ChevronRight size={14} />
              </button>
            </div>
          );
        })}
      </section>
    </>
  );
}
