/*
 * AgentMark is an agent's tinted square with its initials, the agent's
 * face until it has an avatar. The default agent is lime; every other
 * agent takes a tint from its id, so it keeps its colour when renamed.
 */
import { CHIP_COLORS, type ChipColor } from "@/components/base/badges/chip";
import { cx } from "@/utils/cx";

const tints: ChipColor[] = ["purple", "blue", "orange", "rose", "yellow"];

export interface Marked {
  id: string;
  name: string;
  is_default?: boolean;
}

function tintOf(agent: Marked): ChipColor {
  if (agent.is_default) {
    return "lime";
  }
  let hash = 0;
  for (const char of agent.id) {
    hash = (hash * 31 + char.charCodeAt(0)) >>> 0;
  }

  return tints[hash % tints.length] ?? "purple";
}

function initialsOf(name: string): string {
  const words = name
    .trim()
    .split(/[\s&]+/)
    .filter(Boolean);
  if (words.length > 1) {
    return (words[0].charAt(0) + words[1].charAt(0)).toUpperCase();
  }

  return name.trim().charAt(0).toUpperCase() || "?";
}

export function AgentMark({ agent, size = "md" }: { agent: Marked; size?: "sm" | "md" }) {
  return (
    <span
      aria-hidden
      className={cx(
        "flex shrink-0 items-center justify-center",
        size === "md" ? "size-8 rounded-lg text-body-2-medium" : "size-6 rounded-md text-caption-medium",
        CHIP_COLORS[tintOf(agent)],
      )}
    >
      {initialsOf(agent.name)}
    </span>
  );
}
