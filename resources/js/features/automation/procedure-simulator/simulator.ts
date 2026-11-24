/*
 * What the preview and the simulations share that is not on screen: the
 * criteria a simulation can make and how each reads as a sentence,
 * reading typed JSON, the criteria a chat saved as a simulation starts
 * with, finding the working copy's steps and actions, and the words and
 * looks both use.
 */
import type { ChipColor } from "@/components/base/badges/chip";
import type { OperatorStep } from "@/lib/api/operator";
import type { Assertion, SimPersona, Simulation, TestResult } from "@/lib/api/procedures";

import type { Choices } from "../choices";
import type { ChipSources } from "../procedure-writer/procedure-suggestions";
import { stepNumbers, walk } from "../procedures";
import { stepTitle } from "../step-words";
import type { useProcedureDraft } from "../use-procedure-draft";

/*
 * defaultAssertions are what a chat saved as a simulation checks: the
 * procedure matched, every action it called is called again, and it
 * ends the same way, handed over or not.
 */
export function defaultAssertions(turns: Simulation[]): Assertion[] {
  const out: Assertion[] = [{ type: "procedure_matched" }];
  const seen = new Set<string>();
  for (const turn of turns) {
    for (const call of turn.action_calls) {
      if (!call.blocked && !seen.has(call.tool)) {
        seen.add(call.tool);
        out.push({ type: "action_called", action: call.tool });
      }
    }
  }
  out.push(...branchesTaken(turns));
  const handoff = turns.find((turn) => turn.handoff !== null)?.handoff;
  out.push(
    handoff?.team_id
      ? { type: "handed_off", handed_off: true, team_id: handoff.team_id }
      : { type: "handed_off", handed_off: handoff !== undefined },
  );

  return out;
}

/*
 * branchesTaken are the sides the chat took at each branch, the last one
 * seen for each, so a saved chat keeps following the same path.
 */
function branchesTaken(turns: Simulation[]): Assertion[] {
  const sides = new Map<string, "then" | "else">();
  for (const turn of turns) {
    for (const step of turn.procedure_trace?.steps ?? []) {
      if (step.branch_taken) {
        sides.set(step.step_id, step.branch_taken);
      }
    }
  }

  return [...sides].map(([step_id, branch]) => ({ type: "branch_taken", step_id, branch }));
}

export const channelChoices: Choices = [
  ["widget", "Messenger"],
  ["email", "Email"],
  ["whatsapp", "WhatsApp"],
  ["slack", "Slack"],
  ["api", "API"],
];

export const hint = "text-body-2-regular text-text-tertiary";

export const fieldLabel = "text-body-2-medium text-text-primary";

export const fromCustomer =
  "ms-auto w-fit max-w-[85%] rounded-2xl bg-background-primary-default px-3 py-[11px] text-start text-body-regular wrap-break-word whitespace-pre-wrap text-text-primary shadow-card";

export const card = "rounded-2xl bg-background-primary-default shadow-card";

export function words(value: string): string {
  const spaced = value.replaceAll("_", " ");

  return spaced.charAt(0).toUpperCase() + spaced.slice(1);
}

/*
 * LiveStep is one thing dimmi did, as a run sends it while it happens.
 */
export interface LiveStep {
  tool: string;
  summary: string;
  ok: boolean;
  action?: boolean;
  refused?: boolean;
}

/*
 * liveStep reads a step sent while a run happens as a line of the
 * activity log, naming an action as the team wrote it.
 */
export function liveStep(step: LiveStep, sources: ChipSources): OperatorStep {
  if (!step.action) {
    return { tool: step.tool, summary: step.summary, ok: step.ok };
  }
  const name = actionName(sources, step.tool);

  return step.refused
    ? { tool: "action_blocked", summary: `Refused ${name}`, ok: false }
    : { tool: "action_call", summary: `${step.ok ? "Called" : "Failed to call"} ${name}`, ok: step.ok };
}

/*
 * actionName is an action's title as the team wrote it, by its tool
 * name or id.
 */
export function actionName(sources: ChipSources, key: string): string {
  const action = sources.actions.find((a) => a.id === key || a.name === key);

  return action?.title || action?.name || key;
}

/*
 * customerOf names who a persona is, a made-up customer when nobody was
 * picked.
 */
export function customerOf(persona: SimPersona): string {
  return persona.name || persona.email || "Preview customer";
}

export function criterionChoices(draft: Draft, sources: ChipSources): CriterionChoices {
  const numbers = stepNumbers(draft.definition.steps);
  const rules: [string, string][] = [];
  walk(draft.definition.steps, (step) => {
    if (step.kind === "branch" || step.kind === "condition") {
      rules.push([step.id, `${numbers.get(step.id) ?? ""} · ${stepTitle(step, sources)}`]);
    }
  });

  return {
    actions: sources.actions.map((a) => [a.name, a.title || a.name] as const),
    rules,
    teams: sources.teams.map((t) => [t.id, t.name] as const),
  };
}

/*
 * sentenceOf reads a criterion as its kind and what it claims, the way a
 * folded row shows it.
 */
export function sentenceOf(a: Assertion, choices: CriterionChoices): { kind: string; claim: string } {
  const action = nameIn(choices.actions, a.action, "an action");
  switch (a.type) {
    case "procedure_matched":
      return { kind: "Procedure", claim: "dimmi followed this procedure" };
    case "action_called":
      return { kind: "Action", claim: `${action} was called` };
    case "action_not_called":
      return { kind: "Action", claim: `${action} was not called` };
    case "action_order":
      return { kind: "Action", claim: `${action} ran before ${nameIn(choices.actions, a.before, "another")}` };
    case "branch_taken":
      return {
        kind: "Rule",
        claim: `step ${nameIn(choices.rules, a.step_id, "…")} took ${a.branch === "else" ? "if not" : "if so"}`,
      };
    case "handed_off":
      return (a.handed_off ?? true)
        ? {
            kind: "Outcome",
            claim: a.team_id ? `handed over to ${nameIn(choices.teams, a.team_id, "a team")}` : "handed over",
          }
        : { kind: "Outcome", claim: "not handed over to the team" };
    case "reply_contains":
      return { kind: "Reply", claim: a.text ? `says “${a.text}”` : "says…" };
    case "reply_not_contains":
      return { kind: "Reply", claim: a.text ? `never says “${a.text}”` : "never says…" };
    case "llm_rubric":
      return { kind: "Judged", claim: a.rubric || "meets a rubric…" };
    default:
      return { kind: "Check", claim: words(a.type) };
  }
}

/*
 * statusOf is how a simulation did when it last ran.
 */
export function statusOf(result: TestResult | null): { label: string; color: ChipColor } {
  if (!result) {
    return { label: "Not run", color: "neutral" };
  }

  if (result.error) {
    return { label: "Error", color: "orange" };
  }

  return result.passed ? { label: "Pass", color: "lime" } : { label: "Fail", color: "rose" };
}

export type Draft = ReturnType<typeof useProcedureDraft>;

/*
 * CriterionChoices are what a criterion's pickers choose from: the
 * actions, the steps that take a side, and the teams.
 */
export interface CriterionChoices {
  actions: Choices;
  rules: Choices;
  teams: Choices;
}

export function nameIn(choices: Choices, value: string | undefined, fallback: string): string {
  return choices.find(([id]) => id === value)?.[1] ?? value ?? fallback;
}
