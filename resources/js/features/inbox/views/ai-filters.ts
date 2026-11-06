/*
 * The ways a list narrows by the agent's part in each conversation.
 */
import type { AiFilter } from "@/lib/api/inbox";

export const aiFilters: { value: AiFilter | undefined; label: string }[] = [
  { value: undefined, label: "All conversations" },
  { value: "any", label: "Agent involved" },
  { value: "active", label: "Agent answering" },
  { value: "handed_off", label: "Handed over by agent" },
];

/*
 * isAiFilter says whether a value read from the address is a filter.
 */
export function isAiFilter(value: unknown): value is AiFilter {
  return value === "any" || value === "active" || value === "handed_off";
}
