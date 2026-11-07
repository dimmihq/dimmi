/*
 * The agent's part in a conversation, on its list row and in its
 * header: a small chip while the agent answers, and another once it
 * handed the conversation to the team. A conversation the agent never
 * touched shows nothing. A conversation with actions waiting for a
 * teammate's approval shows how many, so they are found from the list.
 */
import { RiShieldCheckLine, RiSparkling2Line } from "@remixicon/react";

import { Chip } from "@/components/base/badges/chip";
import type { AiState } from "@/lib/api/inbox";

const looks: Record<Exclude<AiState, "off">, { label: string; long: string; color: "purple" | "neutral" }> = {
  active: { label: "AI", long: "AI answering", color: "purple" },
  handed_off: { label: "Handed over", long: "Handed over by AI", color: "neutral" },
};

/*
 * AiBadge shows the agent's state; long spells it out where there is
 * room, such as the conversation's header, and falls back to the short
 * label when that header's container runs narrow.
 */
export function ApprovalBadge({ count }: { count: number | undefined }) {
  if (!count) {
    return null;
  }
  const label = count === 1 ? "1 action waits for approval" : `${count} actions wait for approval`;

  return (
    <Chip variant="caption" color="orange" className="shrink-0 gap-0.5" title={label}>
      <RiShieldCheckLine aria-hidden className="size-3" />
      <span className="sr-only">{label}</span>
      <span aria-hidden>{count}</span>
    </Chip>
  );
}

export function AiBadge({ state, long = false }: { state: AiState | undefined; long?: boolean }) {
  if (!state || state === "off") {
    return null;
  }
  const look = looks[state];

  return (
    <Chip variant="caption" color={look.color} className="shrink-0 gap-0.5" title={look.long}>
      <RiSparkling2Line aria-hidden className="size-3" />
      {long ? (
        <>
          <span className="@max-md:hidden">{look.long}</span>
          <span className="@md:hidden">{look.label}</span>
        </>
      ) : (
        look.label
      )}
    </Chip>
  );
}
