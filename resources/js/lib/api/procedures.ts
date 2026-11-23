/*
 * The workspace's procedures: how the team wants dimmi to handle one kind
 * of request. The working copy is saved as it is edited, naming the lock
 * version it started from; dimmi follows only the published copy.
 * Reading takes ai_actions:read; changing takes ai_agent:manage, which
 * the server checks.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export type ProcedureStepKind = "instruction" | "condition" | "action" | "branch" | "sub_procedure";

export type BindingKind = "action" | "attribute" | "article" | "handoff" | "tag" | "ticket_type" | "sub_procedure";

/*
 * ProcedureBinding describes one chip in a step's words, kept in the text
 * as [[kind:ref]]. An action chip says whether dimmi must call it and
 * what to do when it cannot be offered.
 */
export interface ProcedureBinding {
  id?: string;
  kind: BindingKind;
  ref: string;
  label?: string;
  execution?: "required" | "conditional";
  on_missing?: "handoff" | "continue";
  priority?: "low" | "medium" | "high" | "urgent";
}

export interface ProcedureEnforcement {
  requires?: string[];
  max_retries?: number;
  on_violation?: "handoff" | "warn_only";
}

export type ConditionOperator =
  | "eq"
  | "neq"
  | "gt"
  | "gte"
  | "lt"
  | "lte"
  | "in"
  | "not_in"
  | "contains"
  | "exists"
  | "not_exists";

export interface ConditionRule {
  field: string;
  operator: ConditionOperator;
  value?: unknown;
}

/*
 * ConditionExpr is a rule dimmi decides in code, never by the model.
 */
export interface ConditionExpr {
  mode: "all" | "any";
  rules: ConditionRule[];
}

export interface ProcedureStep {
  id: string;
  kind: ProcedureStepKind;
  text: string;
  action_id?: string;
  bindings?: ProcedureBinding[];
  enforcement?: ProcedureEnforcement;
  condition?: ConditionExpr;
  then_steps?: ProcedureStep[];
  else_steps?: ProcedureStep[];
  sub_procedure_id?: string;
}

export interface ProcedureScope {
  audience_ids?: string[];
  channels?: string[];
  inbox_ids?: string[];
  require_verified_identity?: boolean;
  reusable_only?: boolean;
}

/*
 * ProcedureDefinition is a procedure as dimmi reads it: when it applies,
 * where, the steps in order, and when and where to hand over.
 */
export interface ProcedureDefinition {
  description: string;
  scope?: ProcedureScope;
  steps: ProcedureStep[];
  handover: string[];
  handover_rule?: { team_id?: string; priority?: "low" | "medium" | "high" | "urgent" };
}

export interface Procedure {
  id: string;
  name: string;
  status: "draft" | "live";
  lock_version: number;
  definition: ProcedureDefinition;
  has_unpublished_changes: boolean;
  uses: number;
  published_at: string | null;
  updated_at: string;
}

/*
 * ProcedureIssue is one problem that stops a procedure from being
 * published.
 */
export interface ProcedureIssue {
  code: string;
  step_id?: string;
  field?: string;
  message: string;
}

const path = (id: string) => `/procedures/${encodeURIComponent(id)}`;

export function proceduresQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "procedures"],
    queryFn: () => request<Procedure[]>("GET", "/procedures", undefined, { workspace }),
  });
}

export function procedureQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "procedures", id],
    queryFn: () => request<Procedure>("GET", path(id), undefined, { workspace }),
    retry: false,
  });
}

export function createProcedure(workspace: string, name: string): Promise<Procedure> {
  return request<Procedure>("POST", "/procedures", { name }, { workspace });
}

export function saveProcedure(
  workspace: string,
  id: string,
  changes: { name?: string; definition?: ProcedureDefinition; lock_version: number },
): Promise<Procedure> {
  return request<Procedure>("PATCH", path(id), changes, { workspace });
}

export function publishProcedure(workspace: string, id: string): Promise<Procedure> {
  return request<Procedure>("POST", `${path(id)}/publications`, undefined, { workspace });
}

export function unpublishProcedure(workspace: string, id: string): Promise<Procedure> {
  return request<Procedure>("DELETE", `${path(id)}/publications`, undefined, { workspace });
}

export function deleteProcedure(workspace: string, id: string): Promise<unknown> {
  return request<unknown>("DELETE", path(id), undefined, { workspace });
}

export function publishProcedureNote(workspace: string, id: string, changeNote: string): Promise<Procedure> {
  return request<Procedure>("POST", `${path(id)}/publications`, { change_note: changeNote }, { workspace });
}

