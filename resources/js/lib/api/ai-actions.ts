/*
 * The agent's actions: the connections they reach (an HTTP API or a
 * remote MCP server), the actions themselves with their published
 * versions and tests, and the calls the agent made, which teammates
 * approve or decline. Every call names the workspace in the
 * Dimmi-Workspace header, and every id is a public one (acn_, aac_,
 * aacv_, aax_, conv_, air_, msg_, mem_). Secrets are written only:
 * a connection answers with has_secret and secret_hint, never the secret.
 */
import { queryOptions } from "@tanstack/react-query";

import { type Page, request, requestPage } from "./client";

/* The values the server accepts for the fixed-choice fields. */
export const ACTION_CONNECTION_KINDS = ["http", "mcp"] as const;
export const ACTION_AUTH_TYPES = ["none", "bearer", "api_key_header", "api_key_query", "basic"] as const;
export const ACTION_CONNECTION_STATUSES = ["active", "error", "disabled"] as const;
export const ACTION_KINDS = ["http", "mcp_tool"] as const;
export const ACTION_SIDE_EFFECTS = ["read", "write"] as const;
export const ACTION_CONFIRMATIONS = ["none", "customer"] as const;
export const ACTION_APPROVALS = ["none", "teammate", "admin"] as const;
export const ACTION_STATUSES = ["draft", "published", "disabled"] as const;
export const ACTION_METHODS = ["GET", "POST", "PUT", "PATCH", "DELETE"] as const;
export const ACTION_BODY_ENCODINGS = ["json", "form"] as const;
export const ACTION_CALL_STATES = [
  "awaiting_confirmation",
  "pending_approval",
  "approved",
  "declined",
  "expired",
  "canceled",
  "running",
  "succeeded",
  "failed",
] as const;

export type ActionConnectionKind = (typeof ACTION_CONNECTION_KINDS)[number];
export type ActionAuthType = (typeof ACTION_AUTH_TYPES)[number];
export type ActionConnectionStatus = (typeof ACTION_CONNECTION_STATUSES)[number];
export type ActionKind = (typeof ACTION_KINDS)[number];
export type ActionSideEffect = (typeof ACTION_SIDE_EFFECTS)[number];
export type ActionConfirmation = (typeof ACTION_CONFIRMATIONS)[number];
export type ActionApproval = (typeof ACTION_APPROVALS)[number];
export type ActionStatus = (typeof ACTION_STATUSES)[number];
export type ActionMethod = (typeof ACTION_METHODS)[number];
export type ActionBodyEncoding = (typeof ACTION_BODY_ENCODINGS)[number];
export type ActionCallState = (typeof ACTION_CALL_STATES)[number];

/* The stable error codes a failed call carries in error_code. */
export type ActionCallErrorCode =
  | "invalid_arguments"
  | "identity_unavailable"
  | "rate_limited"
  | "not_found"
  | "upstream_rejected"
  | "auth_failed"
  | "unavailable"
  | "redirect_not_followed"
  | "invalid_response"
  | "response_too_large"
  | "host_blocked"
  | "url_escape"
  | "invalid_path_value"
  | "invalid_header_value"
  | "template_type_mismatch"
  | "outcome_unknown";

/*
 * Templates may hold placeholders: {input.<property>}, {contact.email},
 * {contact.external_id}, {contact.attr.<key>} and {connection.<variable>}.
 * Contact values resolve only where the customer's identity is proven,
 * so an action that uses one is offered only there.
 */

/* header_name for api_key_header, query_param for api_key_query, prefix replaces "Bearer". */
export interface ActionAuthConfig {
  header_name?: string;
  query_param?: string;
  prefix?: string;
}

export interface ActionConnection {
  id: string;
  object: "ai_action_connection";
  name: string;
  kind: ActionConnectionKind;
  base_url: string;
  auth_type: ActionAuthType;
  auth_config: ActionAuthConfig;
  has_secret: boolean;
  /* The secret's last four characters, as "…4f2a". */
  secret_hint: string | null;
  default_headers: Record<string, string>;
  variables: Record<string, string>;
  timeout_ms: number;
  /* error: three refused credentials in a row or a blocked host; its actions are not offered. */
  status: ActionConnectionStatus;
  status_reason: "auth" | "host_blocked" | null;
  last_error: string | null;
  last_error_at: string | null;
  last_success_at: string | null;
  tools_synced_at: string | null;
  /* How many live actions use the connection. */
  actions_count: number;
  created_at: string;
  updated_at: string;
}

