/*
 * Broadcasts: one-off messages to an audience, by email, in the
 * messenger or into customers' Slack channels. A draft changes field by
 * field until it is sent; then the server freezes who gets it and the
 * counts fill in as each message goes out, is opened, clicked or
 * answered. Reading takes broadcasts:read; writing and sending take
 * broadcasts:write, which the server checks.
 */
import { keepPreviousData, queryOptions } from "@tanstack/react-query";

import type { AudienceRules } from "./audiences";
import { request } from "./client";

export type Channel = "email" | "chat" | "post" | "slack";

export type BroadcastStatus = "draft" | "scheduled" | "sending" | "paused" | "sent" | "stopped";

export type PauseReason = "bounce_rate" | "complaint_rate" | "paused_by_teammate";

export type ScheduleKind = "now" | "at" | "local";

/*
 * Counts are what happened to a broadcast's recipients so far. A
 * recipient is a person, or a Slack channel for a Slack broadcast.
 */
export interface BroadcastCounts {
  total: number;
  queued: number;
  sent: number;
  delivered: number;
  opened: number;
  clicked: number;
  replied: number;
  reacted: number;
  reactions: number;
  unsubscribed: number;
  converted: number;
  failed: number;
  bounced: number;
  complained: number;
  suppressed: number;
  skipped: number;
  recalled: number;
}

/*
 * Goal is what counts as the broadcast working: the person doing the
 * event within so many days of getting it.
 */
export interface BroadcastGoal {
  event: string;
  days: number;
}

export interface Broadcast {
  id: string;
  name: string;
  channel: Channel;
  status: BroadcastStatus;
  pause_reason: PauseReason | null;
  audience_id: string | null;
  rules: AudienceRules | null;
  room_ids: string[] | null;
  topic_id: string | null;
  service: boolean;
  subject: string;
  preview: string;
  body: string;
  sender_id: string | null;
  email_address_id: string | null;
  slack_as: "bot" | "member";
  reply_team_id: string | null;
  reply_member_id: string | null;
  ai_replies: boolean;
  track_clicks: boolean;
  skip_frequency_cap: boolean;
  schedule_kind: ScheduleKind;
  scheduled_at: string | null;
  goal: BroadcastGoal | null;
  counts: BroadcastCounts | null;
  started_at: string | null;
  finished_at: string | null;
  created_at: string;
  updated_at: string;
}

/*
 * BroadcastChanges is what a save sends: only the fields that changed.
 * An empty id clears the reference it names.
 */
export interface BroadcastChanges {
  name?: string;
  audience_id?: string;
  rules?: AudienceRules | null;
  room_ids?: string[];
  topic_id?: string;
  service?: boolean;
  subject?: string;
  preview?: string;
  body?: string;
  sender_id?: string;
  email_address_id?: string;
  slack_as?: "bot" | "member";
  reply_team_id?: string;
  reply_member_id?: string;
  ai_replies?: boolean;
  track_clicks?: boolean;
  skip_frequency_cap?: boolean;
  schedule?: { kind: ScheduleKind; at?: string };
  goal?: BroadcastGoal | null;
}

/*
 * BroadcastEstimate is who would get the broadcast if it went out now,
 * and how many are left out for each reason. capped says the count
 * stopped early.
 */
export interface BroadcastEstimate {
  reach: number;
  skipped: Record<string, number>;
  capped: boolean;
}

export interface BroadcastReason {
  reason: string;
  count: number;
}

export interface BroadcastLink {
  n: number;
  url: string;
  clicks: number;
}

export interface BroadcastCompany {
  company_id: string;
  name: string;
  total: number;
  delivered: number;
  opened: number;
  clicked: number;
  replied: number;
  reactions: number;
  failed: number;
}

export interface BroadcastReport {
  broadcast: Broadcast;
  counts: BroadcastCounts;
  reasons: BroadcastReason[] | null;
  links: BroadcastLink[] | null;
  companies: BroadcastCompany[] | null;
  waiting: BroadcastReason[] | null;
}

export type RecipientStatus =
  | "queued"
  | "sent"
  | "delivered"
  | "failed"
  | "bounced"
  | "complained"
  | "suppressed"
  | "skipped"
  | "recalled";

/*
 * RecipientFilter narrows the recipients by their status, or by what
 * they did: opened, clicked, replied or unsubscribed.
 */
export type RecipientFilter = RecipientStatus | "opened" | "clicked" | "replied" | "unsubscribed";

export interface BroadcastRecipient {
  id: string;
  status: RecipientStatus;
  reason: string | null;
  contact_id: string | null;
  contact_name: string;
  contact_email: string;
  company_id: string | null;
  company_name: string;
  room_id: string | null;
  room_name: string;
  address: string;
  conversation_id: string | null;
  sent_at: string | null;
  delivered_at: string | null;
  opened_at: string | null;
  clicked_at: string | null;
  replied_at: string | null;
  reactions: number;
  unsubscribed_at: string | null;
  converted_at: string | null;
}

export interface BroadcastTopic {
  id: string;
  name: string;
  description: string;
  consent: "opt_in" | "opt_out";
}

/*
 * BroadcastFilter narrows the list by status, channel and a search of
 * names.
 */
export interface BroadcastFilter {
  status?: BroadcastStatus;
  channel?: Channel;
  q?: string;
}

export const recipientsPerPage = 50;

const path = (id: string) => `/broadcasts/${encodeURIComponent(id)}`;

