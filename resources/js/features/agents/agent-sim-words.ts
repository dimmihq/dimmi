/*
 * How a simulation's checks and runs read in words: a check as a
 * sentence about the agent by name, and where a run stands while it is
 * going.
 */
import type { ChipColor } from "@/components/base/badges/chip";
import type { AgentSimulationHistory } from "@/lib/api/agents";
import type { TestResult } from "@/lib/api/procedures";
import { formatRelative } from "@/lib/i18n/format";

import { type CriterionChoices, statusOf, words } from "../automation/procedure-simulator/simulator";
import type { LiveRun } from "../automation/procedure-simulator/use-live-runs";
import type { ChipSources } from "../automation/procedure-writer/procedure-suggestions";

export function sayOf(said: { kind: string; claim: string }, name: string): string {
  const claim = said.claim.replace(/\bdimmi\b/g, name);
  if (said.kind === "Reply") {
    return `The reply ${claim}`;
  }

  return claim.startsWith(name) ? claim : words(claim);
}

export function choicesOf(sources: ChipSources): CriterionChoices {
  return {
    actions: sources.actions.map((a) => [a.name, a.title || a.name] as const),
    rules: [],
    teams: sources.teams.map((t) => [t.id, t.name] as const),
  };
}

const historyWords: Record<AgentSimulationHistory["state"], { word: string; color: ChipColor }> = {
  passing: { word: "Passing", color: "lime" },
  flaky: { word: "Flaky", color: "yellow" },
  failing: { word: "Failing", color: "rose" },
};

/*
 * How a simulation stands on its latest runs: passing, flaky or
 * failing, with how many passed of how many. A run that could not
 * finish says nothing about the agent, so it shows as itself. With no
 * run on the agent as it stands now, it reads as not run, whatever an
 * older run said.
 */
export function standing(
  result: TestResult | null,
  history: AgentSimulationHistory | null,
): { label: string; color: ChipColor } {
  if (result?.error) {
    return statusOf(result);
  }
  if (!history) {
    return statusOf(null);
  }
  const { word, color } = historyWords[history.state];

  return { label: `${word} · ${history.passed} of ${history.total}`, color };
}

/* lastRun says when a simulation last ran, and whether that was before the agent last changed. */
export function lastRun(result: TestResult | null, history: AgentSimulationHistory | null): string {
  if (!result) {
    return "";
  }
  const stale = history || result.error ? "" : ", before the last change";

  return ` · last run ${formatRelative(result.ran_at)}${stale}`;
}

export function statusNow(
  result: TestResult | null,
  busy: boolean,
  live?: LiveRun,
  history: AgentSimulationHistory | null = null,
): { label: string; color: ChipColor } {
  if (!busy) {
    return standing(result, history);
  }
  if (live?.result) {
    return statusOf(live.result);
  }
  if (live?.judging) {
    return { label: "Judging", color: "blue" };
  }
  const turn = live && live.turns.length > 0 ? ` · turn ${live.turns.length}` : "";

  return { label: `Running${turn}`, color: "blue" };
}

/*
 * judgedOf is the line under a check after a run: why it failed, or the judge's reason for a
 * written check that passed. A check with nothing more to say than that it was met shows no line.
 */
export function judgedOf(
  outcome: TestResult["assertion_results"][number] | undefined,
): { text: string; failed: boolean } | null {
  if (!outcome || outcome.skipped || !outcome.message) {
    return null;
  }
  if (!outcome.passed) {
    return { text: outcome.message, failed: !outcome.unsure };
  }
  if (outcome.type !== "llm_rubric" || outcome.message === "Met.") {
    return null;
  }

  return { text: outcome.message, failed: false };
}