/*
 * The body of POST /ai/action-connections (name, kind, base_url required)
 * and of PATCH /ai/action-connections/{connection} (every field optional).
 * token, or username and password for basic auth, is the write-only secret.
 */
export interface ActionConnectionInput {
  name?: string;
  kind?: ActionConnectionKind;
  base_url?: string;
  auth_type?: ActionAuthType;
  auth_config?: ActionAuthConfig;
  token?: string;
  username?: string;
  password?: string;
  default_headers?: Record<string, string>;
  variables?: Record<string, string>;
  timeout_ms?: number;
  /* Only on update: switch the connection off or back on. */
  status?: "active" | "disabled";
}

export interface ActionConnectionTest {
  object: "ai_action_connection_test";
  status: "ok" | "error";
  status_reason: "auth" | "unreachable" | "host_blocked" | "url_escape" | "credentials" | null;
  http_status: number | null;
  latency_ms: number;
  /* MCP connections only. */
  tools_count: number | null;
}

export interface ActionResponseField {
  name: string;
  /* A plain dotted path into the response body: "data.0.status", "items.#.name". */
  path: string;
  description?: string;
}

export interface ActionVersionRef {
  id: string;
  number: number;
  created_at: string;
}

export interface Action {
  id: string;
  object: "ai_action";
  connection_id: string;
  /* The tool name the model sees: ^[a-z][a-z0-9_]{2,47}$. */
  name: string;
  title: string;
  description: string;
  kind: ActionKind;
  side_effect: ActionSideEffect;
  /* customer: the customer presses Confirm on a card before anything runs. */
  confirmation: ActionConfirmation;
  /* teammate or admin: the call waits for that decision; the conversation goes on meanwhile. */
  approval: ActionApproval;
  approval_timeout_minutes: number;
  status: ActionStatus;
  method: ActionMethod | null;
  path_template: string | null;
  query: Record<string, string>;
  headers: Record<string, string>;
  body_template: unknown;
  body_encoding: ActionBodyEncoding;
  mcp_tool_name: string | null;
  bound_arguments: Record<string, unknown>;
  input_schema: Record<string, unknown>;
  response_fields: ActionResponseField[];
  error_message_path: string | null;
  response_max_bytes: number;
  timeout_ms: number | null;
  channels: string[];
  agents: string[];
  source: "dashboard" | "api" | "mcp_sync";
  /* Always empty for now: no publish check raises warnings yet. */
  warnings: string[];
  has_unpublished_changes: boolean;
  published_version: ActionVersionRef | null;
  created_at: string;
  updated_at: string;
}

/*
 * The body of POST /ai/actions (connection_id, name, title, description
 * required) and of PATCH /ai/actions/{action} (every field optional;
 * body_template: null clears the body). A new action defaults to read
 * for GET and write otherwise, and a write to confirmation "customer".
 * status on PATCH only moves a published action to disabled and back.
 */
export interface ActionInput {
  connection_id?: string;
  name?: string;
  title?: string;
  description?: string;
  side_effect?: ActionSideEffect;
  confirmation?: ActionConfirmation;
  approval?: ActionApproval;
  approval_timeout_minutes?: number;
  method?: ActionMethod;
  path_template?: string;
  query?: Record<string, string>;
  headers?: Record<string, string>;
  body_template?: unknown;
  body_encoding?: ActionBodyEncoding;
  mcp_tool_name?: string;
  bound_arguments?: Record<string, unknown>;
  input_schema?: Record<string, unknown>;
  response_fields?: ActionResponseField[];
  error_message_path?: string;
  response_max_bytes?: number;
  timeout_ms?: number;
  channels?: string[];
  agents?: string[];
  status?: "published" | "disabled";
  /* Makes the working copy that version's definition again; publish it to roll back. */
  restore_version_id?: string;
}

export interface ActionVersion {
  id: string;
  object: "ai_action_version";
  action_id: string;
  number: number;
  /* The frozen definition the agent runs. */
  spec: Record<string, unknown>;
  note: string | null;
  published_by: { id: string; name: string } | null;
  created_at: string;
}

/* The stand-in customer a test binds to, treated as verified. */
export interface ActionTestContact {
  external_id?: string;
  email?: string;
  custom_attributes?: Record<string, unknown>;
}

export interface ActionTestInput {
  input?: Record<string, unknown>;
  contact?: ActionTestContact;
  /* dry_run (default) renders the request and sends nothing. */
  mode?: "dry_run" | "live";
  /* Tests always run the working copy. */
  version?: "draft";
  /* A live test of a write needs this, or the server answers 422 side_effects_not_confirmed. */
  confirm_side_effects?: boolean;
}

