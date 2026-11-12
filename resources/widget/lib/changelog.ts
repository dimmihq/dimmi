/*
 * The What's new tab as a connected messenger reads it: the newest
 * published entries of the changelog the messenger links to, with how
 * many are newer than the newest one the visitor saw on this browser. It
 * is a chunk of its own, fetched only by a messenger with the tab on.
 */

import type { Http } from "../client/http";
import type { Block, Entry } from "../content";

/*
 * Index is what the server answers for the tab.
 */
interface Index {
  url: string;
  entries: { slug: string; title: string; summary: string; label: string; published_at: string; body: Block[] }[];
}

const day: Intl.DateTimeFormatOptions = { month: "short", day: "numeric", year: "numeric" };

/*
 * changelog loads the tab. seen is when the newest entry the visitor
 * saw was published, null when they never opened the tab here.
 */
export async function changelog(
  http: Http,
  appId: string,
  seen: string | null,
): Promise<{ changelog: Entry[]; unread_changelog: number; latest: string }> {
  const entries = (await http.get<{ data: Index }>(`/apps/${appId}/changelog`)).data.entries;
  const since = Date.parse(seen ?? "");

  return {
    changelog: entries.map((e) => ({
      tag: e.label,
      title: e.title,
      summary: e.summary,
      date: new Date(e.published_at).toLocaleDateString(undefined, day),
      body: e.body,
    })),
    unread_changelog: entries.filter((e) => Number.isNaN(since) || Date.parse(e.published_at) > since).length,
    latest: entries[0]?.published_at ?? "",
  };
}
