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

import type { ActionCallState, MessageActionCall } from "./ai-actions";
import { type DownloadedFile, request, requestFile } from "./client";
import type { Tag } from "./tags";

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
  timezone?: string | null;
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
  tags: Tag[];
  viewers?: Viewer[];
  ai_state?: AiState;
  ai_reply_count?: number;
  ai_handoff_reason?: string | null;
  /* How many of the agent's action calls wait for a teammate to approve them. */
  ai_pending_approvals?: number;
  created_at: string;
  updated_at: string;
}

/*
 * AiState is where the agent stands in a conversation: not involved,
 * answering, or handed over to the team.
 */
export type AiState = "off" | "active" | "handed_off";

/*
 * AiFilter narrows a list by the agent's part: any conversation it
 * touched, those it still answers, or those it handed over.
 */
export type AiFilter = "any" | "active" | "handed_off";

/*
 * TimelineEvent is what an event entry says happened: its name, who did
 * it, and what it carried, such as who a conversation went to.
 */
export interface TimelineEvent {
  name: string;
  actor: { type: string; id: string; name: string };
  reason?: string;
  until?: string;
  tags?: string[];
  from?: string;
  to?: { member: Named | null; team: Named | null } | string;
  changes?: Record<string, [unknown, unknown]>;
  /* An ai.action_call_* step: the call, its action, where it then stood and why a failure failed. */
  call_id?: string;
  action?: { id: string; title: string };
  state?: ActionCallState;
  summary?: string;
  error_code?: string | null;
}

/*
 * MentionRef is a teammate or team a note names, with the name it had
 * when the note was written.
 */
export interface MentionRef {
  type: "member" | "team";
  id: string;
  name: string;
}

/*
 * Reaction is one emoji on a note and who reacted with it.
 */
export interface Reaction {
  emoji: string;
  count: number;
  member_ids: string[];
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
  delivery_error: string | null;
  scheduled_send_at?: string | null;
  event: TimelineEvent | null;
  edited_at?: string | null;
  read_at?: string | null;
  mentions?: MentionRef[];
  reactions?: Reaction[];
  deleted_at: string | null;
  deleted_by?: { id: string; name: string } | null;
  /* When the author deleted the message where they wrote it, such as in Slack. Its words stay. */
  source_deleted_at?: string | null;
  created_at: string;
  redactions?: { rule: string; count: number }[];
  attachments?: Attachment[];
  /* The agent's mark on its replies, such as action_call: a change the reply reports. */
  ai?: { action_call?: MessageActionCall } | null;
}

/*
 * Attachment is a file on a message. Its url reads it for a few minutes
 * and is null until the malware scan lets it through; blocked marks a
 * file the scan refused.
 */
export interface Attachment {
  id: string;
  filename: string;
  content_type: string;
  size_bytes: number;
  width: number | null;
  height: number | null;
  url: string | null;
  blocked: boolean;
  content_id?: string | null;
}

/*
 * uploadAttachment sends a file the teammate is about to reply with and
 * returns it, ready to be named by the reply.
 */
export function uploadAttachment(workspace: string, file: File, signal?: AbortSignal): Promise<Attachment> {
  return request<Attachment>("POST", `/attachments?filename=${encodeURIComponent(file.name)}`, file, {
    workspace,
    headers: { "Content-Type": file.type || "application/octet-stream" },
    signal,
  });
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
  settings?: { color: string };
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
  add_tags?: string[];
  remove_tags?: string[];
}

/*
 * ListParams is what one list asks for: the view as filter[view] reads
 * it, the status shown, the order, what the person searched, whether
 * only unread conversations show, the agent's part in them, conditions
 * applied without saving a view, as filter[where] carries them, and how
 * many come back.
 */
export interface ListParams {
  view: string;
  status: StatusFilter;
  sort?: ConversationSort;
  search?: string;
  unread?: boolean;
  ai?: AiFilter;
  where?: string;
  limit?: number;
}

/*
 * conversationsPath is the list's address. Viewers come along with every
 * page, so rows show who has a conversation open.
 */
export function conversationsPath({
  view,
  status,
  sort,
  search = "",
  unread = false,
  ai,
  where,
  limit = 50,
}: ListParams): string {
  const params = new URLSearchParams({ "filter[view]": view, "filter[status]": status, limit: String(limit) });
  if (ai) {
    params.set("filter[ai]", ai);
  }
  if (where) {
    params.set("filter[where]", where);
  }
  params.set("include", "viewers");
  if (unread) {
    params.set("filter[unread]", "true");
  }
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
    queryKey: [
      ...realtimeKeys.conversations(workspace),
      list.view,
      list.status,
      list.sort ?? "",
      list.search ?? "",
      list.unread ?? false,
      list.ai ?? "",
      list.limit ?? 50,
      list.where ?? "",
    ],
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

export function downloadRawEmail(workspace: string, conversation: string, message: string): Promise<DownloadedFile> {
  return requestFile(`${messagePath(conversation, message)}/original/raw`, "message/rfc822", { workspace });
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

export function reply(write: Write, id: string, body: string, then?: Then, files?: string[]): Promise<Message> {
  const after = afterFields(then);
  const attached = files && files.length > 0 ? { attachment_ids: files } : {};

  return request<Message>(
    "POST",
    `/conversations/${encodeURIComponent(id)}/replies`,
    { body, client_message_id: crypto.randomUUID(), delay: true, ...after, ...attached },
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
 * react adds the caller's emoji to a note, or takes it off.
 */
export function react(write: Write, id: string, message: string, emoji: string, remove: boolean): Promise<Message> {
  const path = `/conversations/${encodeURIComponent(id)}/messages/${encodeURIComponent(message)}/reactions`;
  if (remove) {
    return request<Message>("DELETE", `${path}/${encodeURIComponent(emoji)}`, undefined, writeOptions(write));
  }

  return request<Message>("POST", path, { emoji }, writeOptions(write));
}

/*
 * Mentionables are who a note on a conversation may mention.
 */
export interface Mentionables {
  members: { id: string; name: string; avatar_url: string | null; availability: string; can_see: boolean }[];
  teams: { id: string; name: string; member_count: number }[];
}

export function mentionables(workspace: string, conversation: string, query: string): Promise<Mentionables> {
  return request<Mentionables>(
    "GET",
    `/conversations/${encodeURIComponent(conversation)}/mentionables?q=${encodeURIComponent(query)}`,
    undefined,
    { workspace },
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

export function retryDelivery(write: Write, id: string, message: string): Promise<Message> {
  return request<Message>(
    "POST",
    `/conversations/${encodeURIComponent(id)}/messages/${encodeURIComponent(message)}/retry`,
    {},
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

/*
 * GifSearch says whether the composer may search GIFs, and with which
 * key; the browser calls GIPHY itself.
 */
export interface GifSearch {
  enabled: boolean;
  provider: "giphy" | null;
  api_key: string | null;
}

export function gifSearchQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "gif-search"],
    queryFn: () => request<GifSearch>("GET", "/gif-search", undefined, { workspace }),
    staleTime: Number.POSITIVE_INFINITY,
  });
}