export interface ActionTest {
  object: "ai_action_test";
  /* rendered: a dry run; blocked: the request could not be built or bound. */
  status: "rendered" | "succeeded" | "failed" | "blocked";
  error_code: ActionCallErrorCode | null;
  validation_errors: string[];
  /* { method, url, headers, body } with secrets as "[secret]", or { tool, arguments } for MCP. */
  request: Record<string, unknown> | null;
  response: Record<string, unknown> | null;
  /* Exactly what the model would read. */
  result: unknown;
  /* What a customer would confirm; confirmation_text repeats it. */
  summary: string;
  confirmation_text: string;
  warnings: string[];
  latency_ms: number;
  call_id: string | null;
}

export interface ActionToolSync {
  object: "ai_action_tool_sync";
  /* New draft actions, one per new tool, each a write asking the customer to confirm. */
  created: Action[];
  /* A sync never changes or disables an earlier action: both stay empty. */
  updated: Action[];
  disabled: Action[];
  /* Tools that already have an action. */
  existing: number;
  skipped: { tool: string; reason: "schema_unsupported" | "definition_invalid" }[];
}

export interface ActionCall {
  id: string;
  object: "ai_action_call";
  action_id: string;
  action_version_id: string | null;
  connection_id: string;
  conversation_id: string | null;
  run_id: string | null;
  /* The agent message that told the customer, which carries the confirmation card. */
  message_id: string | null;
  mode: "live" | "test";
  side_effect: ActionSideEffect;
  confirmation: ActionConfirmation;
  approval: ActionApproval;
  state: ActionCallState;
  title: string;
  /* What would run, rendered by the server from the validated input. */
  summary: string;
  error_code: ActionCallErrorCode | null;
  http_status: number | null;
  decided_by_member_id: string | null;
  /* Internal; never shown to the customer. */
  decision_reason: string | null;
  /* What the agent passes on to the customer after a decline. */
  customer_message: string | null;
  cancel_reason: "customer" | "superseded" | null;
  latency_ms: number | null;
  requested_at: string;
  confirmed_at: string | null;
  decided_at: string | null;
  sent_at: string | null;
  completed_at: string | null;
  /* When a waiting call lapses. */
  expires_at: string | null;
  /* When the customer was told how it ended. */
  reported_at: string | null;
  /* Payloads: present only for members who may read payloads. */
  input?: Record<string, unknown>;
  request?: Record<string, unknown> | null;
  response?: Record<string, unknown> | null;
  result?: unknown;
  created_at: string;
  updated_at: string;
}

export interface ActionCallFilter {
  conversation_id?: string;
  action_id?: string;
  state?: ActionCallState;
  cursor?: string;
}

/* PATCH /ai/action-calls/{call}: 409 action_call_not_pending or action_call_expired when stale. */
export interface ActionCallDecision {
  status: "approved" | "declined";
  reason?: string;
  customer_message?: string;
}

/*
 * The agent reply that proposes a write carries the card under
 * metadata.ai.confirmation; metadata.ai.action_calls lists the calls the
 * reply reports.
 */
export interface ActionConfirmationCard {
  action_call_id: string;
  title: string;
  text: string;
  status: ActionCallState;
  expires_at: string | null;
  details: { label: string; value: string }[];
}

/*
 * metadata.ai.action_call: the change a reply reports, kept in step as it
 * moves. The messenger sees id, state and summary only.
 */
export interface MessageActionCall {
  id: string;
  state: ActionCallState;
  summary: string;
  action?: { name: string; title: string };
  arguments?: Record<string, unknown>;
  expires_at?: string | null;
  decided_by?: { id: string; name: string };
  decision_reason?: string;
  result_summary?: string;
}

export interface ActionCallRef {
  id: string;
  title: string;
  status: ActionCallState;
}

/*
 * The messenger's view of a call, from
 * GET and PATCH /widget/v1/conversations/{conversation}/action-calls/{call}.
 */
export interface WidgetActionCall {
  id: string;
  object: "action_call";
  title: string;
  summary: string;
  state: ActionCallState;
  /* The same as state. */
  status: ActionCallState;
  expires_at: string | null;
}

/* The visitor's answer on the card; a repeated click answers with where the call stands. */
export interface WidgetActionCallUpdate {
  status: "confirmed" | "canceled";
}

/* The messenger path of a call, under /widget/v1. */
export function widgetActionCallPath(conversation: string, call: string) {
  return `/conversations/${conversation}/action-calls/${call}`;
}

export function aiActionsKey(workspace: string) {
  return ["workspace", workspace, "ai", "actions"];
}