/*
 * The simulator: one turn of dimmi following the working copy, with
 * mocked actions, handing back the state for the next turn.
 */
export interface SimMessage {
  role: "customer" | "agent";
  text: string;
}

export interface SimPersona {
  name?: string;
  email?: string;
  external_id?: string;
  verified?: boolean;
  channel?: string;
  inbox_id?: string;
  attributes?: Record<string, unknown>;
  company?: Record<string, unknown>;
}

export interface ActionMock {
  status?: number;
  body?: unknown;
}

export interface SimPending {
  action_id: string;
  tool: string;
  args: Record<string, unknown>;
  state: "awaiting_confirmation" | "pending_approval";
  summary: string;
}

export interface SimState {
  procedure_id?: string;
  status?: string;
  called_action_ids: string[];
  /* The steps dimmi finished, carried from one simulated turn to the next. */
  completed_step_ids?: string[];
  branches?: Record<string, string>;
  step_outputs: Record<string, unknown>;
  pending: SimPending[];
}

export interface ObservedCall {
  action_id: string;
  tool: string;
  args?: Record<string, unknown>;
  succeeded: boolean;
  blocked?: boolean;
}

export interface TraceStep {
  step_id: string;
  kind: string;
  status: "done" | "active" | "pending" | "locked" | "evaluated";
  branch_taken?: "then" | "else";
  deterministic?: boolean;
  needs?: string[];
  action_id?: string;
  action_call_id?: string;
  bindings_called?: string[];
}

export interface GuardEvent {
  type: string;
  attempt?: number;
  attempted_tool?: string;
  missing_tool?: string;
  step_id?: string;
}

export interface ProcedureTrace {
  procedure_id: string;
  procedure_name: string;
  version: number;
  fit_score?: number | null;
  status: string;
  retries: number;
  steps: TraceStep[];
  guard_events: GuardEvent[];
}

export interface Simulation {
  object: "procedure_simulation";
  run_id: string;
  reply: string;
  outcome: string;
  matched: boolean;
  handoff: { reason: string; note?: string; team_id?: string; priority?: string } | null;
  active_state: SimState;
  procedure_trace?: ProcedureTrace;
  action_calls: ObservedCall[];
}

export interface SimulateInput {
  target?: "draft" | "live";
  history?: SimMessage[];
  message?: string;
  persona?: SimPersona;
  action_mocks?: Record<string, ActionMock>;
  active_state?: SimState;
  confirmed_action_ids?: string[];
  source_conversation_id?: string;
  /* An id of the caller's choosing: each tool the turn finishes is sent live under it. */
  stream_id?: string;
}

export function simulateProcedure(workspace: string, id: string, input: SimulateInput): Promise<Simulation> {
  return request<Simulation>("POST", `${path(id)}/simulate`, input, { workspace });
}

export type AssertionType =
  | "procedure_matched"
  | "action_called"
  | "action_not_called"
  | "action_order"
  | "branch_taken"
  | "handed_off"
  | "reply_contains"
  | "reply_not_contains"
  | "llm_rubric";

export interface Assertion {
  type: AssertionType;
  procedure_id?: string;
  matched?: boolean;
  action?: string;
  args?: Record<string, unknown>;
  before?: string;
  step_id?: string;
  branch?: "then" | "else";
  handed_off?: boolean;
  team_id?: string;
  priority?: string;
  text?: string;
  rubric?: string;
}

export interface TestResult {
  passed: boolean;
  ran_at: string;
  turns: {
    message: string;
    /* The customer pressed Confirm on a card rather than writing. */
    confirm?: boolean;
    /* The simulated customer was asked for something the saved chat never gave. */
    improvised?: boolean;
    reply: string;
    outcome: string;
    calls: ObservedCall[];
    run_id: string;
    /* The turn moved the chat to another AI agent, which wrote the reply. */
    transfer?: { agent_id: string; name: string; from_name: string; summary: string } | null;
  }[];
  assertion_results: { type: AssertionType; passed: boolean; message: string; skipped?: boolean }[];
  error?: string;
}

export interface ProcedureTestCase {
  id: string;
  object: "procedure_test_case";
  procedure_id: string;
  name: string;
  source_conversation_id: string | null;
  persona: SimPersona;
  messages: { text: string; confirm?: boolean }[];
  action_mocks: Record<string, ActionMock>;
  assertions: Assertion[];
  last_result: TestResult | null;
  /* The moment every run happens at, or null to run at the real time. */
  simulated_at: string | null;
  created_at: string;
  updated_at: string;
}

