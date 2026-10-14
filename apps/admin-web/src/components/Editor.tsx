import { useEffect, useState } from "react";
import { ArrowUp, X } from "lucide-react";
import {
  isSessionExpired,
  listPage,
  MAX_PAGE_SIZE,
  type Row,
} from "../api";
import { stateNames } from "../format";
import { defaults, sectionTitle } from "../sections";
import { Modal } from "./Modal";

/** Lists the editor offers as choices for node, endpoint and user fields. */
const referenceKinds = ["nodes", "endpoints", "games", "users"];

export function Editor({
  kind,
  initial,
  onClose,
  onSave,
}: {
  kind: string;
  initial: Row;
  onClose: () => void;
  onSave: (r: Row) => Promise<void>;
}) {
  const [d, set] = useState<Row>(() => ({
    ...defaults(kind),
    ...structuredClone(initial),
  }));
  const [refs, setRefs] = useState<Record<string, Row[]>>({});
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    let alive = true;
    // One request at the largest page size per list: these only fill select
    // options, and a failed list leaves its select empty rather than
    // blocking the form.
    Promise.all(
      referenceKinds.map(
        async (k) =>
          [k, await listPage(k, MAX_PAGE_SIZE, 0).catch(() => [])] as const,
      ),
    ).then((p) => {
      if (alive) setRefs(Object.fromEntries(p));
    });
    return () => {
      alive = false;
    };
  }, []);
  function update(k: string, v: any) {
    set((prev) => {
      const next = structuredClone(prev);
      const bits = k.split(".");
      if (bits.length === 2) {
        next[bits[0]] ??= {};
        next[bits[0]][bits[1]] = v;
      } else next[k] = v;
      return next;
    });
  }
  function val(k: string): any {
    return k.split(".").reduce<any>((o, p) => o?.[p], d);
  }
  function field(
    k: string,
    label: string,
    type = "text",
    choices?: [string, string][],
  ) {
    return (
      <label key={k}>
        {label}
        {type === "check" ? (
          <input
            type="checkbox"
            checked={!!val(k)}
            onChange={(e) => update(k, e.target.checked)}
          />
        ) : type === "list" ? (
          <textarea
            value={(val(k) || []).join("\n")}
            onChange={(e) =>
              update(
                k,
                e.target.value
                  .split("\n")
                  .map((x) => x.trim())
                  .filter(Boolean),
              )
            }
            placeholder="每行一个值"
          />
        ) : choices ? (
          <select
            value={val(k) || ""}
            onChange={(e) => update(k, e.target.value)}
          >
            <option value="">请选择</option>
            {withCurrent(choices, val(k)).map(([v, n]) => (
              <option value={v} key={v}>
                {n}
              </option>
            ))}
          </select>
        ) : (
          <input
            type={type}
            value={val(k) ?? ""}
            onChange={(e) =>
              update(
                k,
                type === "number" ? Number(e.target.value) : e.target.value,
              )
            }
            autoComplete={type === "password" ? "new-password" : "off"}
          />
        )}
      </label>
    );
  }
  const options = (key: string) =>
    refs[key]?.map(
      (r) => [r.id, r.name || r.username || r.id] as [string, string],
    ) || [];
  return (
    <Modal
      title={
        (initial.id ? "编辑" : "新增") +
        (sectionTitle(kind) || "资源")
      }
      close={onClose}
    >
      <form
        onSubmit={async (e) => {
          e.preventDefault();
          setBusy(true);
          setError("");
          try {
            const body = structuredClone(d);
            delete body.id;
            delete body.created_at;
            delete body.updated_at;
            if (kind === "users") {
              delete body.password_hash;
            }
            await onSave(body);
          } catch (e) {
            if (!isSessionExpired(e)) setError((e as Error).message);
          } finally {
            setBusy(false);
          }
        }}
      >
        <div className="fields">
          {["games", "nodes", "endpoints", "lines"].includes(kind) &&
            field("name", "名称")}
          {kind === "games" && (
            <>
              {field("regions", "区服标识，例如 kr、jp", "list")}
              {field("draft.processes", "游戏进程名称", "list")}
              {field("draft.paths", "完整程序路径（可选）", "list")}
              {field("draft.domains", "游戏域名（可选）", "list")}
              {field("draft.cidrs", "目标网段（可选）", "list")}
              {field("draft.exclude_domains", "排除域名（可选）", "list")}
              {field("draft.udp", "需要 UDP", "check")}
              {field("enabled", "启用", "check")}
              <p className="hint">
                内置游戏规则是初始草稿，发布前请按实际安装和区服核对。
              </p>
            </>
          )}
          {kind === "nodes" && (
            <>
              {field("region", "节点地区")}
              {field("address", "节点地址")}
              {field("enabled", "启用节点", "check")}
              {["tcp", "udp", "ipv4", "ipv6", "managed"].map((k) =>
                field(
                  "capabilities." + k,
                  k === "managed" ? "支持托管进程" : k.toUpperCase(),
                  "check",
                ),
              )}
            </>
          )}
          {kind === "endpoints" && (
            <>
              {field("node_id", "所属节点", "text", options("nodes"))}
              {field("kind", "服务类型", "text", [
                ["ss2022", "SS2022 落地"],
                ["realm", "Realm 透明中转"],
              ])}
              {field("host", "客户端／上游可达地址")}
              {field("port", "起始端口", "number")}
              {field("port_end", "结束端口", "number")}
              {field("cipher", "加密方法", "text", [
                ["2022-blake3-aes-128-gcm", "SS2022 AES-128"],
                ["2022-blake3-aes-256-gcm", "SS2022 AES-256"],
              ])}
              {field(
                "password",
                "导入线路密钥（仅新增或更换时填写）",
                "password",
              )}
              {["tcp", "udp", "ipv4", "ipv6", "managed"].map((k) =>
                field(
                  "capabilities." + k,
                  k === "managed" ? "平台管理此端点" : k.toUpperCase(),
                  "check",
                ),
              )}
              {field("enabled", "启用", "check")}
            </>
          )}
          {kind === "lines" && (
            <>
              {field("regions", "可用区服", "list")}
              {field("mode", "运行模式", "text", [
                ["managed", "平台托管"],
                ["imported", "导入现成线路"],
                ["hybrid", "平台入口＋自带落地"],
              ])}
              {field("capacity", "并发容量", "number")}
              {field("priority", "优先级", "number")}
              {field("expected_exit", "预期出口 IP（可选）")}
              <div className="path-editor">
                <span>线路顺序 · 入口 → 中转 → 落地</span>
                {(d.endpoint_ids || []).map((id: string, i: number) => (
                  <div key={i}>
                    <b>{i + 1}</b>
                    <select
                      value={id}
                      onChange={(e) => {
                        const a = [...d.endpoint_ids];
                        a[i] = e.target.value;
                        update("endpoint_ids", a);
                      }}
                    >
                      <option value="">选择服务端点</option>
                      {withCurrent(options("endpoints"), id).map(([v, n]) => (
                        <option key={v} value={v}>
                          {n}
                        </option>
                      ))}
                    </select>
                    <button
                      type="button"
                      aria-label="上移"
                      disabled={!i}
                      onClick={() => {
                        const a = [...d.endpoint_ids];
                        [a[i - 1], a[i]] = [a[i], a[i - 1]];
                        update("endpoint_ids", a);
                      }}
                    >
                      <ArrowUp size={14} />
                    </button>
                    <button
                      type="button"
                      aria-label="删除此跳"
                      onClick={() =>
                        update(
                          "endpoint_ids",
                          d.endpoint_ids.filter(
                            (_: string, j: number) => j !== i,
                          ),
                        )
                      }
                    >
                      <X size={14} />
                    </button>
                  </div>
                ))}
                <button
                  type="button"
                  onClick={() =>
                    update("endpoint_ids", [...(d.endpoint_ids || []), ""])
                  }
                >
                  添加端点
                </button>
              </div>
              {field("game_ids", "限定游戏（留空允许全部游戏）", "list")}
              {field("user_ids", "限定用户（留空允许已授权账号）", "list")}
              <p className="hint">
                发布会创建隔离的验证会话。探测失败时保留之前的已发布版本。
              </p>
            </>
          )}
          {kind === "bindings" && (
            <>
              {field("user_id", "用户", "text", options("users"))}
              {field("endpoint_id", "落地端点", "text", options("endpoints"))}
              {field("kind", "来源", "text", [
                ["shared", "共享"],
                ["dedicated", "独享"],
                ["byo", "自带落地"],
              ])}
              {field("expected_exit", "预期出口 IP")}
              {field("fixed_ip", "要求固定出口", "check")}
              {field("enabled", "启用", "check")}
              <p className="hint">
                关联线路通过实际出口探测后，绑定才会被标记为已验证。
              </p>
            </>
          )}
          {kind === "users" && (
            <>
              {field("username", "账号")}
              {field(
                "password",
                initial.id ? "新密码（留空保留）" : "密码（14–72 字节）",
                "password",
              )}
              {field(
                "role",
                "权限",
                "text",
                Object.entries(stateNames).filter(([k]) =>
                  [
                    "admin",
                    "operator",
                    "publisher",
                    "viewer",
                    "player",
                  ].includes(k),
                ),
              )}
              {field("max_sessions", "并发会话上限", "number")}
              {field("enabled", "启用", "check")}
            </>
          )}
          {kind === "releases" && (
            <>
              {field("version", "版本号")}
              {field("channel", "发布渠道", "text", [
                ["stable", "稳定版"],
                ["beta", "测试版"],
              ])}
              {field("url", "HTTPS 下载地址")}
              {field("sha256", "安装包 SHA256")}
              {field("signature", "可信发布清单签名")}
              {field("required", "必须更新", "check")}
              {field("published", "发布此版本", "check")}
            </>
          )}
        </div>
        {error && (
          <p className="alert" role="alert">
            {error}
          </p>
        )}
        <div className="form-actions">
          <button type="button" className="secondary" onClick={onClose}>
            取消
          </button>
          <button className="primary" disabled={busy}>
            {busy ? "正在保存…" : "保存"}
          </button>
        </div>
      </form>
    </Modal>
  );
}

/**
 * Keep the saved value selectable when it is not among the loaded choices,
 * e.g. a reference beyond the first page. Without it the select would show
 * "请选择" while still holding the old id.
 */
function withCurrent(
  choices: [string, string][],
  current: unknown,
): [string, string][] {
  if (typeof current !== "string" || current === "") return choices;
  if (choices.some(([v]) => v === current)) return choices;
  return [[current, current], ...choices];
}
