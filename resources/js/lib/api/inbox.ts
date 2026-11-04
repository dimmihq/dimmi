/*
 * The shared inbox: the queries that read a view's conversations, one
 * conversation and its timeline, the sidebar's counts, and the calls a
 * teammate makes on a case: replying, adding a note, assigning, and
 * closing, reopening or snoozing it. Every call names the workspace in
 * the Dimmi-Workspace header, every change the id the screen gave it,
 * and every query is cached under the key realtime updates reach.
 */
import { queryOptions } from "@tanstack/react-query";

import { realtimeKeys } from "@/lib/realtime/keys";

import { request, requestFile } from "./client";

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
  body_format: "markdown" | "plain";
  has_original: boolean;
  truncated: boolean;
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
    queryKey: [...realtimeKeys.conversations(workspace), view, search],
    queryFn: () => request<Conversation[]>("GET", conversationsPath(view, search), undefined, { workspace }),
  });
}

export function conversationQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: realtimeKeys.conversation(workspace, id),
    queryFn: () => request<Conversation>("GET", `/conversations/${encodeURIComponent(id)}`, undefined, { workspace }),
  });
}

export function messagesQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: realtimeKeys.messages(workspace, id),
    queryFn: () =>
      request<Message[]>("GET", `/conversations/${encodeURIComponent(id)}/messages?limit=100`, undefined, {
        workspace,
      }),
  });
}

/*
 * MessageOriginal is the whole of a received email: its full text, its
 * sanitized HTML when it had any, whether either was cut at a size
 * limit, and whether the caller may download the email as it arrived.
 */
export interface MessageOriginal {
  id: string;
  text: string;
  text_truncated: boolean;
  html: string | null;
  html_too_large: boolean;
  raw_available: boolean;
}

function messagePath(conversation: string, message: string): string {
  return `/conversations/${encodeURIComponent(conversation)}/messages/${encodeURIComponent(message)}`;
}

export function messageOriginalQuery(workspace: string, conversation: string, message: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "message-original", message],
    queryFn: () =>
      request<MessageOriginal>("GET", `${messagePath(conversation, message)}/original`, undefined, { workspace }),
    staleTime: Number.POSITIVE_INFINITY,
  });
}

export function downloadRawEmail(workspace: string, conversation: string, message: string): Promise<Blob> {
  return requestFile(`${messagePath(conversation, message)}/original/raw`, { workspace });
}

export function countsQuery(workspace: string) {
  return queryOptions({
    queryKey: realtimeKeys.counts(workspace),
    queryFn: () => request<Counts>("GET", "/conversation-counts", undefined, { workspace }),
  });
}

/*
 * A Write is one change a teammate makes: the workspace it is in, and
 * the id the screen gave it, sent so the screen drops its own echo when
 * the change comes back over realtime.
 */
export interface Write {
  workspace: string;
  mutation: string;
}

function writeOptions(write: Write) {
  return { workspace: write.workspace, headers: { "Dimmi-Client-Mutation-Id": write.mutation } };
}

export function updateConversation(write: Write, id: string, changes: ConversationChanges): Promise<Conversation> {
  return request<Conversation>("PATCH", `/conversations/${encodeURIComponent(id)}`, changes, writeOptions(write));
}

export function reply(write: Write, id: string, body: string): Promise<Message> {
  return request<Message>(
    "POST",
    `/conversations/${encodeURIComponent(id)}/replies`,
    { body, client_message_id: crypto.randomUUID() },
    writeOptions(write),
  );
}

export function addNote(write: Write, id: string, body: string): Promise<Message> {
  return request<Message>(
    "POST",
    `/conversations/${encodeURIComponent(id)}/notes`,
    { body, client_message_id: crypto.randomUUID() },
    writeOptions(write),
  );
}

export function markRead(write: Write, id: string): Promise<unknown> {
  return request<unknown>("PATCH", `/conversations/${encodeURIComponent(id)}/read-state`, {}, writeOptions(write));
}

/*
 * sendTyping tells teammates viewing the conversation that the person
 * is writing a reply, or with "stop" that they stopped.
 */
export function sendTyping(workspace: string, id: string, state: "start" | "stop"): Promise<unknown> {
  return request<unknown>(
    "POST",
    `/conversations/${encodeURIComponent(id)}/typing-indicators`,
    { state, composer: "reply" },
    { workspace },
  );
}
