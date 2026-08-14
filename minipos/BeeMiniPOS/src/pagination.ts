export type CursorPage<T> = {
  items?: T[];
  page?: { next_cursor?: string | null; has_more?: boolean };
};

export async function collectCursorPages<T>(
  path: string,
  request: (path: string) => Promise<CursorPage<T>>,
  pageSize = 100,
  maxPages = 100,
): Promise<T[]> {
  const items: T[] = [];
  const seen = new Set<string>();
  let cursor: string | null = null;
  for (let pageNumber = 0; pageNumber < maxPages; pageNumber += 1) {
    const separator = path.includes("?") ? "&" : "?";
    const page = await request(
      `${path}${separator}limit=${pageSize}${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`,
    );
    items.push(...(page.items || []));
    if (!page.page?.has_more) return items;
    const next = page.page.next_cursor;
    if (!next || seen.has(next)) throw new Error("INVALID_PAGINATION_SEQUENCE");
    seen.add(next);
    cursor = next;
  }
  throw new Error("PAGINATION_LIMIT_EXCEEDED");
}
