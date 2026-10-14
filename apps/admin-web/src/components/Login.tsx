import { useState } from "react";
import { ChevronRight } from "lucide-react";
import { login, type Row } from "../api";

export function Login({
  onLogin,
  notice,
}: {
  onLogin: (u: Row) => void;
  /** Why the operator is back here, e.g. an expired session. */
  notice?: string;
}) {
  const [username, setName] = useState("admin");
  const [password, setPass] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  return (
    <div className="login">
      <div className="login-art">
        <div className="brand">
          <span className="brand-symbol">F</span>
          <span>
            FlowLight<strong>Accelerator</strong>
          </span>
        </div>
        <h1>
          每条线路，
          <br />
          清晰可控。
        </h1>
        <p>
          连接游戏、节点与玩家，
          <br />
          管理流光加速器的每一次连接。
        </p>
        <div className="route-graphic">
          <i />
          <span />
          <i />
          <span />
          <i />
        </div>
      </div>
      <form
        onSubmit={async (e) => {
          e.preventDefault();
          setBusy(true);
          setError("");
          try {
            onLogin(await login(username, password));
          } catch (e) {
            setError((e as Error).message);
          } finally {
            setBusy(false);
          }
        }}
      >
        <h2>登录管理后台</h2>
        <p>使用管理员或运维账号继续。</p>
        {notice && !error && (
          <p className="notice" role="status">
            {notice}
          </p>
        )}
        <label>
          账号
          <input
            autoComplete="username"
            required
            value={username}
            onChange={(e) => setName(e.target.value)}
          />
        </label>
        <label>
          密码
          <input
            autoComplete="current-password"
            type="password"
            required
            value={password}
            onChange={(e) => setPass(e.target.value)}
          />
        </label>
        {error && (
          <p className="alert" role="alert">
            {error}
          </p>
        )}
        <button className="primary" disabled={busy}>
          {busy ? "正在登录…" : "登录"}
          <ChevronRight size={16} />
        </button>
        <small>首次账号由控制服务启动配置创建。</small>
      </form>
    </div>
  );
}
