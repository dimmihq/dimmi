/*
 * The comparison as a picture: pick a month's volume, and dimmi and each
 * competitor are drawn as bars on one scale, cheapest first, with the
 * saving against the cheapest competitor called out above them.
 */
import { type ReactNode, useState } from "react";

import { cx } from "@/utils/cx";

import { COMPETITORS, competitorBill, SCENARIOS } from "./competitors";
import { PlanTip } from "./plan-tip";
import { bill, dollars } from "./prices";

export function CompareBars() {
  const [picked, setPicked] = useState(2);
  const scenario = SCENARIOS[picked];
  const usage = { conversations: scenario.conversations, aiShare: scenario.aiShare, operator: 0, ownModel: false };
  const ours = bill(usage).total;
  const rows = COMPETITORS.map((c) => ({
    competitor: c,
    name: c.name,
    total: competitorBill(c, { ...usage, seats: scenario.seats }),
  })).sort((a, b) => a.total - b.total);
  const cheapest = rows[0];
  const highest = rows[rows.length - 1].total;
  const percent = Math.round(((cheapest.total - ours) / cheapest.total) * 100);

  return (
    <div className="flex flex-col gap-8 rounded-xl border border-border-button-default bg-background-full p-5 sm:p-8">
      <div className="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
        <div className="flex flex-col gap-1">
          <span className="text-[14px] leading-5 text-text-secondary">
            {scenario.conversations.toLocaleString("en-US")} conversations, {scenario.seats} seats
          </span>
          <span className="text-[32px] leading-10 font-medium tracking-[-0.02em] text-text-primary tabular-nums">
            {percent}% less <span className="text-text-tertiary">than {cheapest.name}, the cheapest</span>
          </span>
        </div>
        <fieldset
          aria-label="Conversations a month"
          className="m-0 grid min-w-0 grid-cols-5 gap-1 rounded-lg border-0 bg-background-secondary-default p-1 lg:flex"
        >
          {SCENARIOS.map((s, i) => (
            <button
              key={s.conversations}
              type="button"
              aria-pressed={i === picked}
              onClick={() => setPicked(i)}
              className={cx(
                "cursor-pointer rounded-md px-1 py-1.5 text-[13px] sm:px-3 leading-5 font-medium tabular-nums outline-none transition-colors focus-visible:ring-2 focus-visible:ring-border-focus-ring",
                i === picked
                  ? "bg-background-full text-text-primary shadow-xs"
                  : "text-text-secondary hover:text-text-primary",
              )}
            >
              {s.conversations.toLocaleString("en-US")}
            </button>
          ))}
        </fieldset>
      </div>
      <ul className="flex flex-col gap-3">
        <Row name="dimmi" total={ours} highest={highest} own />
        {rows.map((row) => (
          <Row
            key={row.name}
            name={row.name}
            total={row.total}
            highest={highest}
            tip={
              <PlanTip
                competitor={row.competitor}
                usage={{ ...usage, seats: scenario.seats }}
                cheapest={row.total}
                ours={ours}
              />
            }
          />
        ))}
      </ul>
    </div>
  );
}

function Row({
  name,
  total,
  highest,
  own = false,
  tip,
}: {
  name: string;
  total: number;
  highest: number;
  own?: boolean;
  tip?: ReactNode;
}) {
  return (
    <li className="grid grid-cols-[104px_1fr_72px] items-center gap-3 text-[15px] leading-6 sm:grid-cols-[120px_1fr_88px]">
      <span className="flex min-w-0 items-center gap-1">
        <span className={cx("truncate", own ? "font-medium text-text-primary" : "text-text-secondary")}>{name}</span>
        {tip}
      </span>
      <span className="h-3 overflow-hidden rounded-full bg-background-secondary-default">
        <span
          className={cx(
            "block h-full rounded-full transition-[width] duration-500 ease-out",
            own ? "bg-accent-500" : "bg-foreground-icon-tertiary/50",
          )}
          style={{ width: `${Math.max(1.5, (total / highest) * 100)}%` }}
        />
      </span>
      <span className={cx("text-end tabular-nums", own ? "font-medium text-text-primary" : "text-text-secondary")}>
        {dollars(Math.round(total))}
      </span>
    </li>
  );
}
