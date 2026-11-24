/*
 * The studio's simulations of workflows and series. A workflow keeps
 * saved simulations, as an agent does: each plays an AI customer
 * through the draft or the live version, taps buttons, answers
 * questions and lets timeouts pass, and keeps the workflow's trace with
 * how it went. A series takes a dry simulation of one person: every
 * wait skipped and nothing sent, its extract and run action steps
 * answered by mocks. Every id is a public one (wf_, wfs_,
 * wfsr_, ser_, ctc_, cnv_).
 */
import { queryOptions } from "@tanstack/react-query";

import type { AgentSimulationHistory, AnswerAs } from "./agents";
import { request } from "./client";
import type { ActionMock, Assertion, ObservedCall, SimPersona } from "./procedures";

/* The checks only workflows have, beside an agent's. */
export type WorkflowCheckType = "workflow_started" | "step_reached" | "path_taken";

/* A workflow check: one of an agent's, or a workflow's own, whose branch is a path id. */
export type WorkflowAssertion = Omit<Assertion, "type" | "branch"> & {
  type: Assertion["type"] | WorkflowCheckType;
  branch?: string;
};

/*
 * One step a run took: how it went, the path it left by, and whether it
 * was skipped, as a wait the time passed on is. A step of a workflow a
 * run workflow step called sits one level deeper, with that workflow's
 * id and name.
 */
export interface DryStep {
  step_id: string;
  kind: string;
  outcome: string;
  path_id?: string;
  path_label?: string;
  skipped?: boolean;
  detail?: Record<string, unknown>;
  depth?: number;
  workflow_id?: string;
  workflow?: string;
}

/* An action the workflow would take; none of them reaches anyone. */
export interface DryAction {
  step_id: string;
  kind: string;
  summary: string;
  detail?: Record<string, unknown>;
  mocked?: boolean;
}

/* What the workflow did: whether the trigger started it, its steps, variables and actions, and how it ended. */
export interface DryTrace {
  verdict: { started: boolean; reason?: string; failed_rule?: string };
  steps: DryStep[];
  vars: Record<string, unknown>;
  actions: DryAction[];
  end: string;
}

/* One line of the chat: the customer, the workflow's own message, or dimmi inside an agent step. */
export interface ChatLine {
  from: "customer" | "workflow" | "agent";
  text: string;
  step_id?: string;
  choices?: string[];
  name?: string;
  /* The customer tapped one of the buttons. */
  tapped?: boolean;
  /* The customer said nothing and let the wait run out. */
  silent?: boolean;
}

/* dimmi's turn inside an agent step. */
export interface AgentTurn {
  message: string;
  confirm?: boolean;
  reply: string;
  outcome: string;
  calls: ObservedCall[];
  run_id: string;
  transfer?: { agent_id: string; name: string; from_name: string; summary: string } | null;
}

export interface WorkflowSimResult {
  passed: boolean;
  ran_at: string;
  version: AnswerAs;
  trace?: DryTrace;
  transcript: ChatLine[];
  turns: AgentTurn[];
  assertion_results: {
    type: string;
    passed: boolean;
    message: string;
    skipped?: boolean;
    unsure?: boolean;
  }[];
  error?: string;
}

export interface WorkflowSimulation {
  id: string;
  object: "workflow_simulation";
  workflow_id: string;
  name: string;
  source_conversation_id: string | null;
  persona: SimPersona;
  messages: { text: string; confirm?: boolean }[];
  /*
   * By step id: what an HTTP request or run action step answers (a run
   * action step also by action id), the values an extract step finds as
   * its body, or for a classify step the path it takes.
   */
  action_mocks: Record<string, ActionMock>;
  assertions: WorkflowAssertion[];
  scenario: string;
  draft_result: WorkflowSimResult | null;
  live_result: WorkflowSimResult | null;
  draft_history: AgentSimulationHistory | null;
  live_history: AgentSimulationHistory | null;
  simulated_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface WorkflowSimulationRun {
  id: string;
  object: "workflow_simulation_run";
  simulation_id: string;
  version: AnswerAs;
  passed: boolean;
  errored: boolean;
  counted: boolean;
  result: WorkflowSimResult;
  ran_at: string;
}

export type WorkflowSimulationInput = Partial<
  Pick<WorkflowSimulation, "name" | "persona" | "messages" | "action_mocks" | "assertions" | "scenario">
>;

function path(workflow: string): string {
  return `/workflows/${encodeURIComponent(workflow)}/simulations`;
}

export function workflowSimulationsQuery(workspace: string, workflow: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "workflows", workflow, "simulations"],
    queryFn: () => request<WorkflowSimulation[]>("GET", path(workflow), undefined, { workspace }),
  });
}

