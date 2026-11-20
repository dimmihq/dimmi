/*
 * The agent's stories played in a conversation card. The hook owns which
 * story and frame are showing, so a page can drive tabs from the same
 * state; the card is the conversation window itself.
 */
import { cx } from "@/utils/cx";

import { AgentStory } from "./agent-scene";
import { STORIES } from "./agent-stories";
import type { useAgentStories } from "./use-agent-stories";

export function AgentDemoCard({
  stories,
  className,
}: {
  stories: ReturnType<typeof useAgentStories>;
  className?: string;
}) {
  const story = STORIES[stories.state.active];
  return (
    <div
      ref={stories.ref}
      className={cx(
        "flex w-full max-w-[560px] flex-col overflow-hidden rounded-t-2xl border border-b-0 border-border-button-default bg-background-full shadow-[0_24px_60px_-28px_rgb(0_0_0/0.25)]",
        className,
      )}
    >
      <div className="flex items-center justify-between border-b border-separator-border px-5 py-3.5">
        <span className="text-body-medium text-text-primary">Conversation</span>
        <span className="rounded-md bg-background-secondary-default px-2 py-0.5 text-caption-1-medium text-text-secondary">
          {story.channel}
        </span>
      </div>
      <div className="h-[440px] px-5 py-6 text-start">
        <AgentStory key={stories.state.active} story={story} frame={stories.state.frame} />
      </div>
    </div>
  );
}

/* One story's finished picture, still: for a card that shows a single moment. */
export function AgentStill({ story }: { story: number }) {
  const picked = STORIES[story];
  return (
    <div className="flex w-full max-w-[520px] flex-col overflow-hidden rounded-t-2xl border border-b-0 border-border-button-default bg-background-full shadow-[0_24px_60px_-28px_rgb(0_0_0/0.25)]">
      <div className="flex items-center justify-between border-b border-separator-border px-5 py-3">
        <span className="text-body-medium text-text-primary">Conversation</span>
        <span className="rounded-md bg-background-secondary-default px-2 py-0.5 text-caption-1-medium text-text-secondary">
          {picked.channel}
        </span>
      </div>
      <div className="h-[420px] px-5 py-5">
        <AgentStory story={picked} frame={picked.frames.length - 1} />
      </div>
    </div>
  );
}
