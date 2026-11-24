/*
 * The workspace's workflows on the server. A workflow keeps a working
 * copy, saved as it is edited, and the version that runs, made by
 * publishing that copy. Every save names the lock version it started
 * from, so two editors never overwrite each other, and a publish names
 * the copy that was reviewed, so a change saved meanwhile is never
 * published unseen. Reading takes workflows:read; changing them takes
 * the workflows:manage permission, which the server checks.
 */
import { keepPreviousData, queryOptions } from "@tanstack/react-query";

import type { AudienceRules } from "./audiences";
import { type Page, request, requestPage } from "./client";

export type WorkflowStatus = "draft" | "live" | "paused";

/*
 * Condition is one rule of the condition language: a field compared
 * with a value, or a group of rules that must all, or any, hold.
 */
export interface Condition {
  all?: Condition[];
  any?: Condition[];
  not?: Condition;
  field?: string;
  op?: string;
  value?: unknown;
}

/*
 * SpecPath is one way out of a step. Paths people add carry a label
 * and, on a branch, a condition; fixed paths carry the outcome that
 * takes them, such as else, typed or timeout.
 */
export interface SpecPath {
  id: string;
  label?: string;
  outcome?: string;
  condition?: Condition;
  description?: string;
  steps: SpecStep[];
}

export interface SpecStep {
  id: string;
  kind: string;
  settings?: Record<string, unknown>;
  paths?: SpecPath[];
}

/*
 * WorkflowGoal is what analytics counts as the workflow working: the
 * conversation resolved without going to a person, or rated at least
 * min_rating, within the window after the run started ("7d").
 */
export interface WorkflowGoal {
  kind: "resolved" | "csat";
  window?: string;
  min_rating?: number;
}

/*
 * Spec is a workflow definition, exactly as the server stores it.
 */
export interface Spec {
  schema: number;
  trigger: string;
  trigger_settings?: Record<string, unknown>;
  channels?: string[];
  audience?: AudienceRules;
  schedule: { mode: string; schedule_id?: string };
  reentry: { mode: string; cooldown?: string };
  exclusive: boolean;
  goal?: WorkflowGoal | null;
  steps: SpecStep[];
}

/*
 * Issue is one thing checking a definition found, pointing at the step,
 * path or setting it is about.
 */
export interface Issue {
  code: string;
  step_id?: string;
  path_id?: string;
  field?: string;
  message: string;
}

export interface Report {
  errors: Issue[];
  warnings: Issue[];
}

export interface Workflow {
  id: string;
  name: string;
  description: string | null;
  trigger: string;
  status: WorkflowStatus;
  kind: string;
  position: number;
  lock_version: number;
  draft_hash: string;
  has_unpublished_changes: boolean;
  live_version_id: string | null;
  archived: boolean;
  spec?: Spec;
  report?: Report;
  published_at: string | null;
  paused_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface WorkflowVersion {
  id: string;
  number: number;
  note: string | null;
  published_by_member_id: string | null;
  created_at: string;
  kind?: string;
  published_via?: string | null;
  restored_from_version_id?: string | null;
  spec?: Spec;
}

export type RunStatus = "running" | "waiting" | "completed" | "passed" | "exited" | "canceled" | "failed" | "expired";

export type RunStepStatus = "completed" | "waiting" | "skipped" | "failed" | "interrupted";

/*
 * RunStep is what one step did in a run: how it ended, the path it took
 * and what it found, in a detail that differs by kind.
 */
export interface RunStep {
  step_id: string;
  kind: string;
  status: RunStepStatus;
  outcome: string | null;
  path_id: string | null;
  detail: Record<string, unknown>;
  started_at: string;
  finished_at: string | null;
}

/*
 * WorkflowRun is one run of a workflow, on a conversation or, for a
 * workflow run from another, under its parent's run.
 */
export interface WorkflowRun {
  id: string;
  status: RunStatus;
  end_reason: string | null;
  trigger: string;
  conversation_id: string | null;
  parent_run_id: string | null;
  /* The version that handled the run; null for a test run. */
  version_id?: string | null;
  version_number?: number | null;
  started_at: string;
  ended_at: string | null;
  steps: RunStep[];
}

/*
 * WorkflowStepMetrics is what one step did over a range: runs that
 * reached it, wait on it now, failed on it, ended on it without
 * completing, and left by each path.
 */
export interface WorkflowStepMetrics {
  reached: number;
  waiting: number;
  failed: number;
  dropped: number;
  paths: Record<string, number>;
}

/*
 * WorkflowMetrics is how a workflow did over a range, for every version
 * or one. goal_met is null when the workflow has no goal.
 */
export interface WorkflowMetrics {
  workflow_id: string;
  from: string;
  to: string;
  version: number | null;
  goal: { kind: "resolved" | "csat"; window: string; min_rating: number | null } | null;
  triggered: number;
  engaged: number;
  completed: number;
  active: number;
  handed_over: number;
  failed: number;
  goal_met: number | null;
  days: { day: string; triggered: number; completed: number; failed: number }[];
  steps: Record<string, WorkflowStepMetrics>;
}

/*
 * MetricsRange narrows analytics: whole days from and to (YYYY-MM-DD),
 * and a version number, 0 for every version.
 */
export interface MetricsRange {
  from?: string;
  to?: string;
  version?: number;
}

/*
 * WorkflowFailure is one run that failed, or one of whose steps failed,
 * with the first failing step and the conversation it worked.
 */
export interface WorkflowFailure {
  id: string;
  run_id: string;
  status: RunStatus;
  end_reason: string | null;
  error: { code?: string; message?: string } | null;
  conversation_id: string | null;
  conversation_number: number | null;
  version_number: number | null;
  step_id: string | null;
  step_kind: string | null;
  outcome: string | null;
  detail: Record<string, unknown>;
  started_at: string;
  ended_at: string | null;
}

const path = (id: string) => `/workflows/${encodeURIComponent(id)}`;

export function workflowsQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "workflows"],
    queryFn: () => request<Workflow[]>("GET", "/workflows", undefined, { workspace }),
  });
}

