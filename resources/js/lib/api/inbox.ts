/*
 * The shared inbox: the queries that read a view's conversations, one
 * conversation and its timeline, the sidebar's counts, and the calls a
 * teammate makes on a case: replying, adding a note, assigning, and
 * closing, reopening or snoozing it. Every call names the workspace in
 * the Dimmi-Workspace header.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export type ConversationStatus = "open" | "snoozed" | "closed";

export interface Named {
  id: string;
  object: string;
  name: string;
}

export interface ContactSummary {
  id: string;
  kind: "user" | "lead";
  name: string | null;
  email: string | null;
  avatar_url: string | null;
}

export interface Author {
  type: "contact" | "member" | "agent" | "system" | "integration";
  id: string | null;
  name: string;
}

export interface MessageSummary {
  id: string;
  type: string;
  author: Author;
  excerpt: string;
  created_at: string;
}

export interface Conversation {
  id: string;
  number: number;
  subject: string | null;
  status: ConversationStatus;
  spam: boolean;
  priority: boolean;
  unread: boolean;
  snoozed_until: string | null;
  last_message_at: string | null;
  closed_at: string | null;
  message_count: number;
  channel: { id: string; type: string; name: string };
  inbox: Named;
  contact: ContactSummary;
  assignee: Named | null;
  team: Named | null;
  last_message: MessageSummary | null;
  created_at: string;
  updated_at: string;
}

/*
 * TimelineEvent is what an event entry says happened: its name, who did
 * it, and what it carried, such as who a conversation went to.
 */
export interface TimelineEvent {
  name: string;
  actor: { type: string; id: string; name: string };
  reason?: string;
  until?: string;
  priority?: boolean;
  to?: { member: Named | null; team: Named | null };
  changes?: Record<string, [unknown, unknown]>;
}

export interface Message {
  id: string;
  conversation_id: string;
  type: "message" | "note" | "event";
  author: Author;
  body: string | null;
  delivery_status: string | null;
  event: TimelineEvent | null;
  deleted_at: string | null;
  created_at: string;
}

export interface Counts {
  views: {
    mine: { open: number; unread: number; snoozed: number };
    unassigned: { open: number; unread: number };
    all: { open: number; capped: boolean };
    mentions: { unread: number };
  };
}

/*
 * ConversationChanges are the fields a teammate changes on a case; the
 * server applies them together, in its own fixed order.
 */
export interface ConversationChanges {
  assignee_id?: string | null;
  status?: ConversationStatus;
  snooze_preset?: "later_today" | "tomorrow" | "next_week" | "one_week";
  priority?: boolean;
  unread?: boolean;
}

/*
 * viewFilter is how an inbox view of the sidebar asks the list: the
 * view, and for mentions the open conversations someone mentioned the
 * person in.
 */
export function conversationsPath(view: string, search: string): string {
  const params = new URLSearchParams({ "filter[view]": view, limit: "50" });
  if (search.trim()) {
    params.set("q", search.trim());
    params.set("filter[status]", "any");
  }

  return `/conversations?${params.toString()}`;
}

export function conversationsQuery(workspace: string, view: string, search = "") {
  return queryOptions({
    queryKey: ["workspace", workspace, "conversations", view, search],
    queryFn: () => request<Conversation[]>("GET", conversationsPath(view, search), undefined, { workspace }),
  });
}

export function conversationQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "conversation", id],
    queryFn: () => request<Conversation>("GET", `/conversations/${encodeURIComponent(id)}`, undefined, { workspace }),
  });
}

export function messagesQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "conversation", id, "messages"],
    queryFn: () =>
      request<Message[]>("GET", `/conversations/${encodeURIComponent(id)}/messages?limit=100`, undefined, {
        workspace,
      }),
  });
}

export function countsQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "conversation-counts"],
    queryFn: () => request<Counts>("GET", "/conversation-counts", undefined, { workspace }),
  });
}

export function updateConversation(workspace: string, id: string, changes: ConversationChanges): Promise<Conversation> {
  return request<Conversation>("PATCH", `/conversations/${encodeURIComponent(id)}`, changes, { workspace });
}

export function reply(workspace: string, id: string, body: string): Promise<Message> {
  return request<Message>(
    "POST",
    `/conversations/${encodeURIComponent(id)}/replies`,
    { body, client_message_id: crypto.randomUUID() },
    { workspace },
  );
}

export function addNote(workspace: string, id: string, body: string): Promise<Message> {
  return request<Message>(
    "POST",
    `/conversations/${encodeURIComponent(id)}/notes`,
    { body, client_message_id: crypto.randomUUID() },
    { workspace },
  );
}

export function markRead(workspace: string, id: string): Promise<unknown> {
  return request<unknown>("PATCH", `/conversations/${encodeURIComponent(id)}/read-state`, {}, { workspace });
}
