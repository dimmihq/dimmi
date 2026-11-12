/*
 * Saved replies: text a teammate inserts instead of typing it again,
 * shared with the workspace or their own, optionally a macro that also
 * changes the conversation once the reply is sent. Anyone who works
 * conversations keeps their own; shared ones take the
 * saved_replies:manage_shared permission, which the server checks.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";
import type { ConversationPriority } from "./inbox";

export type SnoozePreset = "later_today" | "tomorrow" | "next_week" | "one_week";

/*
 * SavedReplyActions is what a macro does to the conversation: a status,
 * with the snooze preset when snoozing, a teammate or team to assign,
 * tags to add and a priority to set.
 */
export interface SavedReplyActions {
  status?: "open" | "closed" | "snoozed";
  snooze_preset?: SnoozePreset;
  assignee_id?: string;
  team_id?: string;
  tag_ids: string[];
  priority?: ConversationPriority;
}

export interface SavedReply {
  id: string;
  object: "saved_reply";
  title: string;
  body: string;
  shortcut: string | null;
  shared: boolean;
  owner_id: string | null;
  actions: SavedReplyActions;
  created_at: string;
  updated_at: string;
}

/*
 * SavedReplyFields are what a create or update sends; a null shortcut
 * removes it.
 */
export interface SavedReplyFields {
  title?: string;
  body?: string;
  shortcut?: string | null;
  shared?: boolean;
  actions?: Partial<SavedReplyActions>;
}

/*
 * ImportedReply is one row of a spreadsheet import.
 */
export interface ImportedReply {
  title: string;
  body: string;
  shortcut?: string;
  shared: boolean;
}

export function savedRepliesKey(workspace: string) {
  return ["workspace", workspace, "saved-replies"];
}

export function savedRepliesQuery(workspace: string, search = "") {
  const q = search.trim();

  return queryOptions({
    queryKey: [...savedRepliesKey(workspace), q],
    queryFn: () =>
      request<SavedReply[]>("GET", `/saved-replies${q ? `?q=${encodeURIComponent(q)}` : ""}`, undefined, {
        workspace,
      }),
  });
}

export function createSavedReply(workspace: string, fields: SavedReplyFields): Promise<SavedReply> {
  return request<SavedReply>("POST", "/saved-replies", fields, { workspace });
}

export function updateSavedReply(workspace: string, id: string, fields: SavedReplyFields): Promise<SavedReply> {
  return request<SavedReply>("PATCH", `/saved-replies/${encodeURIComponent(id)}`, fields, { workspace });
}

export function deleteSavedReply(workspace: string, id: string): Promise<unknown> {
  return request<unknown>("DELETE", `/saved-replies/${encodeURIComponent(id)}`, undefined, { workspace });
}

export function importSavedReplies(workspace: string, replies: ImportedReply[]): Promise<SavedReply[]> {
  return request<SavedReply[]>("POST", "/saved-reply-imports", { replies }, { workspace });
}
