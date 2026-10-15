import type { Row } from "../api";
import { rollbackLine, rowLock, type Confirmable } from "../actions";
import { labels, text } from "../format";
import { Ago, Pill, RegionChips } from "../visuals";
import { Modal } from "./Modal";

/** Fields that read better as a relative time, a pill or chips. */
function value(k: string, v: unknown, names: Record<string, string>) {
  if (typeof v === "string" && (k.endsWith("_at") || k === "last_seen" || k === "lease_until" || k === "prepare_until" || k === "stop_until"))
    return <Ago at={v} />;
  if (k === "regions") return <RegionChips codes={v} />;
  if (typeof v === "boolean") return v ? <Pill tone="tone-good" label="是" plain /> : <Pill tone="tone-neutral" label="否" plain />;
  if (k === "state" || k === "status") return <Pill value={v} />;
  if (k === "role") return <Pill value={v} plain />;
  if (v !== null && typeof v === "object")
    return Array.isArray(v) && v.every((x) => typeof x === "string") ? (
      v.length ? (
        <span className="chips">
          {(v as string[]).map((x) => (
            <span className="chip" key={x}>
              {names[x] || x}
            </span>
          ))}
        </span>
      ) : (
        <span className="muted">—</span>
      )
    ) : (
      <pre>{JSON.stringify(v, null, 2)}</pre>
    );
  if (k === "id" || k.endsWith("_hash")) return <span className="mono">{String(v ?? "—")}</span>;
  return text(v, k, names);
}

/**
 * Field-by-field view of one row. For line versions it also offers rollback
 * to each verified version, through the same confirmation as row actions.
 */
export function DetailModal({
  detail,
  canPublish,
  pending,
  onConfirm,
  onClose,
  names = {},
}: {
  detail: Row;
  canPublish: boolean;
  pending: Record<string, string>;
  onConfirm: (spec: Confirmable) => void;
  onClose: () => void;
  names?: Record<string, string>;
}) {
  const lineBusy = detail.line_id ? !!pending[rowLock("lines", detail.line_id)] : false;
  if (detail.versions)
    return (
      <Modal drawer lightDismiss title="线路版本" subtitle={detail.line_name} close={onClose}>
        {detail.versions.length === 0 && <p className="muted">这条线路还没有发布过。</p>}
        <div className="versions">
          {detail.versions.map((v: Row) => (
            <div className="version" key={v.id}>
              <div>
                <b>v{v.number}</b>{" "}
                <Pill value={v.status} />
                <div className="muted" style={{ fontSize: 12, marginTop: 4 }}>
                  <Ago at={v.created_at} />
                  {v.probe?.rtt_ms ? " · 探测延迟 " + Math.round(v.probe.rtt_ms) + " ms" : ""}
                </div>
                {v.error && (
                  <div style={{ fontSize: 12, marginTop: 4, color: "var(--bad)" }}>失败原因：{v.error}</div>
                )}
              </div>
              {canPublish && v.status === "ready" && (
                <button
                  className="secondary sm"
                  disabled={lineBusy}
                  onClick={() => onConfirm(rollbackLine(detail.line_id, detail.line_name || detail.line_id, v))}
                >
                  切换到此版本
                </button>
              )}
            </div>
          ))}
        </div>
      </Modal>
    );
  const title = detail.name || detail.username || detail.version || "资源详情";
  return (
    <Modal drawer lightDismiss title={String(title)} subtitle="详细信息" close={onClose}>
      <dl className="kv">
        {Object.entries(detail)
          .filter(([k]) => k !== "title" && k !== "line_name")
          .map(([k, v]) => (
            <div key={k}>
              <dt>{labels[k] || k}</dt>
              <dd>{value(k, v, names)}</dd>
            </div>
          ))}
      </dl>
    </Modal>
  );
}
