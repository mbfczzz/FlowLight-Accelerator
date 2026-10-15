import { useState } from "react";
import { Copy, Terminal } from "lucide-react";
import type { Row } from "../api";
import { Field } from "./controls";
import { Modal } from "./Modal";

const addressKey = "flowlight-control-address";

function remembered() {
  try {
    return localStorage.getItem(addressKey) || "";
  } catch {
    return "";
  }
}

/** https://, or plain http to this machine when testing locally. */
function validAddress(v: string) {
  try {
    const u = new URL(v);
    if (u.protocol === "https:") return !!u.host;
    return u.protocol === "http:" && (u.hostname === "127.0.0.1" || u.hostname === "localhost");
  } catch {
    return false;
  }
}

/**
 * Joins a node server: one command that downloads the node programs from
 * the control service, registers with the one-time secret and starts the
 * agent. The control service names its own address (PUBLIC_URL); when it
 * does not, the operator types it once and the browser keeps it.
 */
export function EnrollmentModal({
  enroll,
  name,
  onCopied,
  onClose,
}: {
  enroll: Row;
  name?: string;
  onCopied: (what: string) => void;
  onClose: () => void;
}) {
  const given = typeof enroll.control_url === "string" ? enroll.control_url : "";
  const fixed = validAddress(given) ? given : "";
  const [typed, setTyped] = useState(remembered);
  const control = (fixed || typed.trim()).replace(/\/+$/, "");
  const ready = validAddress(control);
  const local = control.startsWith("http:");
  const command = ready
    ? `curl -fsSL ${control}/dist/node/install.sh | sudo bash -s -- --control ${control} --node ${enroll.node_id} --secret ${enroll.bootstrap_secret}` +
      (local ? " --dev-http" : "")
    : "";
  const copy = (text: string, what: string) => navigator.clipboard.writeText(text).then(() => onCopied(what));

  return (
    <Modal
      title={"接入节点服务器" + (name ? "：" + name : "")}
      subtitle="在这台服务器上运行一条命令即可完成安装和注册。"
      close={onClose}
    >
      <div className="secret">
        {!fixed && (
          <Field
            label="控制服务地址"
            note="节点服务器连接的 https 地址，例如 https://accel.example.com:8443。在服务器配置 PUBLIC_URL 后这里会自动填好。"
          >
            <input
              value={typed}
              placeholder="https://"
              onChange={(e) => {
                setTyped(e.target.value);
                try {
                  localStorage.setItem(addressKey, e.target.value.trim());
                } catch {
                  /* the address is only a convenience */
                }
              }}
            />
          </Field>
        )}
        <div className="command">
          <div className="command-head">
            <Terminal size={15} />
            用 root 在这台服务器上运行
          </div>
          <code>{command || "填写控制服务地址后生成命令"}</code>
        </div>
        <button className="primary" disabled={!ready} onClick={() => copy(command, "已复制接入命令")}>
          <Copy size={15} />
          复制命令
        </button>
        <ul className="command-notes">
          <li>命令里的凭据 15 分钟内有效、只能用一次，并且只显示这一次；过期后重新生成。</li>
          <li>支持 x86_64 和 arm64 的 Linux（需要 systemd），会安装节点程序、注册并设为开机自启。</li>
          <li>以后在这台服务器上用不带凭据的同一命令再运行一次，就能更新节点程序。</li>
          <li>注册后到“服务端点”为它开服务，并在云安全组和防火墙放行端点的端口（TCP 和 UDP）。</li>
        </ul>
        <details className="manual">
          <summary>手动安装需要的信息</summary>
          <div className="secret">
            <Field label="节点标识">
              <input readOnly value={enroll.node_id} onFocus={(e) => e.target.select()} />
            </Field>
            <Field label="一次性引导凭据">
              <input readOnly value={enroll.bootstrap_secret} onFocus={(e) => e.target.select()} />
            </Field>
            <button
              className="secondary"
              onClick={() => copy(JSON.stringify(enroll, null, 2), "已复制注册信息")}
            >
              <Copy size={15} />
              复制注册信息
            </button>
          </div>
        </details>
      </div>
    </Modal>
  );
}
