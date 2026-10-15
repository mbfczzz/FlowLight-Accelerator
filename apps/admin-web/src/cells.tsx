import type { ReactNode } from "react";
import {
  Cable,
  MonitorSmartphone,
  Route,
  Server,
} from "lucide-react";
import type { Row } from "./api";
import { formatBytes, formatDuration, stateNames, text } from "./format";
import {
  Ago,
  Avatar,
  Cover,
  Entity,
  Heartbeat,
  Pill,
  RegionChips,
  regionLabel,
} from "./visuals";

type Names = Record<string, string>;
type Covers = Record<string, { index?: number; image?: string }>;

const nameOf = (id: unknown, names: Names) =>
  typeof id === "string" && id ? names[id] || id : "";

function Ref({ id, names }: { id: unknown; names: Names }) {
  if (typeof id !== "string" || !id) return <span className="muted">—</span>;
  return names[id] ? (
    <span title={id}>{names[id]}</span>
  ) : (
    <span className="mono">{id.slice(0, 12)}</span>
  );
}

const icon = (node: ReactNode) => <span className="entity-icon">{node}</span>;

const enabled = (v: unknown) =>
  v ? <Pill tone="tone-good" label="启用" /> : <Pill tone="tone-neutral" label="停用" />;

/** One table cell, drawn for what the column holds in this section. */
export function renderCell(
  section: string,
  col: string,
  row: Row,
  names: Names,
  covers: Covers = {},
): ReactNode {
  const v = row[col];
  if (col === "enabled") return enabled(v);
  if (col === "regions") return <RegionChips codes={v} />;
  if (col === "lease_until" || col === "created_at" || (col === "last_seen" && section !== "nodes"))
    return <Ago at={v} />;

  switch (section) {
    case "games":
      if (col === "name") {
        const processes: string[] = row.draft?.processes || [];
        return (
          <Entity
            lead={<Cover seed={row.id} name={row.name} index={row.cover_index} image={row.cover_image} />}
            title={row.name}
            sub={processes.length ? processes.join("、") : "未填写游戏进程"}
          />
        );
      }
      if (col === "published_version")
        return v ? (
          <Pill tone="tone-good" label={"已发布 v" + v} />
        ) : (
          <Pill tone="tone-warn" label="草稿" />
        );
      break;
    case "lines":
      if (col === "name")
        return (
          <Entity
            lead={icon(<Route size={17} />)}
            title={row.name}
            sub={(row.endpoint_ids?.length || 0) + " 跳 · 优先级 " + (row.priority ?? 0)}
          />
        );
      if (col === "mode") return <Pill value={v} plain tone="tone-neutral" />;
      if (col === "capacity") return <span className="figure">{v}</span>;
      if (col === "published_version") return <LineState row={row} />;
      if (col === "draining")
        return !row.published_version ? (
          <Pill tone="tone-neutral" label="未上线" />
        ) : v ? (
          <Pill tone="tone-warn" label="排空中" />
        ) : (
          <Pill tone="tone-good" label="接收新会话" />
        );
      break;
    case "nodes":
      if (col === "name")
        return <Entity lead={icon(<Server size={17} />)} title={row.name} sub={row.agent_version ? "Agent " + row.agent_version : "尚未注册"} />;
      if (col === "region") return v ? <RegionChips codes={[String(v)]} /> : <span className="muted">—</span>;
      if (col === "address") return <span className="mono">{v || "—"}</span>;
      if (col === "last_seen") return <Heartbeat at={v} />;
      break;
    case "endpoints":
      if (col === "name")
        return <Entity lead={icon(<Cable size={17} />)} title={row.name} sub={stateNames[row.kind] || row.kind} />;
      if (col === "node_id") return <Ref id={v} names={names} />;
      if (col === "kind") return <Pill value={v} plain tone="tone-neutral" />;
      if (col === "host") return <span className="mono">{v || "—"}</span>;
      if (col === "port" || col === "port_end") return <span className="figure">{v}</span>;
      break;
    case "bindings":
      if (col === "user_id") {
        const n = nameOf(v, names);
        return <Entity lead={<Avatar name={n} />} title={n || "—"} />;
      }
      if (col === "endpoint_id") return <Ref id={v} names={names} />;
      if (col === "kind") return <Pill value={v} plain tone="tone-neutral" />;
      if (col === "verified_at")
        return v && !String(v).startsWith("0001-") ? (
          <Pill tone="tone-good" label={<>已验证 · <Ago at={v} /></>} />
        ) : (
          <Pill tone="tone-neutral" label="未验证" />
        );
      break;
    case "users":
      if (col === "username")
        return <Entity lead={<Avatar name={row.username} />} title={row.username} sub={roleNote[row.role]} />;
      // A role is a kind of account, not a state: no colour, no dot.
      if (col === "role") return <Pill value={v} plain />;
      if (col === "max_sessions") return <span className="figure">{v}</span>;
      break;
    case "devices":
      if (col === "name") return <Entity lead={icon(<MonitorSmartphone size={17} />)} title={row.name} />;
      if (col === "user_id") return <Ref id={v} names={names} />;
      break;
    case "sessions":
      if (col === "user_id") {
        const n = nameOf(v, names);
        return <Entity lead={<Avatar name={n} />} title={n || "—"} sub={stateNames[row.purpose] || row.purpose} />;
      }
      if (col === "game_id") {
        const n = nameOf(v, names);
        return (
          <Entity
            lead={
              <Cover
                seed={String(v)}
                name={n}
                index={covers[String(v)]?.index}
                image={covers[String(v)]?.image}
                width={44}
                height={25}
              />
            }
            title={n || "—"}
            sub={row.region_id ? regionLabel(row.region_id) : undefined}
          />
        );
      }
      if (col === "line_id") return <Ref id={v} names={names} />;
      if (col === "state") return <Pill value={v} />;
      if (col === "bytes")
        return v > 0 ? <span className="figure">{formatBytes(v)}</span> : <span className="muted">—</span>;
      if (col === "duration") {
        const start = Date.parse(row.created_at);
        // A closed session ended when it was last written.
        const end = row.state === "CLOSED" ? Date.parse(row.updated_at) : Date.now();
        return Number.isFinite(start) && Number.isFinite(end) && end >= start ? (
          <span className="figure">{formatDuration(end - start)}</span>
        ) : (
          <span className="muted">—</span>
        );
      }
      if (col === "end_reason") return v ? <span className="muted">{text(v, col)}</span> : <span className="muted">—</span>;
      break;
    case "jobs":
      if (col === "node_id") return <Ref id={v} names={names} />;
      if (col === "kind") return <Pill value={v} plain tone="tone-neutral" />;
      if (col === "status") return <Pill value={v} />;
      if (col === "attempts") return <span className="figure">{v}</span>;
      if (col === "result")
        return v ? <span className="muted" title={String(v)}>{String(v).slice(0, 80)}</span> : <span className="muted">—</span>;
      break;
    case "releases":
      if (col === "version") return <b className="figure">{v}</b>;
      if (col === "channel") return <Pill value={v} plain />;
      if (col === "published")
        return v ? <Pill tone="tone-good" label="已发布" /> : <Pill tone="tone-neutral" label="未发布" />;
      break;
    case "audits":
      if (col === "actor") {
        const n = nameOf(v, names);
        return <Entity lead={<Avatar name={n} />} title={n || "系统"} />;
      }
      if (col === "action") return <b>{text(v, col)}</b>;
      if (col === "resource") return <Ref id={v} names={names} />;
      break;
  }
  return text(v, col, names);
}

