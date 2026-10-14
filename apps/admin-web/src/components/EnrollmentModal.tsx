import type { Row } from "../api";
import { Modal } from "./Modal";

export function EnrollmentModal({
  enroll,
  onCopied,
  onClose,
}: {
  enroll: Row;
  onCopied: () => void;
  onClose: () => void;
}) {
  return (
    <Modal title="节点一次性注册凭据" close={onClose}>
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
            .then(onCopied)
        }
      >
        复制注册信息
      </button>
    </Modal>
  );
}
