import type { Row } from "../api";
import { rollbackLine, rowLock, type Confirmable } from "../actions";
import { labels, text } from "../format";
import { Modal } from "./Modal";

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
}: {
  detail: Row;
  canPublish: boolean;
  pending: Record<string, string>;
  onConfirm: (spec: Confirmable) => void;
  onClose: () => void;
}) {
  const lineBusy = detail.line_id
    ? !!pending[rowLock("lines", detail.line_id)]
    : false;
  return (
    <Modal title={detail.title || "资源详情"} close={onClose}>
      <dl className="details">
        {Object.entries(detail)
          .filter(([k]) => k !== "title" && k !== "line_name")
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
                disabled={lineBusy}
                onClick={() =>
                  onConfirm(
                    rollbackLine(
                      detail.line_id,
                      detail.line_name || detail.line_id,
                      v,
                    ),
                  )
                }
              >
                切换到版本 {v.number}
              </button>
            ))}
        </div>
      )}
    </Modal>
  );
}
