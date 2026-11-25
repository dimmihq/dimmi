/*
 * The agent's part in a conversation, on its list row and in its
 * header: a small chip while the agent answers, and another once it
 * handed the conversation to the team, or a teammate or a workflow
 * turned AI off. A conversation the agent never
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
 * room, such as the conversation's header, falls back to the short
 * label when that header's container runs narrow, and to the icon alone
 * when it runs narrower still.
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

const turnedOff = { label: "AI off", long: "AI turned off", color: "neutral" as const };

export function AiBadge({
  state,
  reason,
  long = false,
}: {
  state: AiState | undefined;
  reason?: string | null;
  long?: boolean;
}) {
  if (!state || state === "off") {
    return null;
  }
  const look = state === "handed_off" && reason?.startsWith("turned_off") ? turnedOff : looks[state];

  return (
    <Chip variant="caption" color={look.color} className="shrink-0 gap-0.5" title={look.long}>
      <RiSparkling2Line aria-hidden className="size-3" />
      {long ? (
        <>
          <span className="@max-xl:hidden">{look.long}</span>
          <span className="@max-sm:sr-only @xl:hidden">{look.label}</span>
        </>
      ) : (
        look.label
      )}
    </Chip>
  );
}