export function actionConnectionsQuery(workspace: string) {
  return queryOptions({
    queryKey: [...aiActionsKey(workspace), "connections"],
    queryFn: () => request<ActionConnection[]>("GET", "/ai/action-connections", undefined, { workspace }),
  });
}

export function actionConnectionQuery(workspace: string, connection: string) {
  return queryOptions({
    queryKey: [...aiActionsKey(workspace), "connections", connection],
    queryFn: () => request<ActionConnection>("GET", `/ai/action-connections/${connection}`, undefined, { workspace }),
  });
}

export function createActionConnection(workspace: string, body: ActionConnectionInput) {
  return request<ActionConnection>("POST", "/ai/action-connections", body, { workspace });
}

export function updateActionConnection(workspace: string, connection: string, body: ActionConnectionInput) {
  return request<ActionConnection>("PATCH", `/ai/action-connections/${connection}`, body, { workspace });
}

/* 409 connection_in_use while actions still use it, unless force also deletes them. */
export function deleteActionConnection(workspace: string, connection: string, force = false) {
  const query = force ? "?force=true" : "";
  return request<void>("DELETE", `/ai/action-connections/${connection}${query}`, undefined, { workspace });
}

export function testActionConnection(workspace: string, connection: string, body: { path?: string } = {}) {
  return request<ActionConnectionTest>("POST", `/ai/action-connections/${connection}/tests`, body, {
    workspace,
  });
}

export function syncActionConnectionTools(workspace: string, connection: string) {
  return request<ActionToolSync>("POST", `/ai/action-connections/${connection}/tool-syncs`, undefined, {
    workspace,
  });
}

export function actionsQuery(workspace: string, filter: { status?: ActionStatus; connection_id?: string } = {}) {
  const query = new URLSearchParams(filter as Record<string, string>).toString();
  return queryOptions({
    queryKey: [...aiActionsKey(workspace), "list", filter],
    queryFn: () => request<Action[]>("GET", `/ai/actions${query ? `?${query}` : ""}`, undefined, { workspace }),
  });
}

export function actionQuery(workspace: string, action: string) {
  return queryOptions({
    queryKey: [...aiActionsKey(workspace), "action", action],
    queryFn: () => request<Action>("GET", `/ai/actions/${action}`, undefined, { workspace }),
  });
}

export function actionVersionsQuery(workspace: string, action: string) {
  return queryOptions({
    queryKey: [...aiActionsKey(workspace), "action", action, "versions"],
    queryFn: () => request<ActionVersion[]>("GET", `/ai/actions/${action}/versions`, undefined, { workspace }),
  });
}

export function createAction(workspace: string, body: ActionInput) {
  return request<Action>("POST", "/ai/actions", body, { workspace });
}

export function updateAction(workspace: string, action: string, body: ActionInput) {
  return request<Action>("PATCH", `/ai/actions/${action}`, body, { workspace });
}

export function deleteAction(workspace: string, action: string) {
  return request<void>("DELETE", `/ai/actions/${action}`, undefined, { workspace });
}

/* 409 nothing_to_publish when the working copy equals the live version. */
export function publishAction(
  workspace: string,
  action: string,
  body: { note?: string; acknowledge_warnings?: string[] } = {},
) {
  return request<ActionVersion>("POST", `/ai/actions/${action}/publications`, body, { workspace });
}

export function testAction(workspace: string, action: string, body: ActionTestInput) {
  return request<ActionTest>("POST", `/ai/actions/${action}/tests`, body, { workspace });
}

export function actionCallsPage(workspace: string, filter: ActionCallFilter = {}): Promise<Page<ActionCall>> {
  const query = new URLSearchParams(filter as Record<string, string>).toString();
  return requestPage<ActionCall>(`/ai/action-calls${query ? `?${query}` : ""}`, { workspace });
}

export function actionCallsQuery(workspace: string, filter: ActionCallFilter = {}) {
  return queryOptions({
    queryKey: [...aiActionsKey(workspace), "calls", filter],
    queryFn: () => actionCallsPage(workspace, filter),
  });
}

export function actionCallQuery(workspace: string, call: string) {
  return queryOptions({
    queryKey: [...aiActionsKey(workspace), "calls", call],
    queryFn: () => request<ActionCall>("GET", `/ai/action-calls/${call}`, undefined, { workspace }),
  });
}

export function decideActionCall(workspace: string, call: string, body: ActionCallDecision) {
  return request<ActionCall>("PATCH", `/ai/action-calls/${call}`, body, { workspace });
}
