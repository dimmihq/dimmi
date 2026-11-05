/*
 * The home links while someone edits them. A link still being filled in,
 * without a title or an address yet, stays out of the preview, so the
 * preview never refuses a row nobody has finished; Save sends every row,
 * and an unfinished one then says what it lacks. What the server says
 * about a link lands on the row it is about.
 */
import { ApiError } from "@/lib/api/client";
import type { HomeLink, WidgetChange } from "@/lib/api/widget-settings";

function complete(link: HomeLink): boolean {
  return link.title.trim() !== "" && link.url.trim() !== "";
}

/*
 * forPreview returns the draft without unfinished links, and the row of
 * each link it keeps.
 */
export function forPreview(draft: WidgetChange): { change: WidgetChange; rows: number[] } {
  if (!draft.home_links) {
    return { change: draft, rows: [] };
  }
  const rows = draft.home_links.flatMap((link, row) => (complete(link) ? [row] : []));

  return { change: { ...draft, home_links: rows.flatMap((row) => draft.home_links?.[row] ?? []) }, rows };
}

/*
 * linkProblems returns what the error says about the links, keyed by
 * their rows; rows maps the links the request sent back to their rows,
 * and is left out when every row was sent.
 */
export function linkProblems(error: Error | null, rows?: number[]): Record<string, string> {
  if (!(error instanceof ApiError)) {
    return {};
  }

  return Object.fromEntries(
    Object.entries(error.fields).flatMap(([field, message]) => {
      const [, sent, rest] = field.match(/^home_links\.(\d+)\.(.+)$/) ?? [];
      if (sent === undefined) {
        return field === "home_links" ? [[field, message]] : [];
      }
      const row = rows ? rows[Number(sent)] : Number(sent);

      return row === undefined ? [] : [[`home_links.${row}.${rest}`, message]];
    }),
  );
}
