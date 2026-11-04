/*
 * Patching cached query data with a record a broadcast carried. Lists
 * come in several shapes: a plain array, an API collection with its
 * rows under data, or an infinite query's pages of either. A record
 * replaces the cached row with the same id, keeping fields the
 * broadcast left out; whether the record was found tells the caller to
 * refetch a list it may now belong to.
 */

export interface Identified {
  id: string;
}

export interface Patched<T> {
  data: T;
  found: boolean;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isIdentified(value: unknown): value is Identified {
  return isRecord(value) && typeof value.id === "string";
}

function patchArray(rows: unknown[], record: Identified): Patched<unknown[]> {
  let found = false;
  const data = rows.map((row) => {
    if (isIdentified(row) && row.id === record.id) {
      found = true;
      return { ...row, ...record };
    }
    return row;
  });

  return { data: found ? data : rows, found };
}

/*
 * upsertRecord returns the cached data with the record merged into the
 * row of the same id, wherever the shape keeps its rows.
 */
export function upsertRecord<T>(cached: T, record: Identified): Patched<T> {
  if (Array.isArray(cached)) {
    return patchArray(cached, record) as Patched<T>;
  }

  if (!isRecord(cached)) {
    return { data: cached, found: false };
  }

  if (Array.isArray(cached.pages)) {
    let found = false;
    const pages = cached.pages.map((page: unknown) => {
      const patched = upsertRecord(page, record);
      found ||= patched.found;
      return patched.data;
    });

    return { data: (found ? { ...cached, pages } : cached) as T, found };
  }

  if (Array.isArray(cached.data)) {
    const patched = patchArray(cached.data, record);
    return { data: (patched.found ? { ...cached, data: patched.data } : cached) as T, found: patched.found };
  }

  if (cached.id === record.id) {
    return { data: { ...cached, ...record } as T, found: true };
  }

  return { data: cached, found: false };
}

/*
 * appendRecord adds a record at the end of a cached timeline, the last
 * page of an infinite one. It returns false for a shape it cannot
 * append to, so the caller refetches instead.
 */
export function appendRecord<T>(cached: T, record: Identified): Patched<T> {
  if (Array.isArray(cached)) {
    return { data: [...cached, record] as T, found: true };
  }

  if (!isRecord(cached)) {
    return { data: cached, found: false };
  }

  if (Array.isArray(cached.pages) && cached.pages.length > 0) {
    const pages = [...cached.pages];
    const last = appendRecord(pages[pages.length - 1], record);
    if (!last.found) {
      return { data: cached, found: false };
    }
    pages[pages.length - 1] = last.data;
    return { data: { ...cached, pages } as T, found: true };
  }

  if (Array.isArray(cached.data)) {
    return { data: { ...cached, data: [...cached.data, record] } as T, found: true };
  }

  return { data: cached, found: false };
}
