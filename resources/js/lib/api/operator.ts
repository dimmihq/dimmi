/*
 * Operator, the team's own agent: its settings, each teammate's threads
 * with it, the questions they ask, the briefs it writes on a
 * conversation, the changes it proposes for a teammate to apply, decline
 * or undo, and what it was asked to remember. An answer streams on the
 * teammate's own channel while the question's request waits; the request
 * then answers with the stored message.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export const OPERATOR_GROUPS = ["conversations", "notes", "contacts", "articles", "procedures", "actions"] as const;
export type OperatorGroup = (typeof OPERATOR_GROUPS)[number];
export type OperatorPolicy = "auto" | "ask" | "off";

export interface OperatorSettings {
  enabled: boolean;
  brief_on_open: boolean;
  triage_on_arrival: boolean;
  instructions: string;
  policies: Record<OperatorGroup, OperatorPolicy>;
  auto_groups: OperatorGroup[];
  model: string | null;
}

export interface OperatorSettingsChange {
  enabled?: boolean;
  brief_on_open?: boolean;
  triage_on_arrival?: boolean;
  instructions?: string;
  policies?: Partial<Record<OperatorGroup, OperatorPolicy>>;
  model?: string;
}

export interface OperatorStep {
  tool: string;
  summary: string;
  ok: boolean;
  draft?: string;
  changeset_id?: string;
}

export interface OperatorMessage {
  id: string;
  role: "user" | "assistant";
  body: string;
  steps: OperatorStep[];
  status: "done" | "stopped" | "failed";
  created_at: string;
}

export type ChangesetStatus = "pending" | "applied" | "declined" | "reverted" | "failed";

export interface OperatorOperation {
  kind: string;
  label: string;
  text?: string;
  conversation_number?: number;
  outcome?: "applied" | "reverted" | "skipped";
  created_id?: string;
}

export interface OperatorChangeset {
  id: string;
  thread_id: string | null;
  message_id: string | null;
  conversation_id: string | null;
  proposed_to: string | null;
  title: string;
  reason: string;
  status: ChangesetStatus;
  auto: boolean;
  error: string | null;
  operations: OperatorOperation[];
  applied_at: string | null;
  reverted_at: string | null;
  created_at: string;
}

export interface OperatorThread {
  id: string;
  conversation_id: string | null;
  title: string;
  last_message_at: string | null;
  created_at: string;
}

export interface OperatorThreadView extends OperatorThread {
  messages: OperatorMessage[];
  changesets: OperatorChangeset[];
}

export interface OperatorAnswer {
  asked: OperatorMessage | null;
  message: OperatorMessage;
  changesets: OperatorChangeset[];
}

export interface OperatorMemory {
  id: string;
  body: string;
  created_at: string;
}

export type OperatorPurpose = "ask" | "brief" | "triage";

export interface OperatorUsageRun {
  id: string;
  purpose: OperatorPurpose;
  status: "running" | "completed" | "failed" | "skipped";
  member_name: string;
  thread_title: string;
  conversation_id: string | null;
  conversation_number: number | null;
  cost_micros: number;
  latency_ms: number | null;
  created_at: string;
}

export interface OperatorUsage {
  period_start: string;
  cost_micros: number;
  runs_count: number;
  failed_count: number;
  input_tokens: number;
  output_tokens: number;
  agent_cost_micros: number;
  monthly_spend_cap_micros: number | null;
  daily: { day: string; cost_micros: number; runs_count: number }[];
  by_purpose: { purpose: OperatorPurpose; runs_count: number; cost_micros: number }[];
  by_member: { member_id: string; name: string; runs_count: number; cost_micros: number }[];
  by_model: { model: string; cost_micros: number; calls: number }[];
  changesets: { status: ChangesetStatus; count: number }[];
  recent: OperatorUsageRun[];
}

export const operatorKeys = {
  settings: (workspace: string) => ["workspace", workspace, "operator", "settings"] as const,
  threads: (workspace: string) => ["workspace", workspace, "operator", "threads"] as const,
  thread: (workspace: string, id: string) => ["workspace", workspace, "operator", "thread", id] as const,
  conversation: (workspace: string, conversation: string) =>
    ["workspace", workspace, "operator", "conversation", conversation] as const,
  changesets: (workspace: string, status: string) =>
    ["workspace", workspace, "operator", "changesets", status] as const,
  memories: (workspace: string) => ["workspace", workspace, "operator", "memories"] as const,
  usage: (workspace: string) => ["workspace", workspace, "operator", "usage"] as const,
};

export function operatorUsageQuery(workspace: string) {
  return queryOptions({
    queryKey: operatorKeys.usage(workspace),
    queryFn: () => request<OperatorUsage>("GET", "/operator/usage", undefined, { workspace }),
  });
}

export function operatorSettingsQuery(workspace: string) {
  return queryOptions({
    queryKey: operatorKeys.settings(workspace),
    queryFn: () => request<OperatorSettings>("GET", "/operator/settings", undefined, { workspace }),
    staleTime: 60_000,
  });
}

export function updateOperatorSettings(workspace: string, change: OperatorSettingsChange) {
  return request<OperatorSettings>("PATCH", "/operator/settings", change, { workspace });
}

/*
 * conversationThreadQuery opens, or reads, the teammate's thread about a
 * conversation.
 */
