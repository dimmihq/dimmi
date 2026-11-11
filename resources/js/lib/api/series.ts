/*
 * Series: automated journeys people, or whole accounts, travel through
 * over days or weeks. The builder saves the whole working copy with the
 * lock version it read; publishing makes it the version that runs.
 * Reading takes series:read, building series:write, and publishing,
 * pausing and adding people series:publish, which the server checks.
 */
import { keepPreviousData, queryOptions } from "@tanstack/react-query";

import type { AudienceRules } from "./audiences";
import type { BroadcastCounts } from "./broadcasts";
import { request } from "./client";

export type SeriesStatus = "draft" | "live" | "paused" | "archived";

export type SeriesSubject = "contact" | "company";

export type NodeKind =
  | "entry"
  | "email"
  | "chat"
  | "post"
  | "slack"
  | "wait"
  | "wait_for"
  | "condition"
  | "split"
  | "ai_path"
  | "tag"
  | "untag"
  | "set_attribute"
  | "topic"
  | "webhook"
  | "exit";

export interface Engagement {
  node: string;
  action: "opened" | "clicked" | "replied";
}

/*
 * NodeSettings holds every kind's settings; each kind reads its own.
 */
export interface NodeSettings {
  subject?: string;
  preview?: string;
  body?: string;
  track_clicks?: boolean;
  recipients?: AudienceRules;
  mode?: "duration" | "time" | "attribute";
  amount?: number;
  unit?: "minutes" | "hours" | "days" | "weeks";
  at?: string;
  weekdays?: number[];
  attribute?: string;
  offset_days?: number;
  rules?: AudienceRules;
  event?: string;
  engagement?: Engagement;
  paths?: { id: string; label: string; rules?: AudienceRules; engagement?: Engagement; description?: string }[];
  variants?: { id: string; label: string; weight: number }[];
  winner?: string;
  prompt?: string;
  tag_ids?: string[];
  key?: string;
  value?: unknown;
  topic_id?: string;
  subscribed?: boolean;
  url?: string;
}

export interface SeriesNode {
  id: string;
  kind: NodeKind;
  settings?: NodeSettings;
  position?: { x: number; y: number };
}

export interface SeriesEdge {
  from: string;
  path: string;
  to: string;
}

export interface SeriesSpec {
  entry: {
    audience_id?: string;
    rules?: AudienceRules;
    event?: string;
    include_existing?: boolean;
    reentry: { kind: "once" | "again"; cooldown_days?: number };
  };
  exit: { rules?: AudienceRules; on_goal?: boolean; on_reply?: boolean; on_unsubscribe?: boolean };
  goal?: { event?: string; rules?: AudienceRules; days: number } | null;
  quiet_hours?: boolean;
  nodes: SeriesNode[];
  edges: SeriesEdge[];
}

export interface SeriesProblem {
  code: string;
  message: string;
  node_id?: string;
  field?: string;
}

export interface Series {
  id: string;
  name: string;
  description: string;
  subject: SeriesSubject;
  status: SeriesStatus;
  spec: SeriesSpec;
  problems: SeriesProblem[];
  live_version_id: string | null;
  has_unpublished_changes: boolean;
  lock_version: number;
  topic_id: string | null;
  service: boolean;
  email_address_id: string | null;
  sender_id: string | null;
  slack_as: "bot" | "member";
  reply_team_id: string | null;
  reply_member_id: string | null;
  ai_replies: boolean;
  published_at: string | null;
  paused_at: string | null;
  archived_at: string | null;
  totals?: { entered: number; active: number; finished: number; exited: number; goal: number };
  created_at: string;
  updated_at: string;
}

/*
 * SeriesSave is a whole save: everything the builder holds, over the
 * lock version it read.
 */
export interface SeriesSave {
  lock_version: number;
  name: string;
  description?: string;
  spec?: SeriesSpec;
  topic_id?: string;
  service?: boolean;
  email_address_id?: string;
  sender_id?: string;
  slack_as?: "bot" | "member";
  reply_team_id?: string;
  reply_member_id?: string;
  ai_replies?: boolean;
}

export interface SeriesStepReport {
  reached: number;
  waiting: number;
  paths: Record<string, number>;
  outcomes: Record<string, number>;
  message?: Partial<BroadcastCounts>;
  variants?: { path: string; entered: number; goal: number }[];
}

export interface SeriesReport {
  entered: number;
  active: number;
  finished: number;
  exited: number;
  goal: number;
  reasons: { reason: string; count: number }[];
  days: { day: string; entered: number; goal: number }[];
  steps: Record<string, SeriesStepReport>;
}

