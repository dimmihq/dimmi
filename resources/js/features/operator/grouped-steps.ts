/*
 * Sorting a run's steps by who took them. A routine's run can split its
 * work across subagents that work at the same time, so their steps
 * arrive mixed; each subagent's steps are gathered under it, where its
 * first step landed, and the main agent's steps stay in their order.
 */
import type { OperatorStep } from "@/lib/api/operator";

import { merged } from "./merged-steps";

export interface SubagentGroup {
  subagent: number;
  /* The few words the main agent named this part with. */
  title: string;
  steps: OperatorStep[];
  /* The subagent's report once it finished, and whether it finished well. */
  report: string | null;
  ok: boolean;
}

export type LogItem = { kind: "step"; step: OperatorStep } | { kind: "subagent"; group: SubagentGroup };

/*
 * grouped lists what a log shows: the main agent's steps, repeats folded,
 * and one entry per subagent holding its own steps.
 */
export function grouped(steps: OperatorStep[]): LogItem[] {
  const groups = new Map<number, SubagentGroup>();
  const order: (OperatorStep | SubagentGroup)[] = [];
  for (const step of steps) {
    if (!step.subagent) {
      order.push(step);
      continue;
    }
    let group = groups.get(step.subagent);
    if (!group) {
      group = { subagent: step.subagent, title: `Subagent ${step.subagent}`, steps: [], report: null, ok: true };
      groups.set(step.subagent, group);
      order.push(group);
    }
    if (step.tool === "subagent") {
      group.title = step.summary;
    } else if (step.tool === "subagent_report") {
      group.report = step.summary;
      group.ok = step.ok;
    } else {
      group.steps.push(step);
    }
  }

  const out: LogItem[] = [];
  let run: OperatorStep[] = [];
  const flush = () => {
    for (const step of merged(run)) {
      out.push({ kind: "step", step });
    }
    run = [];
  };
  for (const entry of order) {
    if ("tool" in entry) {
      run.push(entry);
      continue;
    }
    flush();
    out.push({ kind: "subagent", group: { ...entry, steps: merged(entry.steps) } });
  }
  flush();

  return out;
}

/*
 * stepCount is how many steps a run took, subagents' steps included and
 * the lines that only name a subagent or carry its report left out.
 */
export function stepCount(steps: OperatorStep[]): number {
  return steps.filter((step) => step.tool !== "subagent" && step.tool !== "subagent_report").length;
}
