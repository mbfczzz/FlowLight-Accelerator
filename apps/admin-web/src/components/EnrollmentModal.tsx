import { Copy } from "lucide-react";
import type { Row } from "../api";
import { Field } from "./controls";
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
    <Modal title="节点注册凭据" subtitle="交给这台节点的部署人员；注册成功后凭据自动失效。" close={onClose}>
      <div className="secret">
        <Field label="节点标识">
          <input readOnly value={enroll.node_id} onFocus={(e) => e.target.select()} />
        </Field>
        <Field label="一次性引导凭据" note="只显示这一次，关闭后无法再次查看，需要时重新生成。">
          <input readOnly value={enroll.bootstrap_secret} onFocus={(e) => e.target.select()} />
        </Field>
        <button
          className="primary"
          onClick={() =>
            navigator.clipboard.writeText(JSON.stringify(enroll, null, 2)).then(onCopied)
          }
        >
          <Copy size={15} />
          复制注册信息
        </button>
      </div>
    </Modal>
  );
}
