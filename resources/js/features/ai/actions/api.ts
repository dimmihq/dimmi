/*
 * The agent actions API as the pages use it: connections to the
 * workspace's own REST APIs and MCP servers, the actions the agent may
 * call through them, their published versions and test runs, and the
 * calls teammates approve or decline. Every call names the workspace in
 * the Dimmi-Workspace header and every id is a public one (acn_, aac_,
 * aacv_, aax_). The types live here, apart from the rest of the agent's
 * API, so they can be swapped for the server's own in one place.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "@/lib/api/client";

export const CONNECTION_KINDS = ["http", "mcp"] as const;
export const AUTH_TYPES = ["none", "bearer", "api_key_header", "api_key_query", "basic"] as const;
export const HTTP_METHODS = ["GET", "POST", "PUT", "PATCH", "DELETE"] as const;
export const FIELD_TYPES = ["string", "number", "integer", "boolean"] as const;
export const ACTION_CHANNELS = ["widget", "email", "api"] as const;
export const CALL_STATES = [
  "awaiting_confirmation",
  "pending_approval",
  "approved",
  "declined",
  "expired",
  "cancelled",
  "running",
  "succeeded",
  "failed",
] as const;

export type ConnectionKind = (typeof CONNECTION_KINDS)[number];
export type AuthType = (typeof AUTH_TYPES)[number];
export type HttpMethod = (typeof HTTP_METHODS)[number];
export type FieldType = (typeof FIELD_TYPES)[number];
export type ActionChannel = (typeof ACTION_CHANNELS)[number];
export type CallState = (typeof CALL_STATES)[number];
export type SideEffect = "read" | "write";
export type ActionStatus = "draft" | "published" | "disabled";
export type Confirmation = "none" | "customer";
export type Approval = "none" | "teammate" | "admin";
export type ConnectionStatus = "active" | "error" | "disabled";

/*
 * ActionConnection is where and how actions call: a base URL (or an MCP
 * endpoint) and its auth. Secrets never come back, only their last four
 * characters in secret_hint.
 */
export interface ActionConnection {
  id: string;
  object: "ai_action_connection";
  name: string;
  kind: ConnectionKind;
  base_url: string;
  auth_type: AuthType;
  auth_config: { header_name?: string; query_param?: string; prefix?: string };
  secret_hint: string | null;
  default_headers: Record<string, string>;
  variables: Record<string, string>;
  timeout_ms: number;
  status: ConnectionStatus;
  status_reason: string | null;
  last_error?: string | null;
  last_success_at: string | null;
  tools_synced_at?: string | null;
  mcp_server_info?: { name: string; version: string } | null;
  actions_count: number;
  created_at: string;
  updated_at: string;
}

/*
 * NewConnection is what adding or changing a connection sends. token
 * carries a bearer token or an API key; username and password are for
 * basic auth. Leaving a secret out keeps the stored one.
 */
export interface NewConnection {
  name: string;
  kind: ConnectionKind;
  base_url: string;
  auth_type: AuthType;
  auth_config?: ActionConnection["auth_config"];
  token?: string;
  username?: string;
  password?: string;
  default_headers?: Record<string, string>;
  variables?: Record<string, string>;
  timeout_ms?: number;
}

export interface ConnectionTest {
  status: "ok" | "error";
  status_reason: string | null;
  http_status: number | null;
  latency_ms: number | null;
  tools_count?: number | null;
}

export interface ToolSync {
  created: number;
  updated: number;
  disabled: number;
  skipped: number;
}

/*
 * SchemaProperty is one input the agent fills, in the portable subset of
 * JSON Schema every model provider accepts.
 */
export interface SchemaProperty {
  type: FieldType;
  description?: string;
  title?: string;
  enum?: (string | number)[];
  minimum?: number;
  maximum?: number;
  minLength?: number;
  maxLength?: number;
  format?: string;
}

export interface InputSchema {
  type: "object";
  properties: Record<string, SchemaProperty>;
  required?: string[];
  additionalProperties?: false;
}

export interface ResponseField {
  name: string;
  path: string;
  description?: string;
}

/*
 * ActionDefinition is everything a version freezes: what the agent reads,
 * how the request is built (or which MCP tool runs), what of the answer
 * the agent keeps, where it may run and who must agree first.
 */
export interface ActionDefinition {
  connection_id: string;
  name: string;
  title: string;
  description: string;
  side_effect: SideEffect;
  method?: HttpMethod | null;
  path_template?: string | null;
  query: Record<string, string>;
  headers: Record<string, string>;
  body_template: unknown;
  body_encoding: "json" | "form";
  mcp_tool_name?: string | null;
  bound_arguments: Record<string, string>;
  input_schema: InputSchema;
  response_fields: ResponseField[];
  channels: ActionChannel[];
  confirmation: Confirmation;
  approval: Approval;
  approval_timeout_minutes: number;
}

export interface VersionRef {
  id: string;
  number: number;
  created_at: string;
}

export interface AiAction extends ActionDefinition {
  id: string;
  object: "ai_action";
  kind: "http" | "mcp_tool";
  status: ActionStatus;
  published_version: VersionRef | null;
  has_unpublished_changes: boolean;
  warnings: string[];
  source: string;
  created_at: string;
  updated_at: string;
}

