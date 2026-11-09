/*
 * A help center's changelog: its entries, labels and subscribers. An
 * entry is saved as a working copy and goes live only when published,
 * now or at a scheduled time; a save names the lock version it started
 * from, so two editors never overwrite each other. Publishing can mail
 * the active subscribers, once per entry.
 */
import { keepPreviousData, queryOptions } from "@tanstack/react-query";

import { request, requestBody, requestPage } from "./client";

export type EntryStatus = "draft" | "scheduled" | "published";

export interface ChangelogTag {
  id: string;
  name: string;
  slug: string;
  color: string;
}

export interface ChangelogPerson {
  member_id: string;
  name: string;
  avatar_url: string | null;
}

export interface ChangelogEntry {
  id: string;
  object: "changelog_entry";
  help_center_id: string;
  title: string;
  slug: string;
  summary: string | null;
  body_markdown?: string;
  cover_url: string | null;
  status: EntryStatus;
  scheduled_for: string | null;
  published_at: string | null;
  has_unpublished_changes: boolean;
  notify_subscribers: boolean;
  notified_at: string | null;
  authors_chosen: boolean;
  labels: ChangelogTag[];
  authors: ChangelogPerson[];
  editors: ChangelogPerson[];
  lock_version: number;
  created_at: string;
  updated_at: string;
}

export interface ChangelogLabel {
  id: string;
  object: "changelog_label";
  name: string;
  slug: string;
  color: string;
  position: number;
}

export interface ChangelogSubscriber {
  id: string;
  object: "changelog_subscriber";
  email: string;
  contact_id: string | null;
  status: "pending" | "active" | "unsubscribed";
  confirmed_at: string | null;
  unsubscribed_at: string | null;
  created_at: string;
}

/*
 * EntryChanges are an entry's fields to save; label_ids and author_ids
 * replace what it had, and an empty author_ids goes back to the editors.
 */
export interface EntryChanges {
  title?: string;
  slug?: string;
  summary?: string;
  body_markdown?: string;
  cover_url?: string;
  label_ids?: string[];
  author_ids?: string[];
}

/*
 * PublishOptions publish now, dated published_at when given, or hold the
 * entry until scheduled_for; notify mails the active subscribers once.
 */
export interface PublishOptions {
  notify?: boolean;
  published_at?: string;
  scheduled_for?: string;
}

export interface EntryFilter {
  status?: EntryStatus;
  label?: string;
  query?: string;
  page: number;
}

export interface EntryList {
  data: ChangelogEntry[];
  meta: { total?: number; per_page: number };
  counts: Record<EntryStatus, number>;
}

export const entriesPerPage = 25;

export const changelogKeys = {
  all: (workspace: string) => ["changelog", workspace] as const,
  entries: (workspace: string, helpCenter: string) => ["changelog", workspace, helpCenter, "entries"] as const,
  entry: (workspace: string, id: string) => ["changelog", workspace, "entry", id] as const,
  labels: (workspace: string, helpCenter: string) => ["changelog", workspace, helpCenter, "labels"] as const,
  subscribers: (workspace: string, helpCenter: string) => ["changelog", workspace, helpCenter, "subscribers"] as const,
};

function params(pairs: [string, string | undefined][]): string {
  const out = new URLSearchParams();
  for (const [key, value] of pairs) {
    if (value) {
      out.set(key, value);
    }
  }
  return out.toString();
}

export function entriesQuery(workspace: string, helpCenter: string, f: EntryFilter) {
  const query = params([
    ["status", f.status],
    ["label_id", f.label],
    ["query", f.query],
    ["page", String(f.page)],
  ]);

  return queryOptions({
    queryKey: [...changelogKeys.entries(workspace, helpCenter), f] as const,
    queryFn: () =>
      requestBody<EntryList>("GET", `/help-centers/${helpCenter}/changelog-entries?${query}`, undefined, {
        workspace,
      }),
    placeholderData: keepPreviousData,
    enabled: workspace !== "" && helpCenter !== "",
  });
}

export function entryQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: changelogKeys.entry(workspace, id),
    queryFn: () => request<ChangelogEntry>("GET", `/changelog-entries/${id}`, undefined, { workspace }),
    enabled: workspace !== "",
  });
}

export function labelsQuery(workspace: string, helpCenter: string) {
  return queryOptions({
    queryKey: changelogKeys.labels(workspace, helpCenter),
    queryFn: () =>
      request<ChangelogLabel[]>("GET", `/help-centers/${helpCenter}/changelog-labels`, undefined, { workspace }),
    enabled: workspace !== "" && helpCenter !== "",
  });
}

export function subscribersQuery(
  workspace: string,
  helpCenter: string,
  f: { status?: ChangelogSubscriber["status"]; query?: string; page: number },
) {
  const query = params([
    ["status", f.status],
    ["query", f.query],
    ["page", String(f.page)],
  ]);

  return queryOptions({
    queryKey: [...changelogKeys.subscribers(workspace, helpCenter), f] as const,
    queryFn: () =>
      requestPage<ChangelogSubscriber>(`/help-centers/${helpCenter}/changelog-subscribers?${query}`, { workspace }),
    placeholderData: keepPreviousData,
    enabled: workspace !== "" && helpCenter !== "",
  });
}

export function createEntry(workspace: string, helpCenter: string, changes: EntryChanges): Promise<ChangelogEntry> {
  return request<ChangelogEntry>("POST", `/help-centers/${helpCenter}/changelog-entries`, changes, { workspace });
}

export function saveEntry(
  workspace: string,
  id: string,
  changes: EntryChanges & { lock_version: number },
): Promise<ChangelogEntry> {
  return request<ChangelogEntry>("PATCH", `/changelog-entries/${id}`, changes, { workspace });
}

export function publishEntry(workspace: string, id: string, options: PublishOptions): Promise<ChangelogEntry> {
  return request<ChangelogEntry>("POST", `/changelog-entries/${id}/publications`, options, { workspace });
}

export function unpublishEntry(workspace: string, id: string): Promise<ChangelogEntry> {
  return request<ChangelogEntry>("DELETE", `/changelog-entries/${id}/publications`, undefined, { workspace });
}

export function deleteEntry(workspace: string, id: string): Promise<void> {
  return request<void>("DELETE", `/changelog-entries/${id}`, undefined, { workspace });
}

export function createLabel(
  workspace: string,
  helpCenter: string,
  label: { name: string; color?: string },
): Promise<ChangelogLabel> {
  return request<ChangelogLabel>("POST", `/help-centers/${helpCenter}/changelog-labels`, label, { workspace });
}

export function updateLabel(
  workspace: string,
  id: string,
  changes: { name?: string; color?: string; position?: number },
): Promise<ChangelogLabel> {
  return request<ChangelogLabel>("PATCH", `/changelog-labels/${id}`, changes, { workspace });
}

export function deleteLabel(workspace: string, id: string): Promise<void> {
  return request<void>("DELETE", `/changelog-labels/${id}`, undefined, { workspace });
}

export function deleteSubscriber(workspace: string, id: string): Promise<void> {
  return request<void>("DELETE", `/changelog-subscribers/${id}`, undefined, { workspace });
}

/*
 * labelColors are the colors offered for a label, in the order the
 * picker shows them.
 */
export const labelColors = ["#22c55e", "#3b82f6", "#f97316", "#a855f7", "#ef4444", "#eab308", "#14b8a6", "#64748b"];
