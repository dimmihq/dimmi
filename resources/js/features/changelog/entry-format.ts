/*
 * How a changelog entry reads in the team's lists and editor: its words
 * as the editor holds them, what a save sends, where it stands, and the
 * date a list shows beside it.
 */
import type { ChangelogEntry, EntryChanges } from "@/lib/api/changelog";
import { formatDate, formatDateTime } from "@/lib/i18n/format";

/*
 * Draft is the entry as the editor holds it before a save. Authors are
 * member ids; none means the editors are named.
 */
export interface Draft {
  title: string;
  slug: string;
  summary: string;
  body: string;
  cover: string;
  labels: string[];
  authors: string[];
}

export function draftOf(entry: ChangelogEntry): Draft {
  return {
    title: entry.title,
    slug: entry.slug,
    summary: entry.summary ?? "",
    body: entry.body_markdown ?? "",
    cover: entry.cover_url ?? "",
    labels: entry.labels.map((l) => l.id),
    authors: entry.authors_chosen ? entry.authors.map((a) => a.member_id) : [],
  };
}

/*
 * changeOf is what a save sends. The address goes only when someone
 * changed it, so a draft's address can keep following its title.
 */
export function changeOf(draft: Draft, entry: ChangelogEntry): EntryChanges {
  return {
    title: draft.title,
    ...(draft.slug === entry.slug ? {} : { slug: draft.slug }),
    summary: draft.summary,
    body_markdown: draft.body,
    cover_url: draft.cover,
    label_ids: draft.labels,
    author_ids: draft.authors,
  };
}

export type EntryState = "Draft" | "Scheduled" | "Published" | "Unpublished changes";

export function entryState(entry: ChangelogEntry): EntryState {
  if (entry.status === "scheduled") {
    return "Scheduled";
  }
  if (entry.status === "published") {
    return entry.has_unpublished_changes ? "Unpublished changes" : "Published";
  }
  return "Draft";
}

/*
 * entryWhen is the date a list shows: when it shipped, when it will, or
 * when it was last edited.
 */
export function entryWhen(entry: ChangelogEntry): string {
  if (entry.status === "scheduled" && entry.scheduled_for) {
    return `Publishes ${formatDateTime(entry.scheduled_for)}`;
  }
  if (entry.status === "published" && entry.published_at) {
    return `Published ${formatDate(entry.published_at)}`;
  }
  return `Edited ${formatDate(entry.updated_at)}`;
}

/*
 * authorNames joins the names of an entry's authors for a list.
 */
export function authorNames(entry: ChangelogEntry): string {
  const names = entry.authors.map((a) => a.name);
  if (names.length <= 2) {
    return names.join(" and ");
  }
  return `${names.slice(0, 2).join(", ")} and ${names.length - 2} more`;
}

export function initialsOf(name: string): string {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((word) => word[0]?.toUpperCase() ?? "")
    .join("");
}
