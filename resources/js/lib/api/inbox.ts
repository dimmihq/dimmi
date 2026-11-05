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

import { request } from "./client";

export type ConversationStatus = "open" | "snoozed" | "closed";

/*
 * The priority levels, lowest first.
 */
export const conversationPriorities = ["none", "low", "medium", "high", "urgent"] as const;

export type ConversationPriority = (typeof conversationPriorities)[number];

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

/*
 * Viewer is a teammate who has the conversation open right now.
 */
export interface Viewer {
  id: string;
  name: string;
  avatar_url: string | null;
}

export interface Conversation {
  id: string;
  number: number;
  subject: string | null;
  status: ConversationStatus;
  spam: boolean;
  priority: ConversationPriority;
  unread: boolean;
  waiting_since: string | null;
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
  viewers?: Viewer[];
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
  from?: string;
  to?: { member: Named | null; team: Named | null } | string;
  changes?: Record<string, [unknown, unknown]>;
}

export interface Message {
  id: string;
  conversation_id: string;
  type: "message" | "note" | "event";
  author: Author;
  body: string | null;
  delivery_status: string | null;
  scheduled_send_at?: string | null;
  event: TimelineEvent | null;
  edited_at?: string | null;
  deleted_at: string | null;
  deleted_by?: { id: string; name: string } | null;
  created_at: string;
}

export interface Counts {
  views: {
    mine: { open: number; unread: number; snoozed: number };
    unassigned: { open: number; unread: number };
    all: { open: number; capped: boolean };
    mentions: { unread: number };
    inboxes: Record<string, { open: number }>;
    teams: Record<string, { open: number }>;
    members: Record<string, { open: number }>;
    custom: Record<string, { open: number }>;
  };
}

/*
 * The orders a list can have, as the API names them, with the words the
 * sort control shows. Snooze ends only applies to snoozed lists.
 */
export const conversationSorts = [
  { value: "-last_message_at", label: "Latest activity" },
  { value: "last_message_at", label: "Oldest activity" },
  { value: "waiting_since", label: "Waiting longest" },
  { value: "-created_at", label: "Newest started" },
  { value: "created_at", label: "Oldest started" },
  { value: "-priority", label: "Priority" },
  { value: "-priority,waiting_since", label: "Priority, waiting longest" },
  { value: "snoozed_until", label: "Snooze ends" },
] as const;

export type ConversationSort = (typeof conversationSorts)[number]["value"];

export type StatusFilter = "open" | "snoozed" | "closed" | "any";

/*
 * ViewPreference is how the person likes one view.
 */
export interface ViewPreference {
  sort?: ConversationSort;
  status?: StatusFilter;
  layout?: "list" | "table";
  columns?: string[];
}

export interface InboxPreferences {
  sidebar: Record<string, unknown>;
  views: Record<string, ViewPreference>;
  pinned_attributes: string[];
  updated_at: string | null;
}

export interface InboxSummary {
  id: string;
  name: string;
}

/*
 * ConversationChanges are the fields a teammate changes on a case; the
 * server applies them together, in its own fixed order.
 */
export interface ConversationChanges {
  assignee_id?: string | null;
  status?: ConversationStatus;
  snooze_preset?: "later_today" | "tomorrow" | "next_week" | "one_week";
  snooze_option?: string;
  snoozed_until?: string;
  priority?: ConversationPriority;
  team_id?: string | null;
  unread?: boolean;
}

/*
 * ListParams is what one list asks for: the view as filter[view] reads
 * it, the status shown, the order and what the person searched.
 */
export interface ListParams {
  view: string;
  status: StatusFilter;
  sort?: ConversationSort;
  search?: string;
}

/*
 * conversationsPath is the list's address. Viewers come along with every
 * page, so rows show who has a conversation open.
 */
export function conversationsPath({ view, status, sort, search = "" }: ListParams): string {
  const params = new URLSearchParams({ "filter[view]": view, "filter[status]": status, limit: "50" });
  params.set("include", "viewers");
  if (sort) {
    params.set("sort", sort);
  }
  if (search.trim()) {
    params.set("q", search.trim());
  }

  return `/conversations?${params.toString()}`;
}

