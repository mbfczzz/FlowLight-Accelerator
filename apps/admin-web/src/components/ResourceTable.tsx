import type { ReactNode } from "react";
import { Box, Search } from "lucide-react";
import type { Row } from "../api";
import { badgeColumns, labels, text } from "../format";
import { columns } from "../sections";

/**
 * Count shown beside the search box. It only claims a total once the last
 * page has arrived; until then it says how many rows are loaded.
 */
function countLabel(loaded: number, matched: number, searching: boolean, hasMore: boolean) {
  if (!searching)
    return hasMore ? "已加载 " + loaded + " 条，还有更多" : loaded + " 条记录";
  return hasMore
    ? "已加载的 " + loaded + " 条中匹配 " + matched + " 条"
    : "匹配 " + matched + " 条，共 " + loaded + " 条";
}

function EmptyState({
  loading,
  failed,
  loaded,
  searching,
  hasMore,
  editable,
}: {
  loading: boolean;
  failed: boolean;
  loaded: number;
  searching: boolean;
  hasMore: boolean;
  editable: boolean;
}) {
  if (loading && !loaded)
    return (
      <div className="empty">
        <Box size={28} />
        <h3>正在读取资源…</h3>
      </div>
    );
  // Nothing arrived, so nothing is known about the list; do not call it empty.
  if (failed && !loaded)
    return (
      <div className="empty">
        <Box size={28} />
        <h3>未能读取记录</h3>
        <p>请查看上方提示后重试。</p>
      </div>
    );
  if (searching && loaded)
    return (
      <div className="empty">
        <Search size={28} />
        <h3>没有匹配的记录</h3>
        <p>
          {hasMore
            ? "只搜索了已加载的 " + loaded + " 条，加载更多后可继续查找。"
            : "试试其他关键词。"}
        </p>
      </div>
    );
  return (
    <div className="empty">
      <Box size={28} />
      <h3>暂无记录</h3>
      <p>
        {editable ? "创建资源后，它会出现在这里。" : "资源运行后会自动产生记录。"}
      </p>
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
  editable,
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
  editable: boolean;
  renderActions: (row: Row) => ReactNode;
}) {
  const needle = search.toLowerCase();
  const filtered = needle
    ? rows.filter((r) => JSON.stringify(r).toLowerCase().includes(needle))
    : rows;
  const searching = needle !== "";
  const cols = columns[section] || [];
  return (
    <>
      <div className="table-toolbar">
        <label>
          <Search size={16} />
          <input
            aria-label="搜索已加载的记录"
            placeholder="搜索名称、状态或标识"
            value={search}
            onChange={(e) => onSearch(e.target.value)}
          />
        </label>
        <span aria-live="polite">
          {loading && !rows.length
            ? "正在读取…"
            : failed && !rows.length
              ? "未能读取"
              : countLabel(rows.length, filtered.length, searching, hasMore)}
        </span>
      </div>
      <div className="table-wrap" aria-busy={loading}>
        <table>
          <thead>
            <tr>
              {cols.map((c) => (
                <th key={c}>{labels[c] || c}</th>
              ))}
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((row) => (
              <tr key={row.id}>
                {cols.map((col, i) => (
                  <td key={col}>
                    {badgeColumns.includes(col) ? (
                      <span className={"badge " + String(row[col]).toLowerCase()}>
                        {text(row[col], col)}
                      </span>
                    ) : (
                      <span className={i === 0 ? "cell-main" : ""}>
                        {text(row[col], col)}
                      </span>
                    )}
                  </td>
                ))}
                <td>
                  <div className="row-actions">{renderActions(row)}</div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {!filtered.length && (
          <EmptyState
            loading={loading}
            failed={failed}
            loaded={rows.length}
            searching={searching}
            hasMore={hasMore}
            editable={editable}
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
              {searching
                ? "搜索仅覆盖已加载的记录"
                : "按列表顺序继续读取下一页"}
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
