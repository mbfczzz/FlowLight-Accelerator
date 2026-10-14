import { useEffect, useRef, useState } from "react";
import { api, isSessionExpired, PAGE_SIZE, type Row } from "./api";
import { loadFromStart, loadNext, type Paged } from "./paging";

/** Error text for display. Session expiry is handled by the shell, not shown. */
export const messageOf = (e: unknown) => (e as Error)?.message || "请求失败";

type Loaded = Paged & { kind: string };
const nothing: Loaded = { kind: "", rows: [], hasMore: false };

/**
 * Rows (or the overview) for the section on screen, with paging.
 *
 * Every load takes a generation number; a response is applied only while
 * its generation is still current, so a slow reply for a section the
 * operator has left cannot overwrite the one they are looking at.
 */
export function useSectionData(section: string) {
  const [list, setList] = useState<Loaded>(nothing);
  const [overview, setOverview] = useState<Row>({});
  const [loading, setLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState("");
  const [moreError, setMoreError] = useState("");
  const generation = useRef(0);
  const moreFor = useRef(-1);

  // Rows tagged with another section are never shown, so switching sections
  // does not flash the previous table under the new headings.
  const rows = list.kind === section ? list.rows : [];
  const hasMore = list.kind === section && list.hasMore;

  const latest = useRef({ section, rows });
  useEffect(() => {
    latest.current = { section, rows };
  });

  async function fetchFromStart(kind: string, target: number) {
    const g = ++generation.current;
    setLoading(true);
    setLoadingMore(false);
    setError("");
    setMoreError("");
    if (kind === "overview") setOverview({});
    try {
      if (kind === "overview") {
        const value = await api("/overview");
        if (g === generation.current) setOverview(value ?? {});
      } else {
        const next = await loadFromStart(kind, target);
        if (g === generation.current) setList({ kind, ...next });
      }
    } catch (e) {
      if (g === generation.current && !isSessionExpired(e))
        setError(messageOf(e));
    } finally {
      if (g === generation.current) setLoading(false);
    }
  }

  useEffect(() => {
    void fetchFromStart(section, PAGE_SIZE);
    return () => {
      generation.current++;
    };
  }, [section]);

  /**
   * Reload whatever section is on screen now, keeping as many rows as are
   * loaded. Safe to call after an await: it never reloads a section the
   * operator has since left into the one they are viewing.
   */
  function reload() {
    const { section: kind, rows: loaded } = latest.current;
    return fetchFromStart(kind, Math.max(PAGE_SIZE, loaded.length));
  }

  async function loadMore() {
    const { section: kind, rows: loaded } = latest.current;
    const g = generation.current;
    // One page request per generation; a reload starts a new generation.
    if (kind === "overview" || moreFor.current === g) return;
    moreFor.current = g;
    setLoadingMore(true);
    setMoreError("");
    try {
      const next = await loadNext(kind, loaded);
      if (g === generation.current) setList({ kind, ...next });
    } catch (e) {
      if (g === generation.current && !isSessionExpired(e))
        setMoreError(messageOf(e));
    } finally {
      if (moreFor.current === g) moreFor.current = -1;
      if (g === generation.current) setLoadingMore(false);
    }
  }

  return {
    rows,
    hasMore,
    overview,
    loading,
    loadingMore,
    error,
    moreError,
    reload,
    loadMore,
  };
}