export function workflowQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "workflows", id],
    queryFn: () => request<Workflow>("GET", path(id), undefined, { workspace }),
    retry: false,
  });
}

export function workflowVersionsQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "workflows", id, "versions"],
    queryFn: () => request<WorkflowVersion[]>("GET", `${path(id)}/versions`, undefined, { workspace }),
  });
}

/*
 * workflowRunsQuery reads a workflow's latest runs, newest first, each
 * with what its steps did.
 */
export function workflowRunsQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "workflows", id, "runs"],
    queryFn: () => request<WorkflowRun[]>("GET", `${path(id)}/runs?limit=20`, undefined, { workspace }),
  });
}

function rangeParams(range: MetricsRange): URLSearchParams {
  const query = new URLSearchParams();
  if (range.from) {
    query.set("from", range.from);
  }
  if (range.to) {
    query.set("to", range.to);
  }
  if (range.version) {
    query.set("version", String(range.version));
  }

  return query;
}

/*
 * workflowMetricsQuery reads a workflow's analytics over a range.
 */
export function workflowMetricsQuery(workspace: string, id: string, range: MetricsRange = {}) {
  return queryOptions({
    queryKey: ["workspace", workspace, "workflows", id, "metrics", range],
    queryFn: () =>
      request<WorkflowMetrics>("GET", `${path(id)}/metrics?${rangeParams(range)}`, undefined, { workspace }),
    placeholderData: keepPreviousData,
  });
}

/*
 * workflowFailuresPage reads a page of a workflow's failed runs, newest
 * first, narrowed to one step when step is set.
 */
export function workflowFailuresPage(
  workspace: string,
  id: string,
  range: MetricsRange & { step?: string; before?: string } = {},
): Promise<Page<WorkflowFailure>> {
  const query = rangeParams(range);
  query.set("limit", "25");
  if (range.step) {
    query.set("step_id", range.step);
  }
  if (range.before) {
    query.set("before", range.before);
  }

  return requestPage<WorkflowFailure>(`${path(id)}/failures?${query}`, { workspace });
}

/*
 * restoreWorkflowVersion copies a version into the working copy, ready
 * to check and publish again; the live version keeps running.
 */
export function restoreWorkflowVersion(workspace: string, id: string, version: string): Promise<Workflow> {
  return request<Workflow>("PATCH", path(id), { restore_version_id: version }, { workspace });
}

export function createWorkflow(workspace: string, name: string, trigger: string): Promise<Workflow> {
  return request<Workflow>("POST", "/workflows", { name, trigger }, { workspace });
}

/*
 * saveWorkflow saves the working copy, or renames the workflow, from the
 * lock version the editor started with.
 */
export function saveWorkflow(
  workspace: string,
  id: string,
  changes: { spec?: Spec; name?: string; lock_version: number },
): Promise<Workflow> {
  return request<Workflow>("PATCH", path(id), changes, { workspace });
}

/*
 * publishWorkflow makes the reviewed working copy the live version.
 * Warnings must be acknowledged to publish past them.
 */
export function publishWorkflow(
  workspace: string,
  id: string,
  body: { draft_hash: string; acknowledge_warnings?: boolean },
): Promise<{ workflow: Workflow; version: WorkflowVersion }> {
  return request<{ workflow: Workflow; version: WorkflowVersion }>("POST", `${path(id)}/publications`, body, {
    workspace,
  });
}

export function setWorkflowStatus(workspace: string, id: string, status: "live" | "paused"): Promise<Workflow> {
  return request<Workflow>("PATCH", path(id), { status }, { workspace });
}

export function archiveWorkflow(workspace: string, id: string): Promise<unknown> {
  return request<unknown>("DELETE", path(id), undefined, { workspace });
}

export function reorderWorkflows(workspace: string, order: string[]): Promise<Workflow[]> {
  return request<Workflow[]>("PATCH", "/workflows", { order }, { workspace });
}
