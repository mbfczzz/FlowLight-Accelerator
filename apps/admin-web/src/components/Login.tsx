import { useState } from "react";
import { ChevronRight, Route, ShieldCheck, Zap } from "lucide-react";
import { login, type Row } from "../api";
import { BrandMark } from "./BrandMark";

/** Light streaks across the brand panel, the same motif as the client. */
function Streaks() {
  const streaks = [
    [8, 18, 46, 0.55],
    [30, 34, 60, 0.35],
    [-6, 52, 52, 0.45],
    [42, 66, 44, 0.6],
    [12, 82, 58, 0.3],
  ];
  return (
    <svg className="streaks" viewBox="0 0 100 100" preserveAspectRatio="none" aria-hidden="true">
      <defs>
        <linearGradient id="login-streak" x1="0" y1="0" x2="1" y2="0">
          <stop offset="0" stopColor="#f7cb72" stopOpacity="0" />
          <stop offset="0.75" stopColor="#f7cb72" stopOpacity="1" />
          <stop offset="1" stopColor="#f7cb72" stopOpacity="0" />
        </linearGradient>
      </defs>
      <g transform="rotate(-18 50 50)">
        {streaks.map(([x, y, w, o], i) => (
          <rect key={i} x={x} y={y} width={w} height={i % 2 ? 0.35 : 0.6} rx="0.3" fill="url(#login-streak)" opacity={o} />
        ))}
      </g>
    </svg>
  );
}

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
        <Streaks />
        <div className="brand">
          <BrandMark size={40} />
          <span className="brand-name">
            流光加速器
            <small>FLOWLIGHT ADMIN</small>
          </span>
        </div>
        <div>
          <h1>
            每条线路
            <br />
            都看得见、管得住
          </h1>
          <p>配置游戏、节点和线路，开通玩家，查看每一次加速。</p>
        </div>
        <ul className="login-points">
          <li>
            <span>
              <Route size={16} />
            </span>
            线路发布前自动探测，失败不影响已上线版本
          </li>
          <li>
            <span>
              <Zap size={16} />
            </span>
            实时查看会话、节点心跳和部署任务
          </li>
          <li>
            <span>
              <ShieldCheck size={16} />
            </span>
            按角色分配权限，每次修改都有审计记录
          </li>
        </ul>
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
        <p>使用管理员或运维账号登录。</p>
        {notice && !error && (
          <div className="notice" role="status">
            <span>{notice}</span>
          </div>
        )}
        <label className="field">
          <span>账号</span>
          <input autoComplete="username" required value={username} onChange={(e) => setName(e.target.value)} />
        </label>
        <label className="field">
          <span>密码</span>
          <input
            autoComplete="current-password"
            type="password"
            required
            value={password}
            onChange={(e) => setPass(e.target.value)}
          />
        </label>
        {error && (
          <div className="alert" role="alert">
            <span>{error}</span>
          </div>
        )}
        <button className="primary" disabled={busy}>
          {busy ? "正在登录…" : "登录"}
          {!busy && <ChevronRight size={16} />}
        </button>
        <small>忘记密码请联系管理员在“用户与权限”中重置。</small>
      </form>
    </div>
  );
}
