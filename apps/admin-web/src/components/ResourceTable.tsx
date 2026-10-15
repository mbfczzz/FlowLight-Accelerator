import type { ReactNode } from "react";
import { CircleAlert, Inbox, LoaderCircle, Plus, Search } from "lucide-react";
import type { Row } from "../api";
import { renderCell } from "../cells";
import { labels, text } from "../format";
import { columns } from "../sections";

/**
 * Count shown beside the search box. It only claims a total once the last
 * page has arrived; until then it says how many rows are loaded.
 */
function countLabel(loaded: number, matched: number, searching: boolean, hasMore: boolean) {
  if (!searching)
    return hasMore ? "已加载 " + loaded + " 条，还有更多" : "共 " + loaded + " 条";
  return hasMore
    ? "已加载的 " + loaded + " 条中匹配 " + matched + " 条"
    : "匹配 " + matched + " 条，共 " + loaded + " 条";
}

/** What an empty list is for, and how it fills up. */
const emptyNotes: Record<string, string> = {
  games: "添加一款游戏，填写区服和进程后发布规则，客户端才会显示它。",
  lines: "线路由服务端点组成，发布并通过探测后玩家才能选择。",
  nodes: "添加节点后生成注册凭据，在服务器上运行节点 Agent 完成注册。",
  endpoints: "端点是节点上的一个入口、中转或落地服务。",
  bindings: "把落地端点分配给指定用户，可共享或独享。",
  users: "为玩家和运维人员开通账号。",
  devices: "玩家登录客户端后，设备会出现在这里。",
  sessions: "玩家开始加速后，会话会出现在这里。",
  jobs: "发布线路和开始加速时，节点任务会出现在这里。",
  releases: "登记新版本客户端的下载地址和校验信息。",
  audits: "后台的每一次修改都会记录在这里。",
};

function EmptyState({
  section,
  loading,
  failed,
  loaded,
  searching,
  hasMore,
  onCreate,
}: {
  section: string;
  loading: boolean;
  failed: boolean;
  loaded: number;
  searching: boolean;
  hasMore: boolean;
  onCreate?: () => void;
}) {
  if (loading && !loaded)
    return (
      <div className="empty">
        <span className="empty-icon">
          <LoaderCircle size={26} />
        </span>
        <h3>正在读取…</h3>
      </div>
    );
  // Nothing arrived, so nothing is known about the list; do not call it empty.
  if (failed && !loaded)
    return (
      <div className="empty">
        <span className="empty-icon">
          <CircleAlert size={26} />
        </span>
        <h3>未能读取记录</h3>
        <p>请查看上方提示后重试。</p>
      </div>
    );
  if (searching && loaded)
    return (
      <div className="empty">
        <span className="empty-icon">
          <Search size={26} />
        </span>
        <h3>没有匹配的记录</h3>
        <p>
          {hasMore
            ? "只搜索了已加载的 " + loaded + " 条，加载更多后可继续查找。"
            : "换个关键词试试。"}
        </p>
      </div>
    );
  return (
    <div className="empty">
      <span className="empty-icon">
        <Inbox size={26} />
      </span>
      <h3>还没有记录</h3>
      <p>{emptyNotes[section] || "资源运行后会自动产生记录。"}</p>
      {onCreate && (
        <button className="primary" onClick={onCreate}>
          <Plus size={16} />
          新增
        </button>
      )}
    </div>
  );
}

export function ResourceTable({
  section,
  rows,
  hasMore,
  loading,
  loadingMore,
  moreError,
  failed,
  onLoadMore,
  search,
  onSearch,
  names = {},
  covers = {},
  onCreate,
  renderActions,
}: {
  section: string;
  rows: Row[];
  hasMore: boolean;
  loading: boolean;
  loadingMore: boolean;
  moreError: string;
  /** The last load failed; shown above the table, but it changes the empty state. */
  failed: boolean;
  onLoadMore: () => void;
  search: string;
  onSearch: (value: string) => void;
  /** Display names for ids in reference columns. */
  names?: Record<string, string>;
  /** Cover index and picture of games, for game columns outside the games list. */
  covers?: Record<string, { index?: number; image?: string }>;
  /** Offered in the empty state when the operator may create rows. */
  onCreate?: () => void;
  renderActions: (row: Row) => ReactNode;
}) {
  const cols = columns[section] || [];
  const needle = search.toLowerCase();
  // Matches what the table shows (玩家, 运行中, a user's name) as well as
  // the raw values and ids.
  const filtered = needle
    ? rows.filter((r) =>
        (JSON.stringify(r) + " " + cols.map((c) => text(r[c], c, names)).join(" "))
          .toLowerCase()
          .includes(needle),
      )
    : rows;
  const searching = needle !== "";
  return (
    <>
      <div className="table-toolbar">
        <label className="search">
          <Search size={16} />
          <input
            value={search}
            onChange={(e) => onSearch(e.target.value)}
            placeholder="搜索名称、状态或标识"
            aria-label="搜索"
          />
        </label>
        <span>
          {loading && !rows.length
            ? "正在读取…"
            : failed && !rows.length
              ? "未能读取"
              : countLabel(rows.length, filtered.length, searching, hasMore)}
        </span>
      </div>
      <div className="table-card" aria-busy={loading}>
        {filtered.length > 0 && (
          <table>
            <thead>
              <tr>
                {cols.map((c) => (
                  <th key={c}>{columnLabel(section, c)}</th>
                ))}
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((row) => (
                <tr key={row.id}>
                  {cols.map((col) => (
                    <td key={col} data-label={columnLabel(section, col)}>
                      {renderCell(section, col, row, names, covers)}
                    </td>
                  ))}
                  <td className="actions-cell">
                    <div className="row-actions">{renderActions(row)}</div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
        {!filtered.length && (
          <EmptyState
            section={section}
            loading={loading}
            failed={failed}
            loaded={rows.length}
            searching={searching}
            hasMore={hasMore}
            onCreate={onCreate}
          />
        )}
      </div>
      {(hasMore || moreError) && (
        <div className="table-foot">
          {moreError ? (
            <span className="table-foot-error" role="alert">
              {moreError}
            </span>
          ) : (
            <span>
              {searching ? "搜索仅覆盖已加载的记录" : "按列表顺序继续读取下一页"}
            </span>
          )}
          <button
            type="button"
            className="secondary"
            onClick={onLoadMore}
            disabled={loading || loadingMore}
          >
            {loadingMore ? "正在加载…" : moreError ? "重试加载" : "加载更多"}
          </button>
        </div>
      )}
    </>
  );
}

/** Column headings that read better in one section than the shared label. */
function columnLabel(section: string, col: string) {
  if (section === "games" && col === "name") return "游戏";
  if (section === "lines" && col === "name") return "线路";
  if (section === "nodes" && col === "name") return "节点";
  if (section === "endpoints" && col === "name") return "端点";
  if (section === "sessions" && col === "user_id") return "玩家";
  if (section === "bindings" && col === "user_id") return "用户";
  if (section === "devices" && col === "user_id") return "用户";
  if (section === "games" && col === "published_version") return "规则状态";
  if (section === "lines" && col === "published_version") return "发布与验证";
  if (section === "lines" && col === "draining") return "接收状态";
  if (section === "nodes" && col === "last_seen") return "心跳";
  if (section === "bindings" && col === "verified_at") return "验证";
  return labels[col] || col;
}
