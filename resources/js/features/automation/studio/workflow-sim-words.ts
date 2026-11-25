/*
 * How a workflow simulation reads in words: steps and paths by the
 * names the builder gives them, what a step found, saved or called,
 * checks as sentences, the steps a mock can answer for, and where a
 * simulation stands on its latest runs or while one is going.
 */
import type { ChipColor } from "@/components/base/badges/chip";
import type { AgentSimulationHistory } from "@/lib/api/agents";
import type { ActionMock } from "@/lib/api/procedures";
import type { DryStep, WorkflowAssertion, WorkflowSimResult } from "@/lib/api/workflow-simulations";
import type { Spec } from "@/lib/api/workflows";
import { formatRelative } from "@/lib/i18n/format";

import { choicesOf, sayOf } from "../../agents/agent-sim-words";
import { sentenceOf } from "../procedure-simulator/simulator";
import type { ChipSources } from "../procedure-writer/procedure-suggestions";
import { stepLook } from "../step-kinds";
import { findStep, pathName, type StepKind } from "../workflows";
import type { LiveWfRun } from "./use-workflow-live-runs";

/* The steps of the copy a simulation runs on, to name steps and paths by. */
export type Steps = Spec["steps"];

/* stepName is a step as the trace names it: its kind's name, or its id once the step is gone. */
export function stepName(steps: Steps, id: string | undefined): string {
  if (!id) {
    return "a step";
  }
  const step = findStep(steps, id);
  const look = step ? stepLook[step.kind as StepKind] : undefined;

  return look ? `“${look.name}”` : id;
}

/* pathNamed is a path of a step by its id, as the builder names it. */
export function pathNamed(steps: Steps, stepId: string | undefined, pathId: string | undefined): string {
  const path = stepId ? findStep(steps, stepId)?.paths?.find((p) => p.id === pathId) : undefined;

  return path ? pathName(path) || (pathId ?? "") : (pathId ?? "a path");
}

/* checkSentence reads a check as a sentence: a workflow's own about its steps, an agent's as an agent's reads. */
export function checkSentence(a: WorkflowAssertion, steps: Steps, sources: ChipSources): string {
  switch (a.type) {
    case "workflow_started":
      return (a.matched ?? true) ? "The workflow starts" : "The workflow does not start";
    case "step_reached":
      return `The workflow reaches ${stepName(steps, a.step_id)}`;
    case "path_taken":
      return `${stepName(steps, a.step_id)} takes the path ${pathNamed(steps, a.step_id, a.branch)}`;
    case "action_called":
    case "action_not_called": {
      const step = a.action ? findStep(steps, a.action) : undefined;
      const look = stepLook[(step?.kind ?? a.action) as StepKind];
      if (look) {
        return `“${look.name}” ${a.type === "action_called" ? "runs" : "does not run"}`;
      }
      break;
    }
  }

  return sayOf(
    sentenceOf({ ...a, branch: undefined } as Parameters<typeof sentenceOf>[0], choicesOf(sources)),
    "dimmi",
  );
}

export type CheckResult = WorkflowSimResult["assertion_results"][number];

/* lined is a run's results when they still line up with the checks, which they stop doing once checks change. */
export function lined(checks: WorkflowAssertion[], result: WorkflowSimResult | null): (CheckResult | undefined)[] {
  return result && result.assertion_results.length === checks.length ? result.assertion_results : [];
}

const historyWords: Record<AgentSimulationHistory["state"], { word: string; color: ChipColor }> = {
  passing: { word: "Passing", color: "lime" },
  flaky: { word: "Flaky", color: "yellow" },
  failing: { word: "Failing", color: "rose" },
};

/* resultStatus is how one run went. */
export function resultStatus(result: WorkflowSimResult | null): { label: string; color: ChipColor } {
  if (!result) {
    return { label: "Not run", color: "neutral" };
  }
  if (result.error) {
    return { label: "Error", color: "orange" };
  }

  return result.passed ? { label: "Pass", color: "lime" } : { label: "Fail", color: "rose" };
}

/*
 * simStanding is how a simulation stands on its latest runs on the
 * workflow as it is now, as an agent's reads: passing, flaky or failing,
 * or not run once the workflow or the simulation changed.
 */
export function simStanding(
  result: WorkflowSimResult | null,
  history: AgentSimulationHistory | null,
): { label: string; color: ChipColor } {
  if (result?.error) {
    return resultStatus(result);
  }
  if (!history) {
    return resultStatus(null);
  }
  const { word, color } = historyWords[history.state];

  return { label: `${word} · ${history.passed} of ${history.total}`, color };
}

/* ranWhen says when a simulation last ran, and whether that was before the last change. */
export function ranWhen(result: WorkflowSimResult | null, history: AgentSimulationHistory | null): string {
  if (!result) {
    return "";
  }

  return ` · last run ${formatRelative(result.ran_at)}${history || result.error ? "" : ", before the last change"}`;
}

/* liveStatus is where a run stands while it is going. */
export function liveStatus(live?: LiveWfRun): { label: string; color: ChipColor } {
  if (live?.result) {
    return resultStatus(live.result);
  }
  if (live?.judging) {
    return { label: "Judging", color: "blue" };
  }
  const at = live && live.steps.length > 0 ? ` · step ${live.steps.length}` : "";

  return { label: `Running${at}`, color: "blue" };
}