export function createWorkflowSimulation(
  workspace: string,
  workflow: string,
  input: WorkflowSimulationInput,
): Promise<WorkflowSimulation> {
  return request<WorkflowSimulation>("POST", path(workflow), input, { workspace });
}

export function importWorkflowSimulation(
  workspace: string,
  workflow: string,
  conversation: string,
): Promise<WorkflowSimulation> {
  return request<WorkflowSimulation>(
    "POST",
    `${path(workflow)}/from-conversation`,
    { conversation_id: conversation },
    { workspace },
  );
}

export function updateWorkflowSimulation(
  workspace: string,
  workflow: string,
  simulation: string,
  input: WorkflowSimulationInput,
): Promise<WorkflowSimulation> {
  return request<WorkflowSimulation>("PATCH", `${path(workflow)}/${encodeURIComponent(simulation)}`, input, {
    workspace,
  });
}

export function deleteWorkflowSimulation(workspace: string, workflow: string, simulation: string): Promise<unknown> {
  return request("DELETE", `${path(workflow)}/${encodeURIComponent(simulation)}`, undefined, { workspace });
}

export function runWorkflowSimulations(
  workspace: string,
  workflow: string,
  input: { simulation_ids?: string[]; version?: AnswerAs; stream_id?: string },
): Promise<WorkflowSimulation[]> {
  return request<WorkflowSimulation[]>("POST", `${path(workflow)}/run`, input, { workspace });
}

export function workflowSimulationRunsQuery(workspace: string, workflow: string, simulation: string, as: AnswerAs) {
  return queryOptions({
    queryKey: ["workspace", workspace, "workflows", workflow, "simulations", simulation, "runs", as],
    queryFn: () =>
      request<WorkflowSimulationRun[]>(
        "GET",
        `${path(workflow)}/${encodeURIComponent(simulation)}/runs?version=${as}`,
        undefined,
        { workspace },
      ),
  });
}

/* simHistoryOf is how a simulation's latest runs went against the copy asked for. */
export function simHistoryOf(sim: WorkflowSimulation, as: AnswerAs): AgentSimulationHistory | null {
  return as === "live" ? sim.live_history : sim.draft_history;
}

/* simResultOf is how a simulation last went against the copy asked for. */
export function simResultOf(sim: WorkflowSimulation, as: AnswerAs): WorkflowSimResult | null {
  return as === "live" ? sim.live_result : sim.draft_result;
}

/* A series' dry simulation of one person. */
export interface SeriesSimulation {
  object: "series_simulation";
  entered: boolean;
  reason?: string;
  steps: {
    node_id: string;
    kind: string;
    path?: string;
    result: string;
    skipped?: boolean;
    detail?: Record<string, unknown>;
  }[];
  messages: { node_id: string; kind: string; to?: string; subject?: string; body: string }[];
  actions: { node_id: string; kind: string; detail?: Record<string, unknown> }[];
  goal_reached: boolean;
  end: string;
}

export type Engagement = "opened" | "clicked" | "replied";

export interface SeriesSimulationInput {
  version?: AnswerAs;
  contact_id?: string;
  person?: { name?: string; email?: string; attributes?: Record<string, unknown> };
  events?: string[];
  engaged?: Engagement[];
  /* What extract and run action steps answer, by step id or action id, instead of the AI or the action. */
  action_mocks?: Record<string, ActionMock>;
}

export function simulateSeries(
  workspace: string,
  series: string,
  input: SeriesSimulationInput,
): Promise<SeriesSimulation> {
  return request<SeriesSimulation>("POST", `/series/${encodeURIComponent(series)}/simulations/run`, input, {
    workspace,
  });
}