export function conversationsQuery(workspace: string, list: ListParams) {
  return queryOptions({
    queryKey: [...realtimeKeys.conversations(workspace), list.view, list.status, list.sort ?? "", list.search ?? ""],
    queryFn: () => request<Conversation[]>("GET", conversationsPath(list), undefined, { workspace }),
  });
}

export function inboxesQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "inboxes"],
    queryFn: () => request<InboxSummary[]>("GET", "/inboxes?limit=100", undefined, { workspace }),
  });
}

export function preferencesQuery(workspace: string) {
  return queryOptions({
    queryKey: realtimeKeys.preferences(workspace),
    queryFn: () => request<InboxPreferences>("GET", "/inbox-preferences", undefined, { workspace }),
  });
}

export function updatePreferences(
  write: Write,
  changes: Partial<Pick<InboxPreferences, "sidebar" | "views" | "pinned_attributes">>,
): Promise<InboxPreferences> {
  return request<InboxPreferences>("PATCH", "/inbox-preferences", changes, writeOptions(write));
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

/*
 * Then is what happens to the conversation once a reply is sent: it
 * keeps its status, closes, or snoozes by one of its snooze options.
 */
export type Then = { kind: "close" } | { kind: "snooze"; option: string };

const thenField = "then";

/*
 * afterFields writes what happens next as the reply's then and snooze
 * fields; the field name comes from a constant because a then key on an
 * object literal reads as a promise to tools.
 */
function afterFields(then: Then | undefined): Record<string, unknown> {
  if (!then) {
    return {};
  }
  const fields: Record<string, unknown> = { [thenField]: then.kind };

  return then.kind === "snooze" ? { ...fields, snooze: { option: then.option } } : fields;
}

export function reply(write: Write, id: string, body: string, then?: Then): Promise<Message> {
  const after = afterFields(then);

  return request<Message>(
    "POST",
    `/conversations/${encodeURIComponent(id)}/replies`,
    { body, client_message_id: crypto.randomUUID(), delay: true, ...after },
    writeOptions(write),
  );
}

/*
 * deleteMessage deletes a message, or undoes the caller's reply still
 * waiting to leave.
 */
export function deleteMessage(write: Write, id: string, message: string): Promise<unknown> {
  return request<unknown>(
    "DELETE",
    `/conversations/${encodeURIComponent(id)}/messages/${encodeURIComponent(message)}`,
    undefined,
    writeOptions(write),
  );
}

/*
 * editNote rewrites one of the caller's notes.
 */
export function editNote(write: Write, id: string, message: string, body: string): Promise<Message> {
  return request<Message>(
    "PATCH",
    `/conversations/${encodeURIComponent(id)}/messages/${encodeURIComponent(message)}`,
    { body },
    writeOptions(write),
  );
}

/*
 * undoReply takes back a reply still waiting to leave and returns its
 * text, for the composer to hold again.
 */
export async function undoReply(write: Write, id: string, message: string): Promise<string> {
  const undone = await request<{ body: string }>(
    "DELETE",
    `/conversations/${encodeURIComponent(id)}/messages/${encodeURIComponent(message)}`,
    undefined,
    writeOptions(write),
  );

  return undone.body;
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

export interface ConversationRef {
  id: string;
  number: number;
  subject: string | null;
  status: ConversationStatus;
}

/*
 * SearchResults are what the quick search found: the conversation a
 * number or id names, conversations with the matching excerpt, and
 * customers.
 */
export interface SearchResults {
  jump: ConversationRef | null;
  conversations: { conversation: ConversationRef; contact: ContactSummary; highlight: string | null }[];
  contacts: ContactSummary[];
}

export function searchQuery(workspace: string, text: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "search", text],
    queryFn: () => request<SearchResults>("GET", `/search?q=${encodeURIComponent(text)}`, undefined, { workspace }),
  });
}

/*
 * SnoozeChoice is one way to snooze a conversation and the moment it
 * ends for the reader.
 */
export interface SnoozeChoice {
  key: string;
  label: string;
  kind: string;
  until: string;
}

export function snoozeChoicesQuery(workspace: string, conversation: string) {
  return queryOptions({
    queryKey: [...realtimeKeys.conversation(workspace, conversation), "snooze-options"],
    queryFn: () =>
      request<SnoozeChoice[]>("GET", `/snooze-options?conversation=${encodeURIComponent(conversation)}`, undefined, {
        workspace,
      }),
  });
}
