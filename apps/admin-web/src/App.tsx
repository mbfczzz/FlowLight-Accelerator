import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { CircleAlert, CircleCheck, Plus, RefreshCw, X } from "lucide-react";
import {
  api,
  clearAuth,
  currentUser,
  isSessionExpired,
  onSessionExpired,
  type Row,
} from "./api";
import { rowLock, type Confirmable } from "./actions";
import {
  isEditable,
  permissionsFor,
  sectionDescription,
  sectionIcon,
  sectionTitle,
} from "./sections";
import { messageOf, useSectionData } from "./useSectionData";
import { useNames } from "./useNames";
import { DetailModal } from "./components/DetailModal";
import { Editor } from "./components/Editor";
import { EnrollmentModal } from "./components/EnrollmentModal";
import { Login } from "./components/Login";
import { ConfirmDialog } from "./components/Modal";
import { Overview } from "./components/Overview";
import { ResourceTable } from "./components/ResourceTable";
import { RowActions } from "./components/RowActions";
import { Sidebar } from "./components/Sidebar";

const EXPIRED = "登录已过期，请重新登录";

/**
 * Session shell. Whatever request discovers that the session can no longer
 * be renewed, the transport reports it here and the app returns to login;
 * the workspace and every dialog in it unmount with the stale user.
 */
export function App() {
  const [user, setUser] = useState<Row | null>(currentUser() || null);
  const [section, setSection] = useState("overview");
  const [loginNotice, setLoginNotice] = useState("");
  // A layout effect is in place before any child's data effect starts a
  // request, so no expiry can be reported before someone is listening.
  useLayoutEffect(
    () =>
      onSessionExpired(() => {
        setUser(null);
        setLoginNotice(EXPIRED);
      }),
    [],
  );
  if (!user)
    return (
      <Login
        notice={loginNotice}
        onLogin={(u) => {
          setLoginNotice("");
          setUser(u);
        }}
      />
    );
  return (
    <Workspace
      user={user}
      section={section}
      onSection={setSection}
      onLoggedOut={() => {
        setLoginNotice("");
        setUser(null);
      }}
    />
  );
}