/* shownValue is a value as a short text. */
function shownValue(value: unknown): string {
  return typeof value === "string" ? value : JSON.stringify(value);
}

/* pairs reads an object's values as "key: value", run variables by their own key. */
function pairs(values: unknown): string {
  if (!values || typeof values !== "object") {
    return "";
  }

  return Object.entries(values as Record<string, unknown>)
    .map(([key, value]) => `${key.replace(/^run\.vars\./, "")}: ${shownValue(value)}`)
    .join(", ");
}

const callRefused: Record<string, string> = {
  depth_limit: "Calls nest five deep at most, so it did not run",
  reference_missing: "That workflow is not live, so it did not run",
  customer_facing_from_background: "A background workflow cannot run one that talks to the customer",
};

/* waitNote says whether the customer interrupted a wait or the time passed. */
function waitNote(done: DryStep): string {
  if (done.outcome === "interrupted") {
    return `The customer wrote first: “${shownValue(done.detail?.message ?? "")}”`;
  }

  return done.skipped ? "The time passed" : "";
}

/* foundNote says what an extract step found and what it was unsure of, and whether a mock answered. */
export function foundNote(detail: Record<string, unknown>): string {
  const found = pairs(detail.values);
  const unsure = Array.isArray(detail.unsure) ? detail.unsure.join(", ") : "";
  const parts = [found ? `Found ${found}` : "", unsure ? `Not sure of ${unsure}` : ""].filter(Boolean);

  return parts.join(" · ") + (parts.length > 0 && detail.mocked ? " (mocked)" : "");
}

/* actionNote says what a run action step saved, or the status its mock failed with. */
function actionNote(done: DryStep): string {
  const detail = done.detail ?? {};
  if (done.outcome === "failed") {
    return detail.status ? `The action answered ${shownValue(detail.status)}` : "The action failed";
  }
  const saved = pairs(detail.saved);

  return saved ? `Saved ${saved}` : "";
}

/* callNote says which workflow a call ran, or why it did not. */
function callNote(done: DryStep): string {
  const detail = done.detail ?? {};
  const name = shownValue(detail.workflow ?? detail.workflow_id ?? "the workflow");
  const said: Record<string, string> = {
    completed: `Ran “${name}”, then came back`,
    calling: `Running “${name}”`,
    ...callRefused,
  };

  return said[done.outcome] ?? "";
}

/*
 * stepNote says what a step did beyond the path it took: the message
 * that interrupted a wait, the values an extract step found, what an
 * action saved or how its mock failed, and the workflow a call ran.
 */
export function stepNote(done: DryStep): string {
  const notes: Record<string, (d: DryStep) => string> = {
    wait: waitNote,
    extract: (d) => foundNote(d.detail ?? {}),
    run_action: actionNote,
    run_workflow: callNote,
  };

  return notes[done.kind]?.(done) ?? "";
}

/* The kinds of step a simulation's mocks answer for: calls that would leave dimmi, and the AI's extraction. */
const mockKinds = ["http_request", "run_action", "extract"] as const;

export type MockKind = (typeof mockKinds)[number];

/* mockableSteps lists every step a mock can answer for, in the order they appear. */
export function mockableSteps(steps: Steps): { id: string; kind: MockKind }[] {
  const out: { id: string; kind: MockKind }[] = [];
  const walk = (list: Steps) => {
    for (const step of list) {
      if ((mockKinds as readonly string[]).includes(step.kind)) {
        out.push({ id: step.id, kind: step.kind as MockKind });
      }
      for (const path of step.paths ?? []) {
        walk(path.steps ?? []);
      }
    }
  };
  walk(steps);

  return out;
}

/* MockDraft is one step's mock as the editor holds it: the status and the body as typed. */
export interface MockDraft {
  status: string;
  body: string;
}

/* mockDrafts are the saved mocks of the steps, as the editor starts from them. */
export function mockDrafts(mocks: Record<string, ActionMock>, ids: string[]): Record<string, MockDraft> {
  const out: Record<string, MockDraft> = {};
  for (const id of ids) {
    const mock = mocks[id];
    out[id] = {
      status: mock?.status ? String(mock.status) : "",
      body: mock?.body === undefined ? "" : JSON.stringify(mock.body, null, 2),
    };
  }

  return out;
}

/*
 * mocksOf is the mocks to save: the edited steps' mocks over the saved
 * ones, an emptied step's mock dropped, and the steps whose body is not
 * JSON or whose status is not one, which hold the save back.
 */
export function mocksOf(
  saved: Record<string, ActionMock>,
  drafts: Record<string, MockDraft>,
): { mocks: Record<string, ActionMock>; bad: string[] } {
  const mocks = { ...saved };
  const bad: string[] = [];
  for (const [id, draft] of Object.entries(drafts)) {
    const status = draft.status.trim();
    const body = draft.body.trim();
    delete mocks[id];
    if (!status && !body) {
      continue;
    }
    const code = Number(status);
    if (status && (!Number.isInteger(code) || code < 100 || code > 599)) {
      bad.push(id);
      continue;
    }
    const mock: ActionMock = status ? { status: code } : {};
    if (body) {
      try {
        mock.body = JSON.parse(body);
      } catch {
        bad.push(id);
        continue;
      }
    }
    mocks[id] = mock;
  }

  return { mocks, bad };
}
