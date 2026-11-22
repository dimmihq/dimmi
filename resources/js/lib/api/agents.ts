/*
 * Agent Studio's API: every agent the workspace runs, each with what
 * customers get and its working copy under draft, the templates a new
 * one starts from, where agents can be placed, published versions, the
 * preview and saved simulations. Every id is a public one (aia_, aiav_,
 * aias_, chn_, chr_, inbox_, team_, prc_, act_, ks_, hc_).
 */
import { queryOptions } from "@tanstack/react-query";

import type { AgentView as AiAgentView, AiModelEntry } from "./ai";
import { request } from "./client";
import type {
  ActionMock,
  Assertion,
  ObservedCall,
  ProcedureTrace,
  SimMessage,
  SimPersona,
  SimState,
  TestResult,
} from "./procedures";

export type AgentStatus = "draft" | "live" | "paused";
export type ScopeMode = "all" | "selected";

export interface AgentScope {
  mode: ScopeMode;
  ids: string[];
}

export interface KnowledgeScope {
  mode: ScopeMode;
  help_center_ids: string[];
  source_ids: string[];
}

export interface AgentPlacement {
  channel_connection_ids: string[];
  channel_room_ids: string[];
  inbox_ids: string[];
}

/* Everything a teammate edits on an agent. */
export interface AgentDefinition {
  name: string;
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
  languages: string[];
  default_language: string;
  streaming: boolean;
  include_contact_context: string;
  pii_redaction: boolean;
  link_allowlist: string[];
  handoff_team_id: string | null;
  raise_priority_on_handoff: boolean;
  handoff_mode: string;
  always_handoff_tag_ids: string[];
  knowledge_scope: KnowledgeScope;
  procedure_scope: AgentScope;
  action_scope: AgentScope;
  transfer_agent_ids: string[];
  placement: AgentPlacement;
}

/* One agent: its fields are what customers get, draft is the working copy. */
export interface Agent extends AgentDefinition {
  id: string;
  object: "ai_agent";
  is_default: boolean;
  role: string;
  template: string | null;
  status: AgentStatus;
  enabled: boolean;
  lock_version: number;
  has_unpublished_changes: boolean;
  live_version: number;
  draft: AgentDefinition;
  published_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface AgentSettings {
  answer_key_id: string | null;
  answer_model: string | null;
  monthly_spend_cap_micros: number | null;
}

export type AgentView = Omit<AiAgentView, "agent"> & { agent: Agent; settings: AgentSettings };

export type { AiModelEntry };

export type AgentPatch = Partial<Omit<AgentDefinition, "placement">> & {
  placement?: Partial<AgentPlacement>;
  lock_version?: number;
};

export interface AgentTemplate {
  key: string;
  object: "ai_agent_template";
  group: "support" | "growth" | "internal" | "blank";
  name: string;
  description: string;
  agent_name: string;
  role: string;
  widget: boolean;
}

export interface PlacementTarget {
  id: string;
  object: "ai_placement_target";
  kind: "connection" | "room" | "inbox";
  name: string;
  channel: string;
  agent_id: string | null;
}

export interface AgentVersion {
  id: string;
  object: "ai_agent_version";
  agent_id: string;
  version: number;
  definition: AgentDefinition;
  change_note: string | null;
  published_by_member_id: string | null;
  published_at: string;
}

const base = "/ai/agents";
const path = (id: string) => `${base}/${encodeURIComponent(id)}`;

export function agentsQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "agents"],
    queryFn: () => request<Agent[]>("GET", base, undefined, { workspace }),
  });
}

export function agentQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "agents", id],
    queryFn: () => request<AgentView>("GET", path(id), undefined, { workspace }),
  });
}

export function agentTemplatesQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "agent-templates"],
    queryFn: () => request<AgentTemplate[]>("GET", "/ai/agent-templates", undefined, { workspace }),
    staleTime: Number.POSITIVE_INFINITY,
  });
}

export function placementsQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "agents", "placements"],
    queryFn: () => request<PlacementTarget[]>("GET", "/ai/placements", undefined, { workspace }),
  });
}

export function createAgent(workspace: string, name: string, template: string): Promise<AgentView> {
  return request<AgentView>("POST", base, { name, ...(template ? { template } : {}) }, { workspace });
}

