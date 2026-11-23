/*
 * How a simulation's checks and runs read in words: a check as a
 * sentence about the agent by name, and where a run stands while it is
 * going.
 */
import type { ChipColor } from "@/components/base/badges/chip";
import type { TestResult } from "@/lib/api/procedures";

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

export function statusNow(
  result: TestResult | null,
  busy: boolean,
  live?: LiveRun,
): { label: string; color: ChipColor } {
  if (!busy) {
    return statusOf(result);
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
