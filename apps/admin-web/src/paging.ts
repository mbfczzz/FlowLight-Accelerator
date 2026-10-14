import { listPage, MAX_PAGE_SIZE, PAGE_SIZE, type Row } from "./api";

export type Paged = { rows: Row[]; hasMore: boolean };

/**
 * Fold one fetched page into the rows already loaded.
 *
 * - A page longer than the requested limit means the server ignored the
 *   paging parameters and sent the whole collection, so it replaces what we
 *   had and there is nothing more to load.
 * - Rows already loaded are skipped by id. Lists ordered by created_at DESC
 *   shift down when new rows arrive between requests, and the overlap would
 *   otherwise show duplicates.
 * - A short page, or a full page that adds nothing new, ends the list.
 */
export function mergePage(existing: Row[], page: Row[], limit: number): Paged {
  if (page.length > limit) return { rows: page, hasMore: false };
  const seen = new Set(existing.map((r) => r.id));
  const fresh = page.filter((r) => r.id == null || !seen.has(r.id));
  return {
    rows: existing.concat(fresh),
    hasMore: page.length === limit && fresh.length > 0,
  };
}

/**
 * Load a list from the start until at least `target` rows are present or the
 * list ends. Used for the first page and for reloads after a change, so an
 * operator who had paged further does not lose the rows they were viewing.
 */
export async function loadFromStart(kind: string, target = PAGE_SIZE): Promise<Paged> {
  let state: Paged = { rows: [], hasMore: true };
  do {
    const want = Math.max(PAGE_SIZE, target - state.rows.length);
    const limit = Math.min(MAX_PAGE_SIZE, want);
    const page = await listPage(kind, limit, state.rows.length);
    state = mergePage(state.rows, page, limit);
  } while (state.hasMore && state.rows.length < target);
  return state;
}

/** Fetch the page after the rows already loaded and append it. */
export async function loadNext(kind: string, existing: Row[]): Promise<Paged> {
  const page = await listPage(kind, PAGE_SIZE, existing.length);
  return mergePage(existing, page, PAGE_SIZE);
}
