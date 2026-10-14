import { useEffect, useId, useRef, type ReactNode } from "react";
import { X } from "lucide-react";
import type { Confirmable } from "../actions";

export function Modal({
  title,
  children,
  close,
  className,
}: {
  title: string;
  children: ReactNode;
  close: () => void;
  className?: string;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  const titleId = useId();
  useEffect(() => {
    ref.current?.showModal();
  }, []);
  return (
    <dialog
      ref={ref}
      className={className}
      aria-labelledby={titleId}
      onCancel={close}
    >
      <div className="modal-head">
        <h2 id={titleId}>{title}</h2>
        <button aria-label="关闭" onClick={close}>
          <X size={20} />
        </button>
      </div>
      {children}
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
  return (
    <Modal title={spec.title} close={onClose} className="confirm">
      <p className="confirm-message">{spec.message}</p>
      {running && (
        <p className="hint" role="status">
          请求已发送。关闭此窗口不会取消操作，完成后会在页面顶部提示结果。
        </p>
      )}
      {error && (
        <p className="alert" role="alert">
          {error}
        </p>
      )}
      <div className="form-actions">
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
    </Modal>
  );
}
