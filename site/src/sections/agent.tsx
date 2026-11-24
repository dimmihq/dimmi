import { cx } from "@/utils/cx";

import { Intro, Panel, Section, SplitHead } from "../components/container";
import { AgentDemoCard } from "../scenes/agent-demo";
import { useAgentStories } from "../scenes/use-agent-stories";

const tabs = [
  {
    title: "Links every source",
    body: "Reads your help center, links the page it used, and says when it does not know.",
  },
  {
    title: "Follows your policies",
    body: "Handles refunds and account changes in order, respects your limits, and gets tested before going live.",
  },
  {
    title: "Hands over with a summary",
    body: "Steps back when a teammate joins and leaves a quick note so nobody rereads 30 messages.",
  },
];

export function Agent() {
  const stories = useAgentStories();
  const { state, started, choose, duration } = stories;

  return (
    <Section anchor="agent" frameClassName="flex flex-col gap-16 md:gap-20">
      <Intro
        title="An AI agent you can trust near real customers."
        body="Most bots make things up and charge $0.99 per reply. dimmi cites your docs, follows your steps, and asks before risky actions."
      />
      <div className="flex flex-col gap-10">
        <SplitHead
          tag="AI agent"
          title="Real sources, simple steps, and honest pricing."
          body="Pick which channels it covers and what it reads. Bring your own OpenAI or Anthropic keys and cut AI costs by up to 83%."
          link={{ href: "/agent", label: "Explore the AI agent" }}
        />
        <Panel className="flex justify-center px-4 pt-10 sm:pt-14">
          <AgentDemoCard stories={stories} />
        </Panel>
        <div role="tablist" aria-label="What the agent does" className="grid gap-x-10 gap-y-6 md:grid-cols-3">
          {tabs.map((tab, i) => (
            <button
              key={tab.title}
              type="button"
              role="tab"
              aria-selected={state.active === i}
              onClick={() => choose(i)}
              className="group relative flex flex-col gap-1 border-t border-separator-border pt-6 text-start"
            >
              {state.active === i && started ? (
                <span
                  key={state.active}
                  className="absolute -top-px left-0 h-px bg-accent-500"
                  style={{ animation: `site-fill-x ${duration}ms linear forwards` }}
                />
              ) : null}
              <span
                className={cx(
                  "text-[16px] leading-7 transition-colors",
                  state.active === i ? "text-text-primary" : "text-text-tertiary group-hover:text-text-secondary",
                )}
              >
                {tab.title}
              </span>
              <span
                className={cx(
                  "text-[16px] leading-7 transition-colors",
                  state.active === i ? "text-text-secondary" : "text-text-tertiary/70",
                )}
              >
                {tab.body}
              </span>
            </button>
          ))}
        </div>
      </div>
    </Section>
  );
}