export interface ActionVersion extends VersionRef {
  object: "ai_action_version";
  note: string | null;
  published_by: { type: string; id: string; name: string } | null;
  spec: ActionDefinition;
}

/*
 * SampleContact is the customer a test runs as, treated as signed in.
 */
export interface SampleContact {
  external_id?: string;
  email?: string;
  custom_attributes?: Record<string, string>;
}

export interface ActionTestRequest {
  input: Record<string, unknown>;
  contact: SampleContact;
  version: "draft";
  mode: "dry_run" | "live";
  confirm_side_effects: boolean;
}

export interface ActionTestResult {
  status: "succeeded" | "failed" | "blocked";
  error_code: string | null;
  request: { method: string; url: string; headers: Record<string, string>; body: unknown } | null;
  response: { status: number; content_type: string | null; bytes: number; body: unknown } | null;
  result: unknown;
  validation_errors: string[];
  warnings: string[];
  latency_ms: number | null;
  confirmation_text?: string | null;
}

/*
 * ActionCall is a call as a conversation's message carries it under
 * metadata.ai.action_call: what runs, with the arguments frozen when it
 * was requested, why the agent asked, and how it was decided.
 */
export interface ActionCall {
  id: string;
  action: string | { name: string; title?: string | null };
  state: CallState;
  summary: string | null;
  arguments: Record<string, unknown>;
  reason: string | null;
  expires_at: string | null;
  decided_by: { id: string; name: string } | null;
  decision_reason: string | null;
  result_summary?: string | null;
}

export interface CallDecision {
  status: "approved" | "declined";
  reason?: string;
  customer_message?: string;
}

/*
 * actionsKey is the root of every actions query of a workspace, so one
 * invalidation refreshes them all.
 */
export function actionsKey(workspace: string) {
  return ["workspace", workspace, "ai-actions"] as const;
}

export function connectionsQuery(workspace: string) {
  return queryOptions({
    queryKey: [...actionsKey(workspace), "connections"],
    queryFn: () => request<ActionConnection[]>("GET", "/ai/action-connections", undefined, { workspace }),
  });
}

export function actionsQuery(workspace: string) {
  return queryOptions({
    queryKey: [...actionsKey(workspace), "actions"],
    queryFn: () => request<AiAction[]>("GET", "/ai/actions", undefined, { workspace }),
  });
}

export function actionQuery(workspace: string, action: string) {
  return queryOptions({
    queryKey: [...actionsKey(workspace), "action", action],
    queryFn: () => request<AiAction>("GET", `/ai/actions/${action}`, undefined, { workspace }),
  });
}

export function versionsQuery(workspace: string, action: string) {
  return queryOptions({
    queryKey: [...actionsKey(workspace), "versions", action],
    queryFn: () => request<ActionVersion[]>("GET", `/ai/actions/${action}/versions`, undefined, { workspace }),
  });
}

export function createConnection(workspace: string, body: NewConnection) {
  return request<ActionConnection>("POST", "/ai/action-connections", body, { workspace });
}

export function updateConnection(
  workspace: string,
  id: string,
  body: Partial<NewConnection> & { status?: "active" | "disabled" },
) {
  return request<ActionConnection>("PATCH", `/ai/action-connections/${id}`, body, { workspace });
}

export function deleteConnection(workspace: string, id: string) {
  return request<void>("DELETE", `/ai/action-connections/${id}?force=true`, undefined, { workspace });
}

export function testConnection(workspace: string, id: string) {
  return request<ConnectionTest>("POST", `/ai/action-connections/${id}/tests`, {}, { workspace });
}

export function syncTools(workspace: string, id: string) {
  return request<ToolSync>("POST", `/ai/action-connections/${id}/tool-syncs`, {}, { workspace });
}

export function createAction(workspace: string, body: ActionDefinition) {
  return request<AiAction>("POST", "/ai/actions", body, { workspace });
}

export function updateAction(workspace: string, id: string, body: Partial<ActionDefinition>) {
  return request<AiAction>("PATCH", `/ai/actions/${id}`, body, { workspace });
}

export function deleteAction(workspace: string, id: string) {
  return request<void>("DELETE", `/ai/actions/${id}`, undefined, { workspace });
}

export function publishAction(workspace: string, id: string, note?: string) {
  return request<ActionVersion>(
    "POST",
    `/ai/actions/${id}/publications`,
    { note: note ?? null, acknowledge_warnings: [] },
    { workspace },
  );
}

export function restoreVersion(workspace: string, id: string, version: string) {
  return request<AiAction>("PATCH", `/ai/actions/${id}`, { restore_version_id: version }, { workspace });
}

export function setActionStatus(workspace: string, id: string, status: "published" | "disabled") {
  return request<AiAction>("PATCH", `/ai/actions/${id}`, { status }, { workspace });
}

export function testAction(workspace: string, id: string, body: ActionTestRequest) {
  return request<ActionTestResult>("POST", `/ai/actions/${id}/tests`, body, { workspace });
}

export function decideCall(workspace: string, id: string, body: CallDecision) {
  return request<ActionCall>("PATCH", `/ai/action-calls/${id}`, body, { workspace });
}
