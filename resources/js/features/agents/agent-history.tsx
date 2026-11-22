/*
 * An agent's History and Analytics, laid out like a procedure's: compare
 * any two versions and restore one; then how the agent did over a range.
 * Mockup: sample versions and numbers.
 */
import { RiArrowRightLine } from "@remixicon/react";
import { useState } from "react";

import { StatCards } from "@/components/application/dashboard/stat-cards";
import { Chip } from "@/components/base/badges/chip";
import { Action } from "@/features/settings/controls";
import { Section } from "@/features/settings/layout";

import { Pick } from "../automation/pick";
import { type Coached, CoachModal } from "./coach-modal";
import { type MockAgent, recent, topics, versions } from "./mock";

const row = "flex items-center gap-3 border-b border-separator-border py-3 pe-3 last:border-b-0";

export function AgentHistory() {
  return (
    <div className="mx-auto flex w-full max-w-[760px] flex-col gap-8 px-6 pt-6 pb-16">
      <Section
        title="Compare"
        action={
          <span className="flex items-center gap-1.5">
            <Pick label="Compare from" choices={[["3", "Version 3"]]} value="3" onChange={() => undefined} />
            <RiArrowRightLine aria-hidden className="size-4 shrink-0 text-foreground-icon-tertiary" />
            <Pick label="Compare to" choices={[["draft", "Working copy"]]} value="draft" onChange={() => undefined} />
          </span>
        }
      >
        <div className="flex flex-col gap-2 py-3 pe-3 text-body-2-regular">
          <span className="flex flex-wrap items-baseline gap-2">
            <span className="w-24 shrink-0 text-text-tertiary">Guidance</span>
            <span className="text-text-error-primary line-through">Never share prices.</span>
            <span className="text-status-lime-text">Quote prices only from Pricing &amp; plans.</span>
          </span>
          <span className="flex flex-wrap items-baseline gap-2">
            <span className="w-24 shrink-0 text-text-tertiary">Knowledge</span>
            <span className="text-status-lime-text">Added Case studies</span>
          </span>
        </div>
      </Section>
      <Section title="Versions">
        {versions.map((v, index) => (
          <div
            key={v.version}
            className="flex items-start gap-3 border-b border-separator-border py-3 pe-2 last:border-b-0"
          >
            <div className="flex min-w-0 flex-1 flex-col gap-1">
              <span className="flex items-center gap-2">
                <span className="text-body-2-medium text-text-primary">Version {v.version}</span>
                {index === 0 ? (
                  <Chip variant="caption" color="lime" className="py-0.5">
                    Live
                  </Chip>
                ) : null}
                <span className="text-body-2-regular text-text-tertiary">{v.when}</span>
              </span>
              <span className="text-body-2-regular text-text-secondary">{v.note}</span>
              <span className="text-body-2-regular text-text-tertiary">
                {v.by} · simulations {v.tests}
              </span>
            </div>
            {index === 0 ? null : <Action>Restore</Action>}
          </div>
        ))}
      </Section>
    </div>
  );
}

export function AgentAnalytics({ agent }: { agent: MockAgent }) {
  const [days, setDays] = useState("30");
  const [coaching, setCoaching] = useState<Coached | null>(null);

  return (
    <div className="@container mx-auto flex w-full max-w-[760px] flex-col gap-8 px-6 pt-6 pb-16">
      <div className="flex flex-col gap-3">
        <span className="flex gap-2">
          <Pick
            label="Range"
            choices={[
              ["7", "Last 7 days"],
              ["30", "Last 30 days"],
              ["90", "Last 90 days"],
            ]}
            value={days}
            onChange={setDays}
          />
        </span>
        <StatCards
          variant="compact"
          className="gap-2"
          stats={[
            { label: "Conversations", value: String(agent.conversations ?? 0), caption: "Answered by this agent" },
            { label: "Resolved by AI", value: `${agent.resolved ?? 0}%`, caption: "123 of 212" },
            { label: "Handed over", value: "31%", caption: "66 to the Sales team" },
            { label: "Transferred", value: "11%", caption: "23 to dimmi" },
          ]}
        />
      </div>
      <Section title="What customers asked">
        {topics.map((t) => (
          <div key={t.name} className={row}>
            <span className="min-w-0 flex-1 truncate text-body-regular text-text-primary">{t.name}</span>
            <span className="text-body-2-regular text-text-tertiary tabular-nums">{t.count}</span>
          </div>
        ))}
      </Section>
      <Section
        title="Recent conversations"
        description="Coach the agent from any reply it got wrong. The same button sits on its replies in the inbox."
      >
        {recent.map((r) => (
          <div key={r.number} className={row}>
            <span className="flex min-w-0 flex-1 flex-col">
              <span className="truncate text-body-regular text-text-primary">
                #{r.number} · {r.who}
              </span>
              <span className="truncate text-body-2-regular text-text-tertiary">{r.customer}</span>
            </span>
            <Chip variant="caption" color={r.tint} className="py-0.5">
              {r.outcome}
            </Chip>
            <Action onPress={() => setCoaching(r)}>Coach</Action>
          </div>
        ))}
      </Section>
      {coaching ? <CoachModal agentName={agent.name} coached={coaching} onClose={() => setCoaching(null)} /> : null}
    </div>
  );
}
