/*
 * What the preview and the simulations both show: what dimmi did in a
 * reply.
 */
import { Chip, type ChipColor } from "@/components/base/badges/chip";
import type { ObservedCall } from "@/lib/api/procedures";

import type { ChipSources } from "../procedure-writer/procedure-suggestions";
import { actionName, words } from "./simulator";

/*
 * The outcomes of a reply that simply answered. They say nothing the
 * reply does not, so they get no chip.
 */
const routine = ["answered", "greeted"];

/*
 * Did lists what dimmi did in one reply, as chips under it: the
 * procedure it followed, the actions it called, a handover, or an
 * outcome worth a second look. A reply that only answered shows none.
 */
export function Did({
  matched,
  calls,
  handoff,
  outcome,
  sources,
}: {
  matched?: boolean;
  calls: ObservedCall[];
  handoff?: string;
  outcome: string;
  sources: ChipSources;
}) {
  const chips: { key: string; color: ChipColor; text: string }[] = [];
  if (matched) {
    chips.push({ key: "matched", color: "blue", text: "Followed this procedure" });
  }
  for (const call of calls) {
    chips.push({
      key: `${call.tool}:${call.blocked ? "b" : "c"}`,
      color: call.blocked ? "rose" : "neutral",
      text: `${call.blocked ? "Blocked" : "Called"} ${actionName(sources, call.action_id || call.tool)}`,
    });
  }
  if (handoff) {
    chips.push({ key: "handoff", color: "orange", text: `Handed over: ${words(handoff).toLowerCase()}` });
  }
  if (chips.length === 0 && !routine.includes(outcome)) {
    chips.push({ key: "outcome", color: "neutral", text: words(outcome) });
  }
  if (chips.length === 0) {
    return null;
  }

  return (
    <span className="flex flex-wrap gap-1">
      {chips.map((chip) => (
        <Chip key={chip.key} variant="caption" color={chip.color} className="py-0.5">
          {chip.text}
        </Chip>
      ))}
    </span>
  );
}
