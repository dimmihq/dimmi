/*
 * Folding the lines of an activity log that repeat one after another.
 */
import type { OperatorStep } from "@/lib/api/operator";

/*
 * merged folds a run of identical lines into one that says how many
 * times it happened, so a step repeated in a loop reads as one line.
 */
export function merged(steps: OperatorStep[]): OperatorStep[] {
  const out: OperatorStep[] = [];
  let repeats = 1;
  for (const step of steps) {
    const before = out.at(-1);
    if (before && before.tool === step.tool && before.summary.replace(/ ×\d+$/, "") === step.summary) {
      repeats++;
      out[out.length - 1] = { ...before, summary: `${step.summary} ×${repeats}` };
      continue;
    }
    repeats = 1;
    out.push(step);
  }

  return out;
}
