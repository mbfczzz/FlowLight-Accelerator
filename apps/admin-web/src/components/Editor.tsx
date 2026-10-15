import { useEffect, useRef, useState, type ReactNode } from "react";
import {
  ArrowDown,
  ArrowUp,
  Cable,
  Gamepad2,
  Image as ImageIcon,
  KeyRound,
  ListChecks,
  MapPin,
  Package,
  Plus,
  Route,
  Server,
  ShieldCheck,
  Trash2,
  UserRound,
  Waypoints,
} from "lucide-react";
import {
  isSessionExpired,
  listPage,
  MAX_PAGE_SIZE,
  type Row,
} from "../api";
import { roleNote } from "../cells";
import { defaults, sectionTitle } from "../sections";
import { passwordLabel, validate } from "../validate";
import { CoverPicker, type CoverChange } from "./CoverPicker";
import { Avatar, Cover, RegionName, commonRegions } from "../visuals";
import {
  Field,
  PickList,
  type PickOption,
  RadioCards,
  Segmented,
  SwitchRow,
  TagInput,
  Toggles,
} from "./controls";
import { Modal } from "./Modal";

/** Lists the editor offers as choices for node, endpoint, game and user fields. */
const referenceKinds = ["nodes", "endpoints", "games", "users"];

const capabilityOptions: [string, string][] = [
  ["tcp", "TCP"],
  ["udp", "UDP"],
  ["ipv4", "IPv4"],
  ["ipv6", "IPv6"],
];

function Section({
  icon,
  title,
  note,
  children,
}: {
  icon: ReactNode;
  title: string;
  note?: string;
  children: ReactNode;
}) {
  return (
    <section className="form-section">
      <h3>
        {icon}
        {title}
      </h3>
      {note && <p>{note}</p>}
      <div className="fields">{children}</div>
    </section>
  );
}

// Chosen regions are coloured by position as in the lists; suggestions are grey.
const regionDisplay = (c: string, slot: number) => <RegionName code={c} slot={slot} />;