export type TestCaseInput = Partial<
  Pick<ProcedureTestCase, "name" | "persona" | "messages" | "action_mocks" | "assertions" | "simulated_at">
> & { source_conversation_id?: string };

export function testCasesQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "procedures", id, "test-cases"],
    queryFn: () => request<ProcedureTestCase[]>("GET", `${path(id)}/test-cases`, undefined, { workspace }),
  });
}

export function createTestCase(workspace: string, id: string, input: TestCaseInput): Promise<ProcedureTestCase> {
  return request<ProcedureTestCase>("POST", `${path(id)}/test-cases`, input, { workspace });
}

export function updateTestCase(
  workspace: string,
  id: string,
  test: string,
  input: TestCaseInput,
): Promise<ProcedureTestCase> {
  return request<ProcedureTestCase>("PATCH", `${path(id)}/test-cases/${encodeURIComponent(test)}`, input, {
    workspace,
  });
}

export function deleteTestCase(workspace: string, id: string, test: string): Promise<unknown> {
  return request<unknown>("DELETE", `${path(id)}/test-cases/${encodeURIComponent(test)}`, undefined, { workspace });
}

export function runTestCases(
  workspace: string,
  id: string,
  tests?: string[],
  stream?: string,
): Promise<ProcedureTestCase[]> {
  const body = { ...(tests ? { test_case_ids: tests } : {}), ...(stream ? { stream_id: stream } : {}) };

  return request<ProcedureTestCase[]>("POST", `${path(id)}/test-cases/run`, body, { workspace });
}

/*
 * Every publish is kept as a numbered version the team can compare and
 * put back.
 */
export interface ProcedureVersion {
  id: string;
  object: "procedure_version";
  procedure_id: string;
  version: number;
  name: string;
  definition: ProcedureDefinition;
  change_note: string | null;
  published_by_member_id: string | null;
  published_at: string;
  conversations: number;
  handed_off: number;
}

export function versionsQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "procedures", id, "versions"],
    queryFn: () => request<ProcedureVersion[]>("GET", `${path(id)}/versions?limit=100`, undefined, { workspace }),
  });
}

export function restoreVersion(
  workspace: string,
  id: string,
  version: number,
  body: { publish?: boolean; lock_version?: number },
): Promise<Procedure> {
  return request<Procedure>("POST", `${path(id)}/versions/${version}/restore`, body, { workspace });
}

/*
 * A procedure's scorecard over the last 7, 30 or 90 days.
 */
export interface ProcedureMetrics {
  object: "procedure_metrics";
  procedure_id: string;
  days: number;
  version: number | null;
  matched_conversations: number;
  ai_involved: number;
  ai_resolved: number;
  ai_confirmed: number;
  ai_assumed: number;
  ai_resolution_rate: number | null;
  handed_off: number;
  handover_rate: number | null;
  handover_reasons: { reason: string; kind: string; conversations: number }[];
  csat: { rated: number; positive: number; positive_rate: number | null; mean: number | null };
  guardrails: Record<string, number>;
  steps: {
    step_id: string;
    reached: number;
    done: number;
    completion_rate: number | null;
    took_then: number;
    took_else: number;
  }[];
  actions: {
    action_id: string;
    name: string;
    calls: number;
    failed: number;
    error_rate: number | null;
    mean_latency_ms: number;
  }[];
  runs: {
    run_id: string;
    conversation_id: string | null;
    conversation_number: number;
    outcome: string | null;
    handoff_reason: string | null;
    rating: number | null;
    reopens: number;
    procedure_trace?: ProcedureTrace;
    created_at: string;
  }[];
}

export function procedureMetricsQuery(workspace: string, id: string, days: number, version: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "procedures", id, "metrics", days, version],
    queryFn: () =>
      request<ProcedureMetrics>(
        "GET",
        `${path(id)}/metrics?days=${days}&version=${encodeURIComponent(version)}`,
        undefined,
        {
          workspace,
        },
      ),
  });
}

export interface ProcedureReportRow {
  procedure_id: string;
  name: string;
  matched_conversations: number;
  ai_involved: number;
  ai_resolved: number;
  ai_resolution_rate: number | null;
  handed_off: number;
  handover_rate: number | null;
  rated: number;
  mean_rating: number | null;
  guard_events: number;
}

export function proceduresReportQuery(workspace: string, from: string, to: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "reports", "procedures", from, to],
    queryFn: () =>
      request<ProcedureReportRow[]>("GET", `/reports/procedures?from=${from}&to=${to}`, undefined, { workspace }),
  });
}
