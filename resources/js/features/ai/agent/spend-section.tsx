/*
 * What the agent may spend in a month, and what it has spent. With a
 * cap, the agent pauses when the month's spend reaches it and starts
 * again on the first; with no cap it never pauses for money. The bar
 * shows this month against the cap, with how its runs ended.
 */
import { useQuery } from "@tanstack/react-query";

import { Meter } from "@/components/base/meter/meter";
import { NumberField } from "@/components/base/number-field/number-field";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { Row, Section } from "@/features/settings/layout";
import { type AiAgent, type AiUsage, aiUsageQuery } from "@/lib/api/ai";

import { type AgentChange, usd } from "../format";

/*
 * defaultCap is the cap offered when a workspace with none sets one.
 */
const defaultCap = 25;

const capModes = { none: "none", cap: "cap" } as const;

/*
 * Tally is one count of how the month's answers ended.
 */
function Tally({ label, value }: { label: string; value: number }) {
  return (
    <div className="flex flex-col gap-0.5 rounded-xl bg-background-tertiary-default px-3 py-2">
      <span className="text-caption-1-medium text-text-secondary">{label}</span>
      <span className="text-headline-medium text-text-primary tabular-nums">{value.toLocaleString()}</span>
    </div>
  );
}

/*
 * MonthUsage is this month's spend against the cap, and its tallies.
 */
export function MonthUsage({ usage, cap }: { usage: AiUsage | undefined; cap: number | null }) {
  const spent = usage?.cost_micros ?? 0;
  const tone = cap && spent >= cap ? "error" : cap && spent >= cap * 0.8 ? "warning" : "accent";

  return (
    <div className="flex flex-col gap-3 py-3 pe-3">
      {cap ? (
        <Meter
          label="This month"
          value={Math.min(spent, cap)}
          maxValue={cap}
          tone={tone}
          valueLabel={`${usd(spent)} of ${usd(cap)}`}
          description={usage?.capped_at ? "Paused: the cap was reached. It answers again next month." : undefined}
        />
      ) : (
        <div className="flex items-baseline justify-between gap-3">
          <span className="text-body-regular text-text-primary">This month</span>
          <span className="text-body-medium text-text-secondary tabular-nums">{usd(spent)} spent</span>
        </div>
      )}
      <div className="grid grid-cols-3 gap-2">
        <Tally label="Answered" value={usage?.answered_count ?? 0} />
        <Tally label="Didn't know" value={usage?.unknown_count ?? 0} />
        <Tally label="Handed over" value={usage?.handoff_count ?? 0} />
      </div>
    </div>
  );
}

export function SpendSection({
  workspace,
  agent,
  canWrite,
  save,
}: {
  workspace: string;
  agent: AiAgent;
  canWrite: boolean;
  save: (change: AgentChange) => void;
}) {
  const { data: usage } = useQuery({ ...aiUsageQuery(workspace), enabled: workspace !== "" });
  const cap = agent.monthly_spend_cap_micros;
  const capped = cap !== null;

  return (
    <Section title="Spend" description="Your own keys are billed by the provider. The cap is your guard.">
      <Row
        label="Monthly cap"
        description={capped ? "It pauses when the month's spend reaches this." : "It never pauses for spend."}
      >
        <SegmentedControl
          aria-label="Monthly cap"
          selectedKeys={[capped ? "cap" : "none"]}
          isDisabled={!canWrite}
          onSelectionChange={(keys) => {
            if (keys.has("none") && capped) {
              save({ monthly_cap_usd: null });
            }
            if (keys.has("cap") && !capped) {
              save({ monthly_cap_usd: defaultCap });
            }
          }}
        >
          <SegmentedControlItem id={capModes.none}>No cap</SegmentedControlItem>
          <SegmentedControlItem id={capModes.cap}>Cap</SegmentedControlItem>
        </SegmentedControl>
        {capped ? (
          <NumberField
            aria-label="Monthly cap in US dollars"
            size="small"
            hideSteppers
            minValue={1}
            isDisabled={!canWrite}
            value={cap / 1_000_000}
            formatOptions={{ style: "currency", currency: "USD", maximumFractionDigits: 0 }}
            onChange={(dollars) => {
              if (Number.isFinite(dollars) && dollars > 0 && dollars * 1_000_000 !== cap) {
                save({ monthly_cap_usd: dollars });
              }
            }}
            className="w-28"
          />
        ) : null}
      </Row>
      <MonthUsage usage={usage} cap={cap} />
    </Section>
  );
}