function Workspace({
  user,
  section,
  onSection,
  onLoggedOut,
}: {
  user: Row;
  section: string;
  onSection: (id: string) => void;
  onLoggedOut: () => void;
}) {
  const data = useSectionData(section);
  const { names, covers, reloadNames } = useNames(section);
  const [search, setSearch] = useState("");
  const [editing, setEditing] = useState<Row | null>(null);
  const [detail, setDetail] = useState<Row | null>(null);
  const [enroll, setEnroll] = useState<Row | null>(null);
  const [notice, setNotice] = useState("");
  const [actionError, setActionError] = useState("");
  const [confirm, setConfirm] = useState<Confirmable | null>(null);
  const [confirmError, setConfirmError] = useState("");
  /** Row lock → action currently in flight for that row. */
  const [pending, setPending] = useState<Record<string, string>>({});
  // State updates land after the render; these refs answer synchronously,
  // so a second click in the same frame cannot start a second request.
  const locks = useRef(new Set<string>());
  const openConfirm = useRef<Confirmable | null>(null);

  const permissions = permissionsFor(user);
  const editable = isEditable(section, user);
  const title = sectionTitle(section);
  const Icon = sectionIcon(section);
  const error = data.error || actionError;

  useEffect(() => {
    setSearch("");
    setActionError("");
  }, [section]);

  function askConfirm(spec: Confirmable) {
    if (locks.current.has(spec.lock)) return;
    openConfirm.current = spec;
    setConfirmError("");
    setConfirm(spec);
  }

  function closeConfirm() {
    openConfirm.current = null;
    setConfirm(null);
    setConfirmError("");
  }

  async function runConfirmed(spec: Confirmable) {
    if (locks.current.has(spec.lock)) return;
    locks.current.add(spec.lock);
    setPending((p) => ({ ...p, [spec.lock]: spec.action }));
    setConfirmError("");
    setActionError("");
    try {
      const result = await api(spec.path, "POST", spec.body);
      if (spec.action === "enroll") setEnroll(result);
      if (spec.action === "rollback")
        setDetail((d) =>
          d?.versions && rowLock("lines", d.line_id) === spec.lock ? null : d,
        );
      setNotice(spec.success);
      if (openConfirm.current === spec) closeConfirm();
      // Hold the row lock until the reload lands: an action such as drain
      // derives its request from the row, and the old row would repeat it.
      await data.reload();
    } catch (e) {
      if (isSessionExpired(e)) return;
      // Report where the operator is looking: in the dialog if it is still
      // open, otherwise on the page.
      if (openConfirm.current === spec) setConfirmError(messageOf(e));
      else setActionError(messageOf(e));
    } finally {
      locks.current.delete(spec.lock);
      setPending((p) => {
        const next = { ...p };
        delete next[spec.lock];
        return next;
      });
    }
  }

  async function showVersions(row: Row) {
    try {
      setDetail({
        title: "线路版本",
        versions: await api("/lines/" + row.id + "/versions"),
        line_id: row.id,
        line_name: row.name,
      });
    } catch (e) {
      if (!isSessionExpired(e)) setActionError(messageOf(e));
    }
  }

  function reload() {
    setActionError("");
    void data.reload();
  }

  async function logout() {
    await api("/auth/logout", "POST").catch(() => {});
    clearAuth();
    onLoggedOut();
  }

  return (
    <div className="shell">
      <Sidebar
        section={section}
        user={user}
        onSelect={(id) => {
          onSection(id);
          setNotice("");
        }}
        onLogout={logout}
      />
      <main>
        <section className="content">
          <div className="page-head">
            <div className="page-title">
              <span className="page-icon" aria-hidden="true">
                <Icon size={21} />
              </span>
              <div>
                <h1>{title}</h1>
                <p>{sectionDescription(section)}</p>
              </div>
            </div>
            <div className="actions">
              <button className="secondary" onClick={reload} disabled={data.loading}>
                <RefreshCw size={15} />
                刷新
              </button>
              {editable && (
                <button className="primary" onClick={() => setEditing({})}>
                  <Plus size={16} />
                  新增
                </button>
              )}
            </div>
          </div>
          {(error || notice) && (
            <div className="page-banners">
              {error && (
                <div className="alert" role="alert">
                  <CircleAlert size={16} />
                  <span>{error}</span>
                  <button onClick={reload} disabled={data.loading}>
                    重试
                  </button>
                </div>
              )}
              {notice && (
                <div className="notice" role="status">
                  <CircleCheck size={16} />
                  <span>{notice}</span>
                  <button aria-label="关闭提示" onClick={() => setNotice("")}>
                    <X size={15} />
                  </button>
                </div>
              )}
            </div>
          )}
          {section === "overview" ? (
            <Overview
              overview={data.overview}
              loading={data.loading}
              onOpen={(id) => {
                onSection(id);
                setNotice("");
              }}
            />
          ) : (
            <ResourceTable
              section={section}
              rows={data.rows}
              hasMore={data.hasMore}
              loading={data.loading}
              loadingMore={data.loadingMore}
              moreError={data.moreError}
              failed={!!data.error}
              onLoadMore={() => void data.loadMore()}
              search={search}
              onSearch={setSearch}
              names={names}
              covers={covers}
              onCreate={editable ? () => setEditing({}) : undefined}
              renderActions={(row) => (
                <RowActions
                  section={section}
                  row={row}
                  editable={editable}
                  permissions={permissions}
                  pending={pending}
                  onDetail={setDetail}
                  onEdit={setEditing}
                  onVersions={showVersions}
                  onConfirm={askConfirm}
                />
              )}
            />
          )}
        </section>
      </main>
      {editing && (
        <Editor
          kind={section}
          initial={editing}
          onClose={() => setEditing(null)}
          onSave={async (body) => {
            await api(
              "/" + section + (editing.id ? "/" + editing.id : ""),
              editing.id ? "PATCH" : "POST",
              body,
            );
            setEditing(null);
            void reloadNames();
            // Only game rules and lines go through a publication; every
            // other change applies as soon as it is saved.
            setNotice(
              section === "games"
                ? "已保存草稿。发布规则后，新会话才会使用它。"
                : section === "lines"
                  ? "已保存。发布线路并通过验证后，新会话才会使用它。"
                  : "已保存",
            );
            await data.reload();
          }}
        />
      )}
      {detail && (
        <DetailModal
          detail={detail}
          canPublish={permissions.canPublish}
          pending={pending}
          onConfirm={askConfirm}
          onClose={() => setDetail(null)}
          names={names}
        />
      )}
      {enroll && (
        <EnrollmentModal
          enroll={enroll}
          onCopied={() => setNotice("已复制注册信息")}
          onClose={() => setEnroll(null)}
        />
      )}
      {confirm && (
        <ConfirmDialog
          spec={confirm}
          running={!!pending[confirm.lock]}
          error={confirmError}
          onConfirm={() => void runConfirmed(confirm)}
          onClose={closeConfirm}
        />
      )}
    </div>
  );
}
