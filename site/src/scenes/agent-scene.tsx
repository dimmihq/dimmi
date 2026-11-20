/*
 * Plays one agent story: frame n shows the story's first n+1 pieces.
 * Procedure frames replace each other; the thinking indicator leaves
 * once the answer arrives.
 */
import { AnimatePresence } from "motion/react";
import type { ReactNode } from "react";

import type { Story } from "./agent-stories";

export function AgentStory({ story, frame }: { story: Story; frame: number }) {
  // Procedure frames replace each other; everything else stacks.
  const shown = story.frames.slice(0, frame + 1).filter((node, i, all) => {
    const key = String((node as { key?: string }).key ?? "");
    if (!key.startsWith("p")) {
      return true;
    }
    const later = all.slice(i + 1).some((n) => String((n as { key?: string }).key ?? "").startsWith("p"));
    return !later;
  });
  const thinkingDone = (node: ReactNode, i: number) =>
    String((node as { key?: string }).key) === "t" && i < shown.length - 1;

  return (
    <div className="flex h-full flex-col justify-start gap-4 overflow-hidden">
      <AnimatePresence initial={false} mode="popLayout">
        {shown.filter((node, i) => !thinkingDone(node, i))}
      </AnimatePresence>
    </div>
  );
}
