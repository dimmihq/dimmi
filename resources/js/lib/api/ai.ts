/*
 * The agent's API: the agent's settings, the provider connections it
 * runs on, the month's usage, a conversation's runs and the playground.
 * Every call names the workspace in the Dimmi-Workspace header, and
 * every id is a public one (aia_, aik_, team_, tag_, conv_, air_, ais_).
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

/* The values the server accepts for the agent's fixed-choice settings. */
export const AI_TONES = ["friendly", "professional", "concise"] as const;
export const AI_SCHEDULES = ["always", "outside_office_hours", "inside_office_hours"] as const;
export const AI_ASSIGNED_BEHAVIORS = ["skip_when_human_active", "never_when_assigned", "always"] as const;
export const AI_UNKNOWN_BEHAVIORS = ["offer_handoff", "handoff", "say_unknown"] as const;
export const AI_CONTACT_CONTEXTS = ["none", "basic"] as const;
export const AI_CHANNELS = ["widget", "email", "api"] as const;
export const AI_HANDOFF_MODES = ["full", "triage_only"] as const;

export type AiTone = (typeof AI_TONES)[number];
export type AiSchedule = (typeof AI_SCHEDULES)[number];
export type AiAssignedBehavior = (typeof AI_ASSIGNED_BEHAVIORS)[number];
export type AiUnknownBehavior = (typeof AI_UNKNOWN_BEHAVIORS)[number];
export type AiContactContext = (typeof AI_CONTACT_CONTEXTS)[number];
export type AiChannel = (typeof AI_CHANNELS)[number];
export type AiHandoffMode = (typeof AI_HANDOFF_MODES)[number];

export interface AiModelEntry {
  id: string;
  display_name: string;
  family: "anthropic" | "openai" | "openweight";
  suggested: string[];
  context_tokens: number;
  input_per_mtok: string | null;
  output_per_mtok: string | null;
  cached_input_per_mtok: string | null;
}

export interface AiProviderDescriptor {
  id: string;
  name: string;
  auth_type: "managed" | "api_key" | "oauth" | "none";
  key_required: boolean;
  docs_url: string;
  available: boolean;
}

export interface AiAgent {
  id: string;
  object: "ai_agent";
  name: string;
  enabled: boolean;
  product_name: string | null;
  tone: AiTone;
  guidance: string;
  enabled_channels: string[];
  schedule: AiSchedule;
  office_hours: Record<string, unknown> | null;
  assigned_behavior: AiAssignedBehavior;
  answer_key_id: string | null;
  answer_model: string | null;
  fallback_key_id: string | null;
  fallback_model: string | null;
  max_output_tokens: number;
  max_ai_replies: number;
  unknown_behavior: AiUnknownBehavior;
  grounding_mode: "citations" | "verify";
  require_citations: boolean;
  languages: string[];
  default_language: string;
  streaming: boolean;
  include_contact_context: string;
  pii_redaction: boolean;
  link_allowlist: string[];
  handoff_team_id: string | null;
  raise_priority_on_handoff: boolean;
  /* full answers and resolves; triage_only sorts the request and hands it to the team. */
  handoff_mode: AiHandoffMode;
  /* Public tag ids: a conversation tagged with one always goes to the team. */
  always_handoff_tag_ids: string[];
  monthly_spend_cap_micros: number | null;
  actions_enabled: boolean;
  updated_at: string;
}