export interface SeriesJourney {
  id: string;
  series_id: string;
  series_name?: string;
  contact_id: string | null;
  contact_name?: string;
  contact_email?: string;
  company_id: string | null;
  company_name?: string;
  status: "active" | "waiting" | "finished" | "exited";
  node_id: string;
  wake_at: string | null;
  entered_at: string;
  entered_by: string;
  goal_at: string | null;
  left_at: string | null;
  end_reason: string | null;
  test: boolean;
}

export interface SeriesStep {
  node_id: string;
  kind: NodeKind;
  status: string;
  path_id: string | null;
  outcome: string | null;
  detail: Record<string, unknown>;
  entered_at: string;
  left_at: string | null;
}

export interface SeriesTrace extends SeriesJourney {
  steps: SeriesStep[] | null;
}

export interface SeriesVersion {
  id: string;
  number: number;
  note: string;
  published_by: string | null;
  created_at: string;
}

const path = (id: string) => `/series/${encodeURIComponent(id)}`;

export function seriesKey(workspace: string) {
  return ["workspace", workspace, "series"] as const;
}

export function seriesListQuery(workspace: string, status?: SeriesStatus) {
  const query = new URLSearchParams({ limit: "100" });
  if (status) {
    query.set("status", status);
  }

  return queryOptions({
    queryKey: [...seriesKey(workspace), "list", status ?? "all"] as const,
    queryFn: () => request<Series[]>("GET", `/series?${query}`, undefined, { workspace }),
    placeholderData: keepPreviousData,
  });
}

export function seriesQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: [...seriesKey(workspace), "one", id] as const,
    queryFn: () => request<Series>("GET", path(id), undefined, { workspace }),
    retry: false,
  });
}

export function seriesReportQuery(workspace: string, id: string, live: boolean) {
  return queryOptions({
    queryKey: [...seriesKey(workspace), "one", id, "report"] as const,
    queryFn: () => request<SeriesReport>("GET", `${path(id)}/report`, undefined, { workspace }),
    refetchInterval: live ? 10_000 : false,
  });
}

export function seriesJourneysQuery(workspace: string, id: string, node?: string) {
  const query = new URLSearchParams({ limit: "50" });
  if (node) {
    query.set("node", node);
  }

  return queryOptions({
    queryKey: [...seriesKey(workspace), "one", id, "journeys", node ?? "all"] as const,
    queryFn: () => request<SeriesJourney[]>("GET", `${path(id)}/enrollments?${query}`, undefined, { workspace }),
    placeholderData: keepPreviousData,
  });
}

export function seriesTraceQuery(workspace: string, id: string, journey: string) {
  return queryOptions({
    queryKey: [...seriesKey(workspace), "one", id, "trace", journey] as const,
    queryFn: () => request<SeriesTrace>("GET", `${path(id)}/enrollments/${journey}`, undefined, { workspace }),
    refetchInterval: 3_000,
  });
}

export function seriesVersionsQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: [...seriesKey(workspace), "one", id, "versions"] as const,
    queryFn: () => request<SeriesVersion[]>("GET", `${path(id)}/versions`, undefined, { workspace }),
  });
}

export function createSeries(
  workspace: string,
  series: { name: string; subject: SeriesSubject; spec?: SeriesSpec },
): Promise<Series> {
  return request<Series>("POST", "/series", series, { workspace });
}

export function saveSeries(workspace: string, id: string, save: SeriesSave): Promise<Series> {
  return request<Series>("PATCH", path(id), save, { workspace });
}

export function deleteSeries(workspace: string, id: string): Promise<void> {
  return request<void>("DELETE", path(id), undefined, { workspace });
}

export function publishSeries(workspace: string, id: string, note: string, moves?: Record<string, string>) {
  return request<Series>("POST", `${path(id)}/publications`, { note, moves }, { workspace });
}

export type SeriesAction = "pause" | "resume" | "archive" | "duplicate";

export function actOnSeries(workspace: string, id: string, action: SeriesAction): Promise<Series> {
  return request<Series>("POST", `${path(id)}/${action}`, undefined, { workspace });
}

export function trySeries(workspace: string, id: string): Promise<SeriesTrace> {
  return request<SeriesTrace>("POST", `${path(id)}/tests`, {}, { workspace });
}

export function skipSeriesWait(workspace: string, id: string, journey: string): Promise<void> {
  return request<void>("POST", `${path(id)}/enrollments/${journey}/skip`, {}, { workspace });
}

export function removeFromSeries(workspace: string, id: string, journey: string): Promise<void> {
  return request<void>("DELETE", `${path(id)}/enrollments/${journey}`, undefined, { workspace });
}

export function pickSeriesWinner(workspace: string, id: string, node: string, winner: string): Promise<Series> {
  return request<Series>("POST", `${path(id)}/nodes/${node}/winner`, { path: winner }, { workspace });
}
