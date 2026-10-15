import { useEffect, useId, useRef, type ReactNode } from "react";
import { CircleAlert, TriangleAlert, X } from "lucide-react";
import type { Confirmable } from "../actions";

/**
 * A native dialog. `drawer` slides it in from the right for editing and
 * details, so the list stays in view; `bare` leaves layout of the body to
 * the caller (a form with its own footer).
 */
export function Modal({
  title,
  subtitle,
  children,
  close,
  className,
  drawer,
  bare,
  lightDismiss,
}: {
  title: string;
  subtitle?: string;
  children: ReactNode;
  close: () => void;
  className?: string;
  drawer?: boolean;
  bare?: boolean;
  /** Close on a backdrop click; off for forms, where it would lose input. */
  lightDismiss?: boolean;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  const titleId = useId();
  useEffect(() => {
    ref.current?.showModal();
  }, []);
  return (
    <dialog
      ref={ref}
      className={[drawer ? "drawer" : "", className || ""].join(" ").trim()}
      aria-labelledby={titleId}
      onCancel={(e) => {
        e.preventDefault();
        close();
      }}
      onClick={(e) => {
        // A click on the backdrop lands on the dialog element itself.
        if (lightDismiss && e.target === e.currentTarget) close();
      }}
    >
      <div className="modal-head">
        <div>
          <h2 id={titleId}>{title}</h2>
          {subtitle && <p>{subtitle}</p>}
        </div>
        <button className="icon-btn" aria-label="关闭" onClick={close}>
          <X size={18} />
        </button>
      </div>
      {bare ? children : <div className="modal-body">{children}</div>}
    </dialog>
  );
}

/**
 * Ask before a state-changing request. While the request runs the confirm
 * button is disabled; closing the dialog does not cancel the request, so the
 * secondary button says so instead of offering a cancel it cannot honour.
 */
export function ConfirmDialog({
  spec,
  running,
  error,
  onConfirm,
  onClose,
}: {
  spec: Confirmable;
  running: boolean;
  error: string;
  onConfirm: () => void;
  onClose: () => void;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  const titleId = useId();
  useEffect(() => {
    ref.current?.showModal();
  }, []);
  return (
    <dialog
      ref={ref}
      className="confirm"
      aria-labelledby={titleId}
      onCancel={(e) => {
        e.preventDefault();
        onClose();
      }}
    >
      <div className="confirm-body">
        <span className={"confirm-icon " + (spec.danger ? "tone-bad" : "tone-accent")}>
          {spec.danger ? <TriangleAlert size={20} /> : <CircleAlert size={20} />}
        </span>
        <div>
          <h2 id={titleId}>{spec.title}</h2>
          <p>{spec.message}</p>
          {running && (
            <p className="hint" role="status">
              请求已发送。关闭此窗口不会取消操作，完成后会在页面顶部提示结果。
            </p>
          )}
          {error && (
            <div className="alert" role="alert">
              <span>{error}</span>
            </div>
          )}
        </div>
      </div>
      <div className="confirm-foot">
        <button type="button" className="secondary" onClick={onClose}>
          {running ? "关闭" : "取消"}
        </button>
        <button
          type="button"
          className={spec.danger ? "danger" : "primary"}
          disabled={running}
          onClick={onConfirm}
        >
          {running ? "正在处理…" : error ? "重试" : spec.confirmLabel}
        </button>
      </div>
    </dialog>
  );
}