/**
 * Whether a line is live and what its newest version's verification says.
 * Published means a verified version is serving; a newer version can still
 * be validating or have failed beside it.
 */
function LineState({ row }: { row: Row }) {
  const latest = row.latest_status as string | undefined;
  const reason = row.latest_error ? String(row.latest_error) : undefined;
  const live = !!row.published_version;
  const newer =
    latest === "validating" ? (
      <Pill tone="tone-warn" label={live ? "新版本验证中" : "验证中"} />
    ) : latest === "failed" ? (
      <Pill tone="tone-bad" label={live ? "新版本未通过" : "验证未通过"} />
    ) : null;
  // Why the newest version failed, in full on hover.
  const why =
    latest === "failed" && reason ? (
      <small className="state-reason" title={reason}>
        {reason}
      </small>
    ) : null;
  if (!live && !newer) return <Pill tone="tone-neutral" label="未发布" />;
  return (
    <span className="state">
      <span className="chips">
        {live && <Pill tone="tone-good" label="已验证上线" />}
        {newer}
      </span>
      {why}
    </span>
  );
}

/** What each role may do, shown under the username and in the editor. */
export const roleNote: Record<string, string> = {
  player: "只能用客户端加速，不能登录后台",
  viewer: "查看全部配置与运行状态",
  operator: "编辑游戏、节点、端点和线路",
  publisher: "编辑并发布游戏规则和线路",
  admin: "全部权限，包括账号与落地绑定",
};
