import { useEffect, useRef, useState, type ReactNode } from "react";
import { Ellipsis, Eye, History, KeyRound, LoaderCircle, OctagonX, Pause, Pencil, Play, Rocket } from "lucide-react";
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

  function guarded(
    spec: Confirmable,
    label: string,
    icon: ReactNode,
    tone?: "emphasis" | "destructive",
    iconOnly = false,
  ) {
    const running = inFlight === spec.action;
    return (
      <button
        key={spec.action}
        className={[tone, iconOnly ? "icon-only" : ""].filter(Boolean).join(" ") || undefined}
        disabled={!!inFlight}
        aria-busy={running || undefined}
        aria-label={iconOnly ? label : undefined}
        title={iconOnly ? label : undefined}
        onClick={() => onConfirm(spec)}
      >
        {running ? <LoaderCircle size={14} /> : icon}
        {iconOnly ? null : running ? "处理中…" : label}
      </button>
    );
  }

  // A draft game is published here; once published, edits go through the
  // same button again, so it stays the emphasised action until it is live.
  return (
    <>
      {section === "games" &&
        canPublish &&
        guarded(publishGame(row), "发布规则", <Rocket size={14} />, row.published_version ? undefined : "emphasis")}
      {section === "lines" && canPublish &&
        guarded(publishLine(row), "发布", <Rocket size={14} />, row.published_version ? undefined : "emphasis")}
      {section === "nodes" &&
        canEdit &&
        guarded(regenerateEnrollment(row), "注册凭据", <KeyRound size={14} />, row.agent_version ? undefined : "emphasis")}
      {section === "sessions" &&
        canEdit &&
        row.state !== "CLOSED" &&
        guarded(stopSession(row), "停止", <OctagonX size={14} />, "destructive")}
      {editable && (
        <button onClick={() => onEdit(row)}>
          <Pencil size={14} />
          编辑
        </button>
      )}
      {section === "lines" && (
        <MoreMenu
          disabled={!!inFlight}
          items={[
            ...(canPublish
              ? [
                  {
                    label: row.draining ? "恢复接收新会话" : "排空：不再接收新会话",
                    icon: row.draining ? <Play size={15} /> : <Pause size={15} />,
                    run: () => onConfirm(drainLine(row)),
                  },
                ]
              : []),
            { label: "查看版本与验证记录", icon: <History size={15} />, run: () => onVersions(row) },
            ...(canPublish
              ? [
                  {
                    label: "终止线路",
                    icon: <OctagonX size={15} />,
                    run: () => onConfirm(terminateLine(row)),
                    danger: true,
                  },
                ]
              : []),
          ]}
        />
      )}
      <button className="icon-only" aria-label="详情" title="详情" onClick={() => onDetail(row)}>
        <Eye size={15} />
      </button>
    </>
  );
}

type MenuItem = { label: string; icon: ReactNode; run: () => void; danger?: boolean };

/**
 * Less frequent row actions behind one button. The list is positioned
 * against the viewport so the table's scroll container cannot clip it.
 */
function MoreMenu({ items, disabled }: { items: MenuItem[]; disabled?: boolean }) {
  const [at, setAt] = useState<{ top?: number; bottom?: number; right: number } | null>(null);
  const button = useRef<HTMLButtonElement>(null);
  const menu = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!at) return;
    const close = (e: Event) => {
      if (e.type === "keydown" && (e as KeyboardEvent).key !== "Escape") return;
      // The button's own click toggles the menu.
      if (
        e.type === "mousedown" &&
        (menu.current?.contains(e.target as Node) || button.current?.contains(e.target as Node))
      )
        return;
      setAt(null);
      if (e.type === "keydown") button.current?.focus();
    };
    window.addEventListener("mousedown", close);
    window.addEventListener("keydown", close);
    window.addEventListener("scroll", close, true);
    window.addEventListener("resize", close);
    menu.current?.querySelector<HTMLElement>("button")?.focus();
    return () => {
      window.removeEventListener("mousedown", close);
      window.removeEventListener("keydown", close);
      window.removeEventListener("scroll", close, true);
      window.removeEventListener("resize", close);
    };
  }, [at]);
  return (
    <>
      <button
        ref={button}
        aria-haspopup="menu"
        aria-expanded={!!at}
        disabled={disabled}
        onClick={() => {
          if (at) return setAt(null);
          const r = button.current!.getBoundingClientRect();
          const right = window.innerWidth - r.right;
          // Open upwards when the rows below the button cannot hold the menu.
          const tall = items.length * 38 + 12;
          setAt(
            r.bottom + 6 + tall > window.innerHeight && r.top - 6 - tall > 0
              ? { bottom: window.innerHeight - r.top + 6, right }
              : { top: r.bottom + 6, right },
          );
        }}
      >
        <Ellipsis size={15} />
        更多
      </button>
      {at && (
        <div
          ref={menu}
          className="menu"
          role="menu"
          style={at}
          onKeyDown={(e) => {
            if (e.key !== "ArrowDown" && e.key !== "ArrowUp") return;
            e.preventDefault();
            const all = [...menu.current!.querySelectorAll<HTMLElement>("button")];
            const i = all.indexOf(document.activeElement as HTMLElement);
            all[(i + (e.key === "ArrowDown" ? 1 : all.length - 1)) % all.length]?.focus();
          }}
        >
          {items.map((item) => (
            <button
              key={item.label}
              role="menuitem"
              className={item.danger ? "danger-item" : undefined}
              onClick={() => {
                setAt(null);
                item.run();
              }}
            >
              {item.icon}
              {item.label}
            </button>
          ))}
        </div>
      )}
    </>
  );
}
