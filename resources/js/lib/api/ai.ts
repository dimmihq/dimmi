/*
 * The agent's API: the agent's settings, the provider connections it
 * runs on, and the month's usage. Every call names the workspace in the
 * Dimmi-Workspace header.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

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
  docs_url?: string;
  available: boolean;
}

export interface AiAgent {
  id: string;
  name: string;
  enabled: boolean;
  product_name: string | null;
  tone: string;
  guidance: string;
  enabled_channels: string[];
  schedule: string;
  assigned_behavior: string;
  answer_key_id: string | null;
  answer_model: string | null;
  fallback_key_id: string | null;
  fallback_model: string | null;
  max_ai_replies: number;
  unknown_behavior: string;
  require_citations: boolean;
  default_language: string;
  streaming: boolean;
  pii_redaction: boolean;
  monthly_spend_cap_micros: number | null;
  actions_enabled: boolean;
  include_contact_context: string;
  office_hours: Record<string, unknown> | null;
  handoff: { target_team_id?: string; raise_priority?: boolean };
}

export interface AiProviderKey {
  id: string;
  name: string;
  provider: string;
  base_url: string | null;
  auth_type: string;
  key_hint: string | null;
  status: string;
  status_reason: string | null;
  models: { id: string }[];
  last_validated_at: string | null;
  last_used_at: string | null;
  created_at: string;
}

export interface AgentView {
  agent: AiAgent;
  keys: AiProviderKey[];
  providers: AiProviderDescriptor[];
  catalog: Record<string, AiModelEntry[]>;
  ready: boolean;
  readiness_errors: string[];
  managed: boolean;
}

export interface AiUsage {
  period_start: string;
  cost_micros: number;
  input_tokens: number;
  output_tokens: number;
  runs_count: number;
  answered_count: number;
  unknown_count: number;
  handoff_count: number;
  capped_at: string | null;
  daily: { day: string; cost_micros: number; runs_count: number }[];
  by_model: { model: string; cost_micros: number; calls: number }[];
  by_outcome: { outcome: string; runs_count: number }[];
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
    queryFn: () => request<{ data: AiProviderKey[] }>("GET", "/ai/keys", undefined, { workspace }),
  });
}

export function aiUsageQuery(workspace: string) {
  return queryOptions({
    queryKey: [...aiKey(workspace), "usage"],
    queryFn: () => request<AiUsage>("GET", "/ai/usage", undefined, { workspace }),
  });
}

export interface AgentPatch {
  name?: string;
  enabled?: boolean;
  tone?: string;
  guidance?: string;
  product_name?: string;
  enabled_channels?: string[];
  schedule?: string;
  answer_key_id?: string;
  answer_model?: string;
  fallback_key_id?: string;
  fallback_model?: string;
  monthly_cap_usd?: number;
  max_ai_replies?: number;
  require_citations?: boolean;
  pii_redaction?: boolean;
  languages?: string[];
  default_language?: string;
  unknown_behavior?: string;
  assigned_behavior?: string;
  streaming?: boolean;
  include_contact_context?: string;
  handoff_team_id?: string;
  raise_priority_on_handoff?: boolean;
}

export function updateAgent(workspace: string, body: AgentPatch) {
  return request<AiAgent>("PATCH", "/ai/agent", body, { workspace });
}

export interface NewAiKey {
  name: string;
  provider: string;
  base_url?: string;
  api_key?: string;
}

export function createAiKey(workspace: string, body: NewAiKey) {
  return request<AiProviderKey>("POST", "/ai/keys", body, { workspace });
}

export function updateAiKey(workspace: string, keyId: string, body: Partial<NewAiKey>) {
  return request<AiProviderKey>("PATCH", `/ai/keys/${keyId}`, body, { workspace });
}

export function deleteAiKey(workspace: string, keyId: string) {
  return request("DELETE", `/ai/keys/${keyId}`, undefined, { workspace });
}

export function testAiKey(workspace: string, keyId: string) {
  return request<{ status: string; reason?: string }>("POST", `/ai/keys/${keyId}/test`, undefined, { workspace });
}

export function startChatGPTConnect(workspace: string) {
  return request<{ authorize_url: string; expires_at: string }>("POST", "/ai/chatgpt/connect", undefined, {
    workspace,
  });
}