export interface AiProviderKey {
  id: string;
  object: "ai_provider_key";
  name: string;
  provider: string;
  base_url: string | null;
  region: string | null;
  auth_type: string;
  key_hint: string | null;
  status: "unverified" | "valid" | "invalid" | "error";
  status_reason: string | null;
  models: string[];
  models_count: number;
  source: string;
  last_validated_at: string | null;
  last_used_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface AgentView {
  agent: AiAgent;
  keys: AiProviderKey[];
  providers: AiProviderDescriptor[];
  catalog: Record<string, AiModelEntry[]>;
  ready: boolean;
  readiness_errors: string[];
  managed: boolean;
  prompt_version: string;
}

export interface AiUsage {
  period_start: string;
  cost_micros: number;
  input_tokens: number;
  cached_input_tokens: number;
  output_tokens: number;
  runs_count: number;
  answered_count: number;
  unknown_count: number;
  handoff_count: number;
  silent_count: number;
  monthly_spend_cap_micros: number | null;
  capped_at: string | null;
  daily: { day: string; cost_micros: number; runs_count: number }[];
  by_model: { model: string; cost_micros: number; calls: number }[];
  by_outcome: { outcome: string; runs_count: number }[];
}

export interface AiRunStep {
  id: string;
  seq: number;
  kind: string;
  provider: string | null;
  model: string | null;
  status: string;
  error_kind: string | null;
  latency_ms: number | null;
  input_tokens: number;
  output_tokens: number;
  cost_micros: number;
  /* Present only for those allowed to read payloads, and null once retention passes. */
  request: unknown;
  response: unknown;
}

export interface AiRun {
  id: string;
  conversation_id: string | null;
  kind: string;
  status: string;
  outcome: string | null;
  skip_reason: string | null;
  handoff_reason: string | null;
  language: string | null;
  answer_message_id: string | null;
  input_tokens: number;
  output_tokens: number;
  cost_micros: number;
  latency_ms: number | null;
  first_token_ms: number | null;
  prompt_version: string;
  flags: string[];
  error: { code: string; message: string | null } | null;
  feedback: "helped" | "not_helped" | null;
  steps?: AiRunStep[];
  payloads_visible?: boolean;
  created_at: string;
  completed_at: string | null;
}

export interface AiPlaygroundAnswer {
  object: "ai_playground_answer";
  run_id: string | null;
  outcome: string;
  classification: { intent: string; language: string; query: string };
  retrieved: { n: number; title: string; url: string; similarity: number; used: boolean; text: string }[];
  answer: { body: string; citations: { n: number; title: string; url: string }[] };
  usage: { input_tokens: number; cached_input_tokens: number; output_tokens: number; cost_micros: number };
  timing: { first_token_ms: number; latency_ms: number };
}

export function aiKey(workspace: string) {
  return ["workspace", workspace, "ai"];
}

export function agentQuery(workspace: string) {
  return queryOptions({
    queryKey: [...aiKey(workspace), "agent"],
    queryFn: () => request<AgentView>("GET", "/ai/agent", undefined, { workspace }),
  });
}

export function aiKeysQuery(workspace: string) {
  return queryOptions({
    queryKey: [...aiKey(workspace), "keys"],
    queryFn: () => request<AiProviderKey[]>("GET", "/ai/keys", undefined, { workspace }),
  });
}

/* period is a month as YYYY-MM; leave it out for the current month. */
export function aiUsageQuery(workspace: string, period?: string) {
  const query = period ? `?period=${encodeURIComponent(period)}` : "";
  return queryOptions({
    queryKey: [...aiKey(workspace), "usage", period ?? "current"],
    queryFn: () => request<AiUsage>("GET", `/ai/usage${query}`, undefined, { workspace }),
  });
}

export function aiRunsQuery(workspace: string, conversation: string) {
  return queryOptions({
    queryKey: [...aiKey(workspace), "runs", conversation],
    queryFn: () => request<AiRun[]>("GET", `/conversations/${conversation}/ai/runs`, undefined, { workspace }),
  });
}

export function aiRunQuery(workspace: string, conversation: string, run: string) {
  return queryOptions({
    queryKey: [...aiKey(workspace), "runs", conversation, run],
    queryFn: () => request<AiRun>("GET", `/conversations/${conversation}/ai/runs/${run}`, undefined, { workspace }),
  });
}

/*
 * AgentPatch changes only the fields it carries. Nullable fields
 * (product_name, the models, the key ids, handoff_team_id,
 * monthly_cap_usd) clear when sent as null or ""; a cap of 0 also means
 * no cap.
 */
export interface AgentPatch {
  name?: string;
  enabled?: boolean;
  tone?: string;
  guidance?: string;
  product_name?: string | null;
  enabled_channels?: string[];
  schedule?: string;
  answer_key_id?: string | null;
  answer_model?: string | null;
  fallback_key_id?: string | null;
  fallback_model?: string | null;
  monthly_cap_usd?: number | null;
  max_ai_replies?: number;
  require_citations?: boolean;
  pii_redaction?: boolean;
  languages?: string[];
  default_language?: string;
  unknown_behavior?: string;
  assigned_behavior?: string;
  streaming?: boolean;
  include_contact_context?: string;
  /* Hosts the agent may link to; "*.example.com" covers every subdomain. */
  link_allowlist?: string[];
  handoff_team_id?: string | null;
  raise_priority_on_handoff?: boolean;
  handoff_mode?: AiHandoffMode;
  /* Replaces the whole list; at most 50 of the workspace's tags. */
  always_handoff_tag_ids?: string[];
}

export function updateAgent(workspace: string, body: AgentPatch) {
  return request<AgentView>("PATCH", "/ai/agent", body, { workspace });
}

export interface NewAiKey {
  name: string;
  provider: string;
  base_url?: string;
  api_key?: string;
  config?: Record<string, unknown>;
}

export function createAiKey(workspace: string, body: NewAiKey) {
  return request<AiProviderKey>("POST", "/ai/keys", body, { workspace });
}

export function updateAiKey(workspace: string, keyId: string, body: Partial<NewAiKey>) {
  return request<AiProviderKey>("PATCH", `/ai/keys/${keyId}`, body, { workspace });
}

export function deleteAiKey(workspace: string, keyId: string) {
  return request<void>("DELETE", `/ai/keys/${keyId}`, undefined, { workspace });
}

export function testAiKey(workspace: string, keyId: string) {
  return request<AiProviderKey>("POST", `/ai/keys/${keyId}/test`, undefined, { workspace });
}

export function askPlayground(workspace: string, body: { message: string; conversation?: string }) {
  return request<AiPlaygroundAnswer>("POST", "/ai/playground", body, { workspace });
}

/*
 * ChatGPTStart is a started subscription sign-in: a device code to type
 * on OpenAI's page, or a browser address whose final address the person
 * pastes back.
 */
export type ChatGPTStart =
  | {
      method: "device";
      state: string;
      url: string;
      user_code: string;
      interval_seconds: number;
      expires_at: string;
    }
  | { method: "browser"; state: string; url: string; redirect_uri: string; expires_at: string };

/*
 * ChatGPTProgress is where a sign-in stands: still waiting for approval,
 * or connected as a new connection.
 */
export type ChatGPTProgress =
  | { status: "pending"; interval_seconds?: number }
  | { status: "connected"; key_id: string; account_id: string };

export function startChatGPTConnect(workspace: string, method: "device" | "browser" = "device") {
  return request<ChatGPTStart>("POST", "/ai/chatgpt/connect", { method }, { workspace });
}

/* code is the browser sign-in's final address, or its code; a device sign-in sends none. */
export function completeChatGPTConnect(workspace: string, body: { state: string; code?: string }) {
  return request<ChatGPTProgress>("POST", "/ai/chatgpt/callback", body, { workspace });
}