export function saveAgent(workspace: string, id: string, patch: AgentPatch): Promise<AgentView> {
  return request<AgentView>("PATCH", path(id), patch, { workspace });
}

export function deleteAgent(workspace: string, id: string): Promise<unknown> {
  return request("DELETE", path(id), undefined, { workspace });
}

export function publishAgent(workspace: string, id: string, changeNote = ""): Promise<AgentView> {
  return request<AgentView>("POST", `${path(id)}/publications`, changeNote ? { change_note: changeNote } : {}, {
    workspace,
  });
}

export function pauseAgent(workspace: string, id: string): Promise<AgentView> {
  return request<AgentView>("DELETE", `${path(id)}/publications`, undefined, { workspace });
}

export function agentVersionsQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "agents", id, "versions"],
    queryFn: () => request<AgentVersion[]>("GET", `${path(id)}/versions`, undefined, { workspace }),
  });
}

export function restoreAgentVersion(
  workspace: string,
  id: string,
  version: number,
  publish: boolean,
): Promise<AgentView> {
  return request<AgentView>(
    "POST",
    `${path(id)}/versions/${version}/restore`,
    { publish },
    {
      workspace,
    },
  );
}

/* Preview and simulations: answering as the draft or the live version. */
export type AnswerAs = "draft" | "live";

export interface AgentPreview {
  object: "ai_agent_preview";
  run_id: string;
  reply: string;
  outcome: string;
  handoff: { reason: string; note?: string; team_id?: string; priority?: string } | null;
  active_state: SimState;
  procedure_trace?: ProcedureTrace;
  action_calls: ObservedCall[];
}

export interface AgentPreviewInput {
  version?: AnswerAs;
  history?: SimMessage[];
  message?: string;
  persona?: SimPersona;
  action_mocks?: Record<string, ActionMock>;
  active_state?: SimState;
  confirmed_action_ids?: string[];
  source_conversation_id?: string;
  stream_id?: string;
}

export function previewAgent(workspace: string, id: string, input: AgentPreviewInput): Promise<AgentPreview> {
  return request<AgentPreview>("POST", `${path(id)}/preview`, input, { workspace });
}

export interface AgentSimulation {
  id: string;
  object: "ai_agent_simulation";
  agent_id: string;
  name: string;
  source_conversation_id: string | null;
  persona: SimPersona;
  messages: { text: string; confirm?: boolean }[];
  action_mocks: Record<string, ActionMock>;
  assertions: Assertion[];
  draft_result: TestResult | null;
  live_result: TestResult | null;
  simulated_at: string | null;
  created_at: string;
  updated_at: string;
}

export type AgentSimulationInput = Partial<
  Pick<AgentSimulation, "name" | "persona" | "messages" | "action_mocks" | "assertions" | "simulated_at">
>;

export function agentSimulationsQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "agents", id, "simulations"],
    queryFn: () => request<AgentSimulation[]>("GET", `${path(id)}/simulations`, undefined, { workspace }),
  });
}

export function createAgentSimulation(
  workspace: string,
  id: string,
  input: AgentSimulationInput,
): Promise<AgentSimulation> {
  return request<AgentSimulation>("POST", `${path(id)}/simulations`, input, { workspace });
}

export function updateAgentSimulation(
  workspace: string,
  id: string,
  simulation: string,
  input: AgentSimulationInput,
): Promise<AgentSimulation> {
  return request<AgentSimulation>("PATCH", `${path(id)}/simulations/${encodeURIComponent(simulation)}`, input, {
    workspace,
  });
}

export function deleteAgentSimulation(workspace: string, id: string, simulation: string): Promise<unknown> {
  return request("DELETE", `${path(id)}/simulations/${encodeURIComponent(simulation)}`, undefined, { workspace });
}

export function runAgentSimulations(
  workspace: string,
  id: string,
  input: { simulation_ids?: string[]; version?: AnswerAs; stream_id?: string },
): Promise<AgentSimulation[]> {
  return request<AgentSimulation[]>("POST", `${path(id)}/simulations/run`, input, { workspace });
}

/* resultOf is how a simulation last went against the copy asked for. */
export function resultOf(sim: AgentSimulation, as: AnswerAs): TestResult | null {
  return as === "live" ? sim.live_result : sim.draft_result;
}
