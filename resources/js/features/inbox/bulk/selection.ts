/*
 * Picking conversations in a list for a bulk change. A click toggles one
 * row; a shift-click sets every row between the last one clicked and
 * this one to what the click gives this one, as file lists do. Rows that
 * leave the list leave the selection, so a change never reaches a
 * conversation the person no longer sees.
 */
import { useCallback, useMemo, useState } from "react";

export interface Selection {
  ids: ReadonlySet<string>;
  anchor: string | null;
}

export const emptySelection: Selection = { ids: new Set(), anchor: null };

/*
 * toggled returns the selection after a click on the row, a range from
 * the anchor when range is set and the anchor is still in the list.
 */
export function toggled(selection: Selection, rows: readonly string[], id: string, range: boolean): Selection {
  const on = !selection.ids.has(id);
  const next = new Set(selection.ids);
  const from = selection.anchor ? rows.indexOf(selection.anchor) : -1;
  const to = rows.indexOf(id);

  if (range && from >= 0 && to >= 0) {
    const [start, end] = from < to ? [from, to] : [to, from];
    for (const row of rows.slice(start, end + 1)) {
      if (on) {
        next.add(row);
      } else {
        next.delete(row);
      }
    }
  } else if (on) {
    next.add(id);
  } else {
    next.delete(id);
  }

  return { ids: next, anchor: id };
}

/*
 * shown keeps only the selected rows still in the list.
 */
export function shown(selection: Selection, rows: readonly string[]): Selection {
  const present = rows.filter((id) => selection.ids.has(id));
  if (present.length === selection.ids.size) {
    return selection;
  }

  return {
    ids: new Set(present),
    anchor: selection.anchor && rows.includes(selection.anchor) ? selection.anchor : null,
  };
}

/*
 * useSelection keeps the picked rows of the list and the moves on them.
 */
export function useSelection(rows: readonly string[]) {
  const [raw, setRaw] = useState<Selection>(emptySelection);
  const selection = useMemo(() => shown(raw, rows), [raw, rows]);

  const toggle = useCallback(
    (id: string, range = false) => setRaw((was) => toggled(shown(was, rows), rows, id, range)),
    [rows],
  );
  const selectAll = useCallback(() => setRaw({ ids: new Set(rows), anchor: null }), [rows]);
  const clear = useCallback(() => setRaw(emptySelection), []);

  return { ids: selection.ids, count: selection.ids.size, toggle, selectAll, clear };
}

export type SelectionControls = ReturnType<typeof useSelection>;
