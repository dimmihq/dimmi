/*
 * A step in plain words, for the places that show a step without the
 * writer: the history's comparison and the analytics.
 */
import type { ProcedureStep } from "@/lib/api/procedures";

import { type ChipSources, labelOf } from "./procedure-writer/procedure-suggestions";
import { chipsOf, marker } from "./procedure-writer/tokens";

export const kindWords: Record<string, string> = {
  instruction: "Instruction",
  condition: "Condition",
  action: "Action",
  branch: "Rule",
  sub_procedure: "Sub-procedure",
};

/*
 * stepTitle is a step's words with each chip read as what it points at.
 * A step without words is named by the sub-procedure it follows, or by
 * its kind.
 */
export function stepTitle(step: ProcedureStep | undefined, sources: ChipSources): string {
  if (!step) {
    return "Removed step";
  }
  const text = chipsOf(step.text).reduce(
    (line, chip) => line.replaceAll(marker(chip), labelOf(sources, chip) || chip.ref),
    step.text,
  );
  const followed = sources.procedures.find((p) => p.id === step.sub_procedure_id)?.name;

  return text.trim() || followed || kindWords[step.kind] || "Step";
}
