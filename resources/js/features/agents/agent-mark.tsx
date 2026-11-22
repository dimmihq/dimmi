/*
 * AgentMark is an agent's tinted square with its initials, the agent's
 * face until it has an avatar.
 */
import { CHIP_COLORS } from "@/components/base/badges/chip";
import { cx } from "@/utils/cx";

import type { MockAgent } from "./mock";

export function AgentMark({ agent, size = "md" }: { agent: MockAgent; size?: "sm" | "md" }) {
  return (
    <span
      aria-hidden
      className={cx(
        "flex shrink-0 items-center justify-center",
        size === "md" ? "size-8 rounded-lg text-body-2-medium" : "size-6 rounded-md text-caption-medium",
        CHIP_COLORS[agent.tint],
      )}
    >
      {agent.initials}
    </span>
  );
}