export function conversationThreadQuery(workspace: string, conversation: string) {
  return queryOptions({
    queryKey: operatorKeys.conversation(workspace, conversation),
    queryFn: () =>
      request<OperatorThreadView>("POST", "/operator/threads", { conversation_id: conversation }, { workspace }),
    staleTime: 30_000,
  });
}

export function operatorThreadQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: operatorKeys.thread(workspace, id),
    queryFn: () =>
      request<OperatorThreadView>("GET", `/operator/threads/${encodeURIComponent(id)}`, undefined, { workspace }),
  });
}

export function askOperator(workspace: string, thread: string, body: string, signal?: AbortSignal) {
  return request<OperatorAnswer>(
    "POST",
    `/operator/threads/${encodeURIComponent(thread)}/messages`,
    { body },
    { workspace, signal },
  );
}

/*
 * briefConversation asks for a brief on one of the teammate's threads
 * about the conversation, the latest when none is named; attach also
 * keeps the brief on the conversation as a note the whole team sees.
 */
export function briefConversation(
  workspace: string,
  conversation: string,
  options: { thread?: string; attach?: boolean; signal?: AbortSignal } = {},
) {
  return request<OperatorThreadView>(
    "POST",
    `/conversations/${encodeURIComponent(conversation)}/operator/brief`,
    { thread_id: options.thread, attach: options.attach },
    { workspace, signal: options.signal },
  );
}

/*
 * conversationThreadsQuery lists the teammate's past threads about a
 * conversation, latest first.
 */
export function conversationThreadsQuery(workspace: string, conversation: string) {
  return queryOptions({
    queryKey: [...operatorKeys.conversation(workspace, conversation), "list"] as const,
    queryFn: () =>
      request<OperatorThread[]>(
        "GET",
        `/operator/threads?filter[conversation]=${encodeURIComponent(conversation)}`,
        undefined,
        { workspace },
      ),
  });
}

/*
 * startConversationThread starts a fresh thread about a conversation.
 */
export function startConversationThread(workspace: string, conversation: string) {
  return request<OperatorThreadView>(
    "POST",
    "/operator/threads",
    { conversation_id: conversation, fresh: true },
    { workspace },
  );
}

export type ChangesetDecision = "applications" | "declines" | "reversions";

export function decideChangeset(workspace: string, id: string, decision: ChangesetDecision) {
  return request<OperatorChangeset>(
    "POST",
    `/operator/changesets/${encodeURIComponent(id)}/${decision}`,
    {},
    { workspace },
  );
}

export function operatorMemoriesQuery(workspace: string) {
  return queryOptions({
    queryKey: operatorKeys.memories(workspace),
    queryFn: () => request<OperatorMemory[]>("GET", "/operator/memories", undefined, { workspace }),
  });
}

export function forgetOperatorMemory(workspace: string, id: string) {
  return request<void>("DELETE", `/operator/memories/${encodeURIComponent(id)}`, undefined, { workspace });
}
