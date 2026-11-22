/*
 * Operator's routines: prompts it runs on a clock, as a narrowed copy of
 * their owner, leaving suggestions the team reviews. A routine starts
 * from a template, runs daily, weekly, monthly, on a cron or once, and
 * tells the teammates and Slack channels it names how each run went.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export type RoutineCadence = "daily" | "weekly" | "monthly" | "cron" | "once";
export type RoutineStatus = "active" | "paused" | "completed";
export type NotifyPolicy = "on_findings" | "always" | "on_error_only";
export type RoutineOutcome = "findings" | "noop" | "failed" | "skipped";
export type RoutineToolGroup = "conversations" | "knowledge" | "insights" | "changes" | "actions" | "github";

export const ROUTINE_TOOL_GROUPS: RoutineToolGroup[] = [
  "conversations",
  "knowledge",
  "insights",
  "changes",
  "actions",
  "github",
];

export interface RoutineGitHubScope {
  installation_id: string;
  repositories: string[];
  base_branch: string | null;
  ignore_labels: string[];
}

export interface RoutineDelivery {
  in_app_member_ids: string[];
  email_member_ids: string[];
  slack_destination_ids: string[];
}

export interface Routine {
  id: string;
  name: string;
  description: string;
  template_key: string | null;
  prompt: string;
  cadence: RoutineCadence;
  cron_expression: string | null;
  run_at: string | null;
  timezone: string;
  next_run_at: string | null;
  next_runs: string[];
  created_by: string | null;
  owner_id: string | null;
  permissions: string[];
  tool_groups: RoutineToolGroup[];
  inbox_ids: string[];
  write_policies: Record<string, string>;
  scope: { help_center_ids: string[]; github: RoutineGitHubScope | null };
  model: string | null;
  effort: "low" | "medium" | "high" | null;
  max_model_calls: number;
  max_tool_calls: number;
  monthly_budget_micros: number | null;
  month_spend_micros: number;
  notify_policy: NotifyPolicy;
  delivery: RoutineDelivery;
  state: Record<string, unknown>;
  status: RoutineStatus;
  pause_reason: string | null;
  consecutive_failures: number;
  last_run_at: string | null;
  last_success_at: string | null;
  last_outcome: RoutineOutcome | null;
  created_at: string;
  updated_at: string;
}

export interface RoutineTemplate {
  key: string;
  name: string;
  description: string;
  prompt: string;
  cadence: RoutineCadence;
  cron_expression: string;
  tool_groups: RoutineToolGroup[];
  permissions: string[];
  notify_policy: NotifyPolicy;
  requires: string[];
}

export interface RoutineRun {
  id: string;
  routine_id: string;
  trigger: "scheduled" | "manual" | "dry_run";
  triggered_by: string | null;
  scheduled_for: string | null;
  thread_id: string | null;
  status: "queued" | "running" | "completed" | "failed" | "skipped";
  outcome: RoutineOutcome | null;
  noop_reason: string | null;
  skip_reason: string | null;
  error_code: string | null;
  error_message: string | null;
  changesets_count: number;
  cost_micros: number;
  notified: boolean;
  started_at: string | null;
  completed_at: string | null;
  created_at: string;
}

/*
 * RoutineInput is a routine as the builder sends it; on an update, what
 * is left out stays as it is.
 */
export interface RoutineInput {
  template_key?: string;
  name?: string;
  description?: string;
  prompt?: string;
  cadence?: RoutineCadence;
  cron_expression?: string;
  run_at?: string;
  timezone?: string;
  owner_id?: string;
  permissions?: string[];
  tool_groups?: RoutineToolGroup[];
  inbox_ids?: string[];
  scope?: {
    help_center_ids?: string[];
    github?: { installation_id: string; repositories: string[]; base_branch?: string; ignore_labels?: string[] };
  };
  model?: string;
  effort?: string;
  monthly_budget_micros?: number;
  clear_budget?: boolean;
  notify_policy?: NotifyPolicy;
  delivery?: RoutineDelivery;
  status?: "active" | "paused";
  reset_state?: boolean;
}

export const routineKeys = {
  all: (workspace: string) => ["workspace", workspace, "routines"] as const,
  templates: (workspace: string) => ["workspace", workspace, "routines", "templates"] as const,
  one: (workspace: string, id: string) => ["workspace", workspace, "routines", id] as const,
  runs: (workspace: string, id: string) => ["workspace", workspace, "routines", id, "runs"] as const,
};

const base = "/operator/routines";
const one = (id: string) => `${base}/${encodeURIComponent(id)}`;

export function routinesQuery(workspace: string) {
  return queryOptions({
    queryKey: routineKeys.all(workspace),
    queryFn: () => request<Routine[]>("GET", base, undefined, { workspace }),
  });
}

export function routineTemplatesQuery(workspace: string) {
  return queryOptions({
    queryKey: routineKeys.templates(workspace),
    queryFn: () => request<RoutineTemplate[]>("GET", "/operator/routine-templates", undefined, { workspace }),
    staleTime: 5 * 60_000,
  });
}

export function routineQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: routineKeys.one(workspace, id),
    queryFn: () => request<Routine>("GET", one(id), undefined, { workspace }),
  });
}

/*
 * routineRunsQuery reads a routine's runs, and keeps reading every few
 * seconds while one is queued or running.
 */
export function routineRunsQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: routineKeys.runs(workspace, id),
    queryFn: () => request<RoutineRun[]>("GET", `${one(id)}/runs`, undefined, { workspace }),
    refetchInterval: (query) =>
      query.state.data?.some((r) => r.status === "queued" || r.status === "running") ? 3000 : false,
  });
}

export function createRoutine(workspace: string, input: RoutineInput) {
  return request<Routine>("POST", base, input, { workspace });
}

export function updateRoutine(workspace: string, id: string, input: RoutineInput) {
  return request<Routine>("PATCH", one(id), input, { workspace });
}

export function deleteRoutine(workspace: string, id: string) {
  return request<void>("DELETE", one(id), undefined, { workspace });
}

export function runRoutine(workspace: string, id: string, dryRun: boolean) {
  return request<RoutineRun>("POST", `${one(id)}/runs`, { dry_run: dryRun }, { workspace });
}