function params(pairs: [string, string | undefined][]): string {
  const search = new URLSearchParams();
  for (const [key, value] of pairs) {
    if (value) {
      search.set(key, value);
    }
  }

  return search.toString();
}

export function broadcastsKey(workspace: string) {
  return ["workspace", workspace, "broadcasts"] as const;
}

export function broadcastsQuery(workspace: string, filter: BroadcastFilter = {}) {
  const query = params([
    ["status", filter.status],
    ["channel", filter.channel],
    ["q", filter.q],
    ["limit", "100"],
  ]);

  return queryOptions({
    queryKey: [...broadcastsKey(workspace), "list", filter] as const,
    queryFn: () => request<Broadcast[]>("GET", `/broadcasts?${query}`, undefined, { workspace }),
    placeholderData: keepPreviousData,
  });
}

export function broadcastQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: [...broadcastsKey(workspace), "one", id] as const,
    queryFn: () => request<Broadcast>("GET", path(id), undefined, { workspace }),
    retry: false,
  });
}

export function broadcastEstimateQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: [...broadcastsKey(workspace), "one", id, "estimate"] as const,
    queryFn: () => request<BroadcastEstimate>("GET", `${path(id)}/estimate`, undefined, { workspace }),
    placeholderData: keepPreviousData,
  });
}

export function broadcastReportQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: [...broadcastsKey(workspace), "one", id, "report"] as const,
    queryFn: () => request<BroadcastReport>("GET", `${path(id)}/report`, undefined, { workspace }),
  });
}

export function broadcastRecipientsQuery(
  workspace: string,
  id: string,
  filter: { status?: RecipientFilter; company?: string; page: number },
) {
  const query = params([
    ["status", filter.status],
    ["company_id", filter.company],
    ["limit", String(recipientsPerPage)],
    ["offset", String((filter.page - 1) * recipientsPerPage)],
  ]);

  return queryOptions({
    queryKey: [...broadcastsKey(workspace), "one", id, "recipients", filter] as const,
    queryFn: () => request<BroadcastRecipient[]>("GET", `${path(id)}/recipients?${query}`, undefined, { workspace }),
    placeholderData: keepPreviousData,
  });
}

export function broadcastTopicsQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "broadcast-topics"],
    queryFn: () => request<BroadcastTopic[]>("GET", "/broadcast-topics", undefined, { workspace }),
  });
}

export function createBroadcast(workspace: string, name: string, channel: Channel): Promise<Broadcast> {
  return request<Broadcast>("POST", "/broadcasts", { name, channel }, { workspace });
}

export function saveBroadcast(workspace: string, id: string, changes: BroadcastChanges): Promise<Broadcast> {
  return request<Broadcast>("PATCH", path(id), changes, { workspace });
}

export function deleteBroadcast(workspace: string, id: string): Promise<void> {
  return request<void>("DELETE", path(id), undefined, { workspace });
}

export type BroadcastAction = "send" | "pause" | "resume" | "stop" | "cancel" | "duplicate";

/*
 * actOnBroadcast sends, pauses, resumes, stops, unschedules or
 * duplicates a broadcast, returning it as it now is, or the copy.
 */
export function actOnBroadcast(workspace: string, id: string, action: BroadcastAction): Promise<Broadcast> {
  return request<Broadcast>("POST", `${path(id)}/${action}`, undefined, { workspace });
}

/*
 * testBroadcast sends one test: an email to an address, or a Slack post
 * into one channel.
 */
export function testBroadcast(workspace: string, id: string, to: { email: string } | { room_id: string }) {
  return request<void>("POST", `${path(id)}/tests`, to, { workspace });
}

export function createBroadcastTopic(
  workspace: string,
  topic: { name: string; description?: string; consent?: BroadcastTopic["consent"] },
): Promise<BroadcastTopic> {
  return request<BroadcastTopic>("POST", "/broadcast-topics", topic, { workspace });
}

export function updateBroadcastTopic(
  workspace: string,
  id: string,
  topic: { name: string; description?: string; archived?: boolean },
): Promise<BroadcastTopic> {
  return request<BroadcastTopic>("PATCH", `/broadcast-topics/${encodeURIComponent(id)}`, topic, { workspace });
}

/*
 * BroadcastProgress is what the server says of a broadcast while it
 * goes out: its status, why it paused and its latest counts.
 */
export interface BroadcastProgress {
  id: string;
  object: "broadcast";
  status: BroadcastStatus;
  pause_reason?: PauseReason | null;
  counts?: BroadcastCounts;
}

/*
 * EmailPreferences is what a person who got a broadcast email chose:
 * the topics they hear about, or nothing at all. The token in their
 * unsubscribe link names them, so no sign-in is needed.
 */
export interface EmailPreferences {
  object: string;
  workspace: string;
  email: string;
  unsubscribed_all: boolean;
  topics: { id: string; name: string; description: string; subscribed: boolean }[] | null;
}

const preferencesPath = (token: string) => `/broadcast-preferences/${encodeURIComponent(token)}`;

export function emailPreferencesQuery(token: string) {
  return queryOptions({
    queryKey: ["broadcast-preferences", token],
    queryFn: () => request<EmailPreferences>("GET", preferencesPath(token)),
    retry: false,
  });
}

export function saveEmailPreferences(
  token: string,
  choice: { topics: string[]; unsubscribe_all: boolean },
): Promise<EmailPreferences> {
  return request<EmailPreferences>("PUT", preferencesPath(token), choice);
}
