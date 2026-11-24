/*
 * How a workflow simulation reads in words: steps and paths by the
 * names the builder gives them, checks as sentences, and where a
 * simulation stands on its latest runs or while one is going.
 */
import type { ChipColor } from "@/components/base/badges/chip";
import type { AgentSimulationHistory } from "@/lib/api/agents";
import type { WorkflowAssertion, WorkflowSimResult } from "@/lib/api/workflow-simulations";
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
