import { useCallback, useEffect, useState } from "react";
import { isSessionExpired, listPage, MAX_PAGE_SIZE, type Row } from "./api";

/** Lists whose rows other rows refer to by id. */
const referenced = ["users", "nodes", "endpoints", "lines", "games", "devices"];

export type Names = Record<string, string>;

/**
 * Display names for ids that appear in other rows (操作人, 所属节点, 线路 …),
 * so lists show "admin" rather than a 32-character id. A list that fails to
 * load leaves its ids as they are.
 */
export function useNames(section: string) {
  const [names, setNames] = useState<Names>({});
  const load = useCallback(async () => {
    const lists = await Promise.all(
      referenced.map((k) =>
        listPage(k, MAX_PAGE_SIZE, 0).catch((e) => {
          if (isSessionExpired(e)) throw e;
          return [] as Row[];
        }),
      ),
    );
    const next: Names = {};
    for (const rows of lists)
      for (const r of rows)
        if (r.id) next[r.id] = r.name || r.username || r.id;
    setNames(next);
  }, []);
  useEffect(() => {
    if (section !== "overview") load().catch(() => {});
  }, [section, load]);
  return { names, reloadNames: () => load().catch(() => {}) };
}
