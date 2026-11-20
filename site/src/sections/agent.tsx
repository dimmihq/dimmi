import { cx } from "@/utils/cx";

import { Intro, Panel, Section, SplitHead } from "../components/container";
import { AgentDemoCard } from "../scenes/agent-demo";
import { useAgentStories } from "../scenes/use-agent-stories";

const tabs = [
  {
    title: "Answers from your knowledge",
    body: "It searches your help center and links the article it used. No made-up answers.",
  },
  { title: "Follows your procedures", body: "It runs the steps you wrote, and asks a human before the risky ones." },
  { title: "Knows when to stop", body: "When it shouldn't answer, it hands over with a summary for the right team." },
];

export function Agent() {
  const stories = useAgentStories();
  const { state, started, choose, duration } = stories;

  return (
    <Section anchor="agent" frameClassName="flex flex-col gap-16 md:gap-20">
      <Intro
        title="Let the agent take the first reply."
        body="It works in the same threads as your team, on the messenger, email, Slack, WhatsApp, and the phone."
      />
      <div className="flex flex-col gap-10">
        <SplitHead
          tag="AI agent"
          title="An agent that shows its work."
          body="You set what it knows, what it can do, and when it hands over. Bring your own model key and an AI conversation costs the same as a human one."
          link={{ href: "/agent", label: "See the AI agent" }}
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
