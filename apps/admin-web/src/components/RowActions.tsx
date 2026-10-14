import type { Row } from "../api";
import {
  drainLine,
  publishGame,
  publishLine,
  regenerateEnrollment,
  rowLock,
  stopSession,
  terminateLine,
  type Confirmable,
} from "../actions";
import type { Permissions } from "../sections";

/**
 * Buttons in a row's 操作 cell. Every state-changing button opens a
 * confirmation through `onConfirm`; while any of them is in flight for this
 * row (`pending[lock]` names which), all of them stay disabled.
 */
export function RowActions({
  section,
  row,
  editable,
  permissions,
  pending,
  onDetail,
  onEdit,
  onVersions,
  onConfirm,
}: {
  section: string;
  row: Row;
  editable: boolean;
  permissions: Permissions;
  pending: Record<string, string>;
  onDetail: (row: Row) => void;
  onEdit: (row: Row) => void;
  onVersions: (row: Row) => void;
  onConfirm: (spec: Confirmable) => void;
}) {
  const { canEdit, canPublish } = permissions;
  const inFlight = pending[rowLock(section, row.id)];

  function guarded(spec: Confirmable, label: string, destructive = false) {
    const running = inFlight === spec.action;
    return (
      <button
        key={spec.action}
        className={destructive ? "destructive" : undefined}
        disabled={!!inFlight}
        aria-busy={running || undefined}
        onClick={() => onConfirm(spec)}
      >
        {running ? "处理中…" : label}
      </button>
    );
  }

  return (
    <>
      <button onClick={() => onDetail(row)}>详情</button>
      {editable && <button onClick={() => onEdit(row)}>编辑</button>}
      {section === "games" && canPublish && guarded(publishGame(row), "发布规则")}
      {section === "lines" && canPublish && (
        <>
          {guarded(publishLine(row), "发布")}
          {guarded(drainLine(row), row.draining ? "接收新会话" : "排空")}
          {guarded(terminateLine(row), "终止", true)}
          <button onClick={() => onVersions(row)}>版本</button>
        </>
      )}
      {section === "nodes" &&
        canEdit &&
        guarded(regenerateEnrollment(row), "注册凭据")}
      {section === "sessions" &&
        canEdit &&
        row.state !== "CLOSED" &&
        guarded(stopSession(row), "停止", true)}
    </>
  );
}