export function Editor({
  kind,
  initial,
  onClose,
  onSave,
}: {
  kind: string;
  initial: Row;
  onClose: () => void;
  /** Saves the row, then applies a cover change (games only). */
  onSave: (r: Row, cover: CoverChange | null) => Promise<void>;
}) {
  const [cover, setCover] = useState<CoverChange | null>(null);
  const [d, set] = useState<Row>(() => ({
    ...defaults(kind),
    ...structuredClone(initial),
  }));
  const [refs, setRefs] = useState<Record<string, Row[]>>({});
  const [error, setError] = useState("");
  // The field the current error belongs to; "" for errors from the service.
  const [errorField, setErrorField] = useState("");
  const form = useRef<HTMLFormElement>(null);
  // Bring the field at fault into view and put the cursor in it.
  useEffect(() => {
    if (!errorField) return;
    const at = form.current?.querySelector<HTMLElement>('[data-field="' + errorField + '"]');
    // A field this form does not show: report the error in the footer.
    if (!at) return setErrorField("");
    at?.scrollIntoView({ block: "center", behavior: "smooth" });
    at?.querySelector<HTMLElement>("input, select, textarea, button")?.focus({ preventScroll: true });
  }, [errorField, error]);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    let alive = true;
    // One request at the largest page size per list: these only fill
    // choices, and a failed list leaves its choices empty rather than
    // blocking the form.
    Promise.all(
      referenceKinds.map(
        async (k) =>
          [
            k,
            await listPage(k, MAX_PAGE_SIZE, 0).catch((e) => {
              if (isSessionExpired(e)) throw e;
              return [];
            }),
          ] as const,
      ),
    )
      .then((p) => {
        if (alive) setRefs(Object.fromEntries(p));
      })
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, []);

  function update(k: string, v: any) {
    setError("");
    setErrorField("");
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
  const val = (k: string): any => k.split(".").reduce<any>((o, p) => o?.[p], d);
  const str = (k: string) => (val(k) ?? "") as string;
  const list = (k: string) => (val(k) || []) as string[];
  const err = (k: string) => (errorField === k ? error : undefined);
  const options = (key: string, filter?: (r: Row) => boolean): [string, string][] =>
    (refs[key] || [])
      .filter((r) => !filter || filter(r))
      .map((r) => [r.id, r.name || r.username || r.id]);

  const text = (k: string, label: string, opts: { note?: ReactNode; placeholder?: string; wide?: boolean; type?: string } = {}) => (
    <Field label={label} note={opts.note} wide={opts.wide} name={k} error={err(k)}>
      <input
        aria-invalid={!!err(k) || undefined}
        type={opts.type || "text"}
        value={str(k)}
        placeholder={opts.placeholder}
        autoComplete={opts.type === "password" ? "new-password" : "off"}
        onChange={(e) => update(k, e.target.value)}
      />
    </Field>
  );
  const number = (k: string, label: string, note?: ReactNode) => (
    <Field label={label} note={note} name={k} error={err(k)}>
      <input
        aria-invalid={!!err(k) || undefined}
        type="number"
        inputMode="numeric"
        value={val(k) ?? ""}
        onChange={(e) => update(k, e.target.value === "" ? "" : Number(e.target.value))}
      />
    </Field>
  );
  const pick = (k: string, label: string, choices: [string, string][], placeholder: string, note?: ReactNode) => {
    const current = str(k);
    const all = current && !choices.some(([v]) => v === current) ? [[current, current] as [string, string], ...choices] : choices;
    return (
      <Field label={label} note={note} name={k} error={err(k)}>
        <select aria-invalid={!!err(k) || undefined} value={current} onChange={(e) => update(k, e.target.value)}>
          <option value="">{placeholder}</option>
          {all.map(([v, n]) => (
            <option key={v} value={v}>
              {n}
            </option>
          ))}
        </select>
      </Field>
    );
  };
  const toggle = (k: string, label: string, note?: string) => (
    <SwitchRow label={label} note={note} checked={!!val(k)} onChange={(v) => update(k, v)} />
  );
  const capabilities = (label = "支持的协议") => (
    <Field group label={label} wide>
      <Toggles
        label={label}
        options={capabilityOptions}
        isOn={(k) => !!val("capabilities." + k)}
        onToggle={(k) => update("capabilities." + k, !val("capabilities." + k))}
      />
    </Field>
  );

  const gameOptions: PickOption[] = (refs.games || []).map((g) => ({
    value: g.id,
    label: g.name || g.id,
    lead: <Cover seed={g.id} name={g.name} index={g.cover_index} image={g.cover_image} width={22} height={13} />,
  }));
  const playerOptions: PickOption[] = (refs.users || [])
    .filter((u) => u.role === "player")
    .map((u) => ({ value: u.id, label: u.username, lead: <Avatar name={u.username} small /> }));

  const body = (() => {
    switch (kind) {
      case "games":
        return (
          <>
            <Section icon={<Gamepad2 size={16} />} title="基本信息">
              {text("name", "游戏名称", { placeholder: "例如 VALORANT", wide: true })}
              {toggle("enabled", "在客户端显示", "停用后客户端不再列出这款游戏。")}
            </Section>
            <Section icon={<ImageIcon size={16} />} title="封面" note="客户端游戏库和游戏页显示这张图；不上传则使用自动生成的封面。">
              <div className="field wide">
                <CoverPicker
                  seed={initial.id || "new-game"}
                  name={str("name")}
                  index={initial.cover_index}
                  current={initial.cover_image || undefined}
                  change={cover}
                  onChange={(c) => {
                    setError("");
                    setErrorField("");
                    setCover(c);
                  }}
                />
              </div>
            </Section>
            <Section icon={<MapPin size={16} />} title="区服" note="客户端按这里的顺序列出区服，常见代码会显示中文名。">
              <Field group label="区服代码" wide name="regions" error={err("regions")}>
                <TagInput
                  label="区服代码"
                  value={list("regions")}
                  onChange={(v) => update("regions", v)}
                  placeholder="输入代码后按回车，例如 kr"
                  suggestions={commonRegions}
                  display={regionDisplay}
                  normalize={(s) => s.toLowerCase()}
                />
              </Field>
            </Section>
            <Section icon={<ListChecks size={16} />} title="分流规则" note="匹配到的流量走加速线路，其余流量直连。进程名和完整路径至少填一项。">
              <Field group label="游戏进程名" wide note="按回车添加，可一次粘贴多行。" name="draft.processes" error={err("draft.processes")}>
                <TagInput label="游戏进程名" value={list("draft.processes")} onChange={(v) => update("draft.processes", v)} placeholder="例如 VALORANT-Win64-Shipping.exe" />
              </Field>
              <Field group label="完整程序路径（可选）" wide>
                <TagInput label="完整程序路径" value={list("draft.paths")} onChange={(v) => update("draft.paths", v)} placeholder="例如 C:\Riot Games\VALORANT\live\VALORANT.exe" />
              </Field>
              <Field group label="游戏域名（可选）">
                <TagInput label="游戏域名" value={list("draft.domains")} onChange={(v) => update("draft.domains", v)} placeholder="例如 riotgames.com" />
              </Field>
              <Field group label="目标网段（可选）">
                <TagInput label="目标网段" value={list("draft.cidrs")} onChange={(v) => update("draft.cidrs", v)} placeholder="例如 203.0.113.0/24" />
              </Field>
              <Field group label="排除域名（可选）" wide note="这些域名即使匹配上面的规则也直连，例如下载更新的 CDN。">
                <TagInput label="排除域名" value={list("draft.exclude_domains")} onChange={(v) => update("draft.exclude_domains", v)} placeholder="例如 cdn.example.com" />
              </Field>
              {toggle("draft.udp", "需要 UDP", "对局数据走 UDP 的游戏需要开启，线路的每一跳都必须支持 UDP。")}
            </Section>
            <p className="hint">内置游戏是初始草稿，发布前请按实际安装路径和区服核对。保存只更新草稿，点“发布规则”后客户端才会使用。</p>
          </>
        );
      case "lines": {
        const hops = list("endpoint_ids");
        const endpointChoices = options("endpoints");
        const hopName = (i: number) =>
          hops.length === 1 ? "入口·落地" : i === 0 ? "入口" : i === hops.length - 1 ? "落地" : "中转";
        const move = (i: number, j: number) => {
          const a = [...hops];
          [a[i], a[j]] = [a[j], a[i]];
          update("endpoint_ids", a);
        };
        return (
          <>
            <Section icon={<Route size={16} />} title="基本信息">
              {text("name", "线路名称", { placeholder: "例如 KR 直连-01", wide: true })}
              <Field group label="运行模式" wide>
                <RadioCards
                  label="运行模式"
                  value={str("mode")}
                  onChange={(v) => update("mode", v)}
                  options={[
                    ["managed", "平台托管", "节点由平台按会话开通和回收"],
                    ["imported", "导入现成线路", "使用已有的 SS2022 服务"],
                    ["hybrid", "平台入口＋自带落地", "入口由平台托管，落地自备"],
                  ]}
                />
              </Field>
              {number("capacity", "并发容量", "同时在这条线路上加速的玩家上限，1–10000。")}
              {number("priority", "优先级", "仅作记录；客户端按延迟排序线路。")}
            </Section>
            <Section icon={<MapPin size={16} />} title="覆盖范围">
              <Field group label="可用区服" wide name="regions" error={err("regions")}>
                <TagInput label="可用区服" value={list("regions")} onChange={(v) => update("regions", v)} placeholder="输入代码后按回车" suggestions={commonRegions} display={regionDisplay} normalize={(s) => s.toLowerCase()} />
              </Field>
              <Field group label="限定游戏" wide>
                <PickList label="添加限定游戏" value={list("game_ids")} onChange={(v) => update("game_ids", v)} options={gameOptions} empty="不限，所有游戏都可使用" />
              </Field>
              <Field group label="限定用户" wide>
                <PickList label="添加限定用户" value={list("user_ids")} onChange={(v) => update("user_ids", v)} options={playerOptions} empty="不限，所有玩家都可使用" />
              </Field>
            </Section>
            <Section icon={<Cable size={16} />} title="线路路径" note="按顺序选择服务端点：第一跳是玩家连接的入口，最后一跳是落地，中间是 Realm 中转。最多 5 跳。">
              <div className={"hops field wide" + (err("endpoint_ids") ? " invalid" : "")} role="group" aria-label="线路路径" data-field="endpoint_ids">
                {hops.map((id, i) => (
                  <div className="hop" key={i}>
                    <span className="hop-label tone-neutral">{hopName(i)}</span>
                    <select aria-label={"第 " + (i + 1) + " 跳"} value={id} onChange={(e) => {
                      const a = [...hops];
                      a[i] = e.target.value;
                      update("endpoint_ids", a);
                    }}>
                      <option value="">选择服务端点</option>
                      {(id && !endpointChoices.some(([v]) => v === id) ? [[id, id] as [string, string], ...endpointChoices] : endpointChoices).map(([v, n]) => (
                        <option key={v} value={v}>{n}</option>
                      ))}
                    </select>
                    <span className="hop-tools">
                      <button type="button" className="icon-btn" aria-label="上移" disabled={!i} onClick={() => move(i, i - 1)}><ArrowUp size={15} /></button>
                      <button type="button" className="icon-btn" aria-label="下移" disabled={i === hops.length - 1} onClick={() => move(i, i + 1)}><ArrowDown size={15} /></button>
                      <button type="button" className="icon-btn" aria-label="删除这一跳" onClick={() => update("endpoint_ids", hops.filter((_, j) => j !== i))}><Trash2 size={15} /></button>
                    </span>
                  </div>
                ))}
                {hops.length < 5 && (
                  <button type="button" className="ghost sm" onClick={() => update("endpoint_ids", [...hops, ""])}>
                    <Plus size={14} />
                    添加一跳
                  </button>
                )}
                {err("endpoint_ids") && (
                  <small className="field-error" role="alert">
                    {err("endpoint_ids")}
                  </small>
                )}
              </div>
              {text("expected_exit", "预期出口 IP（可选）", { placeholder: "例如 203.0.113.7", wide: true, note: "填写后，探测到的出口必须与它一致，线路才算可用。" })}
            </Section>
            <p className="hint">保存不会影响正在使用的版本。点“发布”会创建隔离的验证会话探测整条路径，探测失败时保留之前的已发布版本。</p>
          </>
        );
      }
      case "nodes":
        return (
          <>
            <Section icon={<Server size={16} />} title="基本信息">
              {text("name", "服务器名称", { placeholder: "例如 首尔-01" })}
              {text("region", "所在地区", { placeholder: "例如 kr" })}
              {text("address", "服务器地址", { placeholder: "公网 IP 或域名", wide: true, note: "节点 Agent 注册后，控制服务用它识别这台机器。" })}
              {toggle("enabled", "启用", "停用后不再向这台服务器下发新任务。")}
            </Section>
            <Section icon={<ShieldCheck size={16} />} title="能力" note="服务器注册后会上报实际能力，平台取两者都支持的部分。">
              {capabilities()}
              {toggle("capabilities.managed", "支持托管服务", "平台可在这台服务器上按会话启动和回收端点的服务进程。")}
            </Section>
            <p className="hint">节点服务器本身不转发流量。保存后到“服务端点”里为它添加 Realm 中转或 SS2022 落地，同一台服务器可以同时开多个。</p>
          </>
        );
      case "endpoints": {
        const isExit = str("kind") === "ss2022";
        return (
          <>
            <Section icon={<Cable size={16} />} title="基本信息">
              {text("name", "端点名称", { placeholder: "例如 首尔落地-A" })}
              {pick("node_id", "所在服务器", options("nodes"), "选择节点服务器")}
              <Field group label="服务类型" wide name="kind" error={err("kind")}>
                <RadioCards
                  label="服务类型"
                  value={str("kind")}
                  onChange={(v) => update("kind", v)}
                  options={[
                    ["ss2022", "SS2022 落地", "线路的最后一跳，流量从这里出去"],
                    ["realm", "Realm 中转", "透明转发，用作入口或中转"],
                  ]}
                />
              </Field>
              {toggle("enabled", "启用", "停用后，使用它的线路无法再发布。")}
            </Section>
            <Section icon={<Waypoints size={16} />} title="地址与端口">
              {text("host", "客户端或上一跳可达的地址", { placeholder: "公网 IP 或域名", wide: true })}
              {number("port", "起始端口", "1–65535；平台托管时不低于 1024。")}
              {number("port_end", "结束端口", "托管端点从这个范围里为每个会话分配端口。")}
            </Section>
            {isExit && (
              <Section icon={<KeyRound size={16} />} title="加密">
                <Field group label="加密方法" wide>
                  <Segmented
                    label="加密方法"
                    value={str("cipher")}
                    onChange={(v) => update("cipher", v)}
                    options={[
                      ["2022-blake3-aes-128-gcm", "AES-128-GCM"],
                      ["2022-blake3-aes-256-gcm", "AES-256-GCM"],
                    ]}
                  />
                </Field>
                {text("password", "导入线路密钥（可选）", {
                  type: "password",
                  wide: true,
                  note: "只在导入现成线路或更换密钥时填写；AES-128 为 16 字节、AES-256 为 32 字节的 Base64。留空保留原密钥。",
                })}
              </Section>
            )}
            <Section icon={<ShieldCheck size={16} />} title="能力">
              {capabilities()}
              {toggle("capabilities.managed", "由平台管理", "开启后平台会在所在服务器上为每个会话开通端口；导入的端点请关闭。")}
            </Section>
          </>
        );
      }
      case "bindings":
        return (
          <Section icon={<Waypoints size={16} />} title="分配落地">
            {pick("user_id", "用户", options("users"), "选择用户")}
            {pick("endpoint_id", "落地端点", options("endpoints", (e) => e.kind === "ss2022"), "选择落地端点")}
            <Field group label="来源" wide name="kind" error={err("kind")}>
              <RadioCards
                label="来源"
                value={str("kind")}
                onChange={(v) => update("kind", v)}
                options={[
                  ["shared", "共享", "多个用户共用同一落地"],
                  ["dedicated", "独享", "只分配给这一个用户"],
                  ["byo", "自带落地", "用户自己提供的落地"],
                ]}
              />
            </Field>
            {text("expected_exit", "预期出口 IP", { placeholder: "例如 203.0.113.7" })}
            {toggle("fixed_ip", "要求固定出口", "开启后，出口与预期不一致的会话会被判定为失败。")}
            {toggle("enabled", "启用")}
            <p className="hint field wide">关联线路通过实际出口探测后，绑定才会标记为已验证。</p>
          </Section>
        );
      case "users":
        return (
          <>
            <Section icon={<UserRound size={16} />} title="账号">
              {text("username", "账号", { placeholder: "登录用的名字，不能有空格" })}
              {text("password", passwordLabel(d.role, !!initial.id), { type: "password" })}
              {toggle("enabled", "启用", "停用后无法登录，已登录的设备也会失效。")}
            </Section>
            <Section icon={<ShieldCheck size={16} />} title="权限">
              <Field group label="角色" wide name="role" error={err("role")}>
                <RadioCards
                  label="角色"
                  value={str("role")}
                  onChange={(v) => update("role", v)}
                  options={(["player", "viewer", "operator", "publisher", "admin"] as const).map(
                    (r): [string, string, string] => [
                      r,
                      { player: "玩家", viewer: "只读运维", operator: "配置运维", publisher: "发布运维", admin: "管理员" }[r],
                      roleNote[r],
                    ],
                  )}
                />
              </Field>
              {number("max_sessions", "并发会话上限", "同一账号最多同时加速的设备数，1–20。")}
            </Section>
            <p className="hint">玩家密码至少 8 位，管理和运维账号至少 14 位。修改密码后，该账号已登录的设备需要重新登录。</p>
          </>
        );
      default:
        return (
          <Section icon={<Package size={16} />} title="版本信息">
            {text("version", "版本号", { placeholder: "例如 0.1.5" })}
            <Field group label="发布渠道">
              <Segmented label="发布渠道" value={str("channel")} onChange={(v) => update("channel", v)} options={[["stable", "稳定版"], ["beta", "测试版"]]} />
            </Field>
            {text("url", "下载地址", {
              placeholder: "https://…/FlowLight-Accelerator-Setup.exe",
              wide: true,
              note: "客户端从这里下载安装包。GitHub Release 页面里安装包的链接即可，也可以放到自己的服务器或 CDN 上。",
            })}
            {text("sha256", "安装包 SHA256", {
              placeholder: "64 位十六进制",
              wide: true,
              note: "客户端下载后核对，不一致就不会安装。GitHub Release 的说明里列有每个安装包的 SHA256。",
            })}
            <Field label="发布清单签名（可选）" wide name="signature" error={err("signature")} note="暂不校验，留作记录。">
              <textarea aria-invalid={!!err("signature") || undefined} value={str("signature")} onChange={(e) => update("signature", e.target.value)} />
            </Field>
            {toggle("required", "必须更新", "低于此版本的客户端会被要求更新，提示不能关闭。")}
            {toggle("published", "发布此版本", "开启后，客户端启动时和每隔几小时会检查到它并提示更新。")}
          </Section>
        );
    }
  })();

  return (
    <Modal
      drawer
      bare
      title={(initial.id ? "编辑" : "新增") + (sectionTitle(kind) || "资源")}
      subtitle={initial.id ? d.name || d.username || d.version || undefined : undefined}
      close={onClose}
    >
      <form
        ref={form}
        noValidate
        onSubmit={async (e) => {
          e.preventDefault();
          const body = structuredClone(d);
          delete body.id;
          delete body.created_at;
          delete body.updated_at;
          if (kind === "users") {
            delete body.password_hash;
            body.username = String(body.username || "").trim();
          }
          const problem = validate(kind, body, initial);
          if (problem) {
            setError(problem.message);
            setErrorField(problem.field);
            return;
          }
          setBusy(true);
          setError("");
          setErrorField("");
          try {
            await onSave(body, kind === "games" ? cover : null);
          } catch (e) {
            if (!isSessionExpired(e)) setError((e as Error).message);
          } finally {
            setBusy(false);
          }
        }}
      >
        <div className="modal-body">{body}</div>
        <div className="drawer-foot">
          {error && !errorField && (
            <div className="alert" role="alert">
              <span>{error}</span>
            </div>
          )}
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
