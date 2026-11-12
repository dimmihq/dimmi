/*
 * The workspace's workflows on the server. A workflow keeps a working
 * copy, saved as it is edited, and the version that runs, made by
 * publishing that copy. Every save names the lock version it started
 * from, so two editors never overwrite each other, and a publish names
 * the copy that was reviewed, so a change saved meanwhile is never
 * published unseen. Reading takes workflows:read; changing them takes
 * the workflows:manage permission, which the server checks.
 */
import { queryOptions } from "@tanstack/react-query";

import type { AudienceRules } from "./audiences";
import { request } from "./client";

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
  started_at: string;
  ended_at: string | null;
  steps: RunStep[];
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
