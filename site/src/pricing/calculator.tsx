/*
 * The estimate: the month's customer conversations, how many the AI agent
 * answers, the Operator threads teammates start, and whether the workspace
 * brings its own model. The bill updates as the inputs move, and the same
 * month is priced at each competitor's public rates beside it.
 */
import { RiArrowRightLine, RiBankCardLine, RiInformation2Line } from "@remixicon/react";
import { type ReactNode, useState } from "react";

import { ButtonLink } from "@/components/base/buttons/button";
import { cx } from "@/utils/cx";

import { APP_URL } from "../links";

import { COMPETITORS, competitorBill } from "./competitors";
import { ModelChoice } from "./model-choice";
import { PlanTip } from "./plan-tip";
import { bill, dollars, FREE_CONVERSATIONS } from "./prices";
import { Range } from "./range";

export function Calculator() {
  const [conversations, setConversations] = useState(100);
  const [aiShare, setAiShare] = useState(70);
  const [operator, setOperator] = useState(0);
  const [seats, setSeats] = useState(3);
  const [ownModel, setOwnModel] = useState(false);

  const usage = { conversations, aiShare: aiShare / 100, operator, ownModel };
  const result = bill(usage);
  const included = bill({ ...usage, ownModel: false });
  const saving = included.total - bill({ ...usage, ownModel: true }).total;
  const rows = COMPETITORS.map((c) => ({
    competitor: c,
    name: c.name,
    total: competitorBill(c, { ...usage, seats }),
  })).sort((a, b) => a.total - b.total);
  const ours = result.custom ? null : result.total;
  const highest = Math.max(result.total, ...rows.map((r) => r.total), 1);
  // The saving is always measured against Intercom, the name most teams compare with.
  const intercom = rows.find((r) => r.name === "Intercom");
  const saved = intercom ? intercom.total - result.total : 0;

  return (
    <div className="grid grid-cols-1 overflow-hidden rounded-xl border border-border-button-default lg:grid-cols-[1fr_minmax(0,440px)] [&>*]:min-w-0">
      <div className="flex flex-col gap-9 bg-background-full p-6 sm:p-10">
        <Range
          label="Customer support conversations"
          hint="Per month, on every channel."
          value={conversations}
          onChange={setConversations}
          max={10000}
          curve
          freeUpTo={FREE_CONVERSATIONS}
        />
        <Range
          label="Answered by the AI agent"
          hint="The share the agent replies in."
          value={aiShare}
          onChange={setAiShare}
          max={100}
          format={{ style: "unit", unit: "percent" }}
        />
        <Range
          label="Operator conversations"
          hint="Threads your team starts with Operator."
          value={operator}
          onChange={setOperator}
          max={5000}
          curve
        />
        <Range
          label="Teammates"
          hint="Free on dimmi. Only prices the competitors."
          value={seats}
          onChange={setSeats}
          max={50}
        />
        <div className="flex flex-col gap-3">
          <span className="text-[15px] leading-6 font-medium text-text-primary">AI model</span>
          <ModelChoice own={ownModel} onChange={setOwnModel} />
        </div>
      </div>

      <div className="flex flex-col gap-8 border-t border-border-button-default bg-background-secondary-default p-6 sm:p-10 lg:border-t-0 lg:border-s">
        <div className="flex flex-col gap-1">
          <span className="font-mono text-[12px] leading-5 font-medium tracking-[0.04em] text-text-tertiary uppercase">
            Your month on dimmi
          </span>
          {result.custom ? (
            <span className="text-[44px] leading-[1.1] font-medium tracking-[-0.03em] text-text-primary">
              Talk to us
            </span>
          ) : (
            <span className="text-[56px] leading-[1.05] font-medium tracking-[-0.035em] text-text-primary tabular-nums">
              {dollars(result.total)}
            </span>
          )}
          <span className="text-[14px] leading-5 text-text-secondary">
            {result.custom
              ? "Above 10,000 conversations a month we agree a price with you."
              : result.total === 0
                ? `Free: ${FREE_CONVERSATIONS} conversations or fewer this month.`
                : `${seats} ${seats === 1 ? "seat" : "seats"} included free. No subscription.`}
          </span>
          <Savings saved={result.custom ? 0 : saved} against="Intercom" />
        </div>

        <dl className="flex flex-col divide-y divide-separator-border border-y border-separator-border text-[15px] leading-6">
          <Line
            label="AI conversations"
            detail={`${result.ai.toLocaleString("en-US")} (${Math.round(conversations * usage.aiShare).toLocaleString("en-US")} customer + ${operator.toLocaleString("en-US")} Operator)`}
            value={dollars(result.aiCost)}
          />
          <Line
            label="Human conversations"
            detail={result.human.toLocaleString("en-US")}
            value={dollars(result.humanCost)}
          />
          <Line
            label="First 100 conversations"
            detail="customer and Operator, every month"
            value={result.waived > 0 ? `−${dollars(result.waived)}` : "free"}
            accent
          />
          <Line label="Seats" detail={`${seats}, or any number`} value="free" />
          <Line
            label="Saved with your own keys"
            detail={ownModel ? "you pay your AI provider directly" : "if you bring your own keys or ChatGPT plan"}
            value={`−${dollars(saving)}`}
            accent={ownModel}
          />
        </dl>

        <div className="flex flex-col items-center gap-2.5">
          <ButtonLink href={`${APP_URL}/signup`} trailingIcon={RiArrowRightLine} className="w-full">
            Start free
          </ButtonLink>
          <span className="inline-flex items-center gap-1.5 text-[13px] leading-5 text-text-secondary">
            <RiBankCardLine className="size-3.5 text-foreground-icon-tertiary" aria-hidden />
            No credit card required
          </span>
        </div>

        <div className="flex flex-col gap-3">
          <span className="font-mono text-[12px] leading-5 font-medium tracking-[0.04em] text-text-tertiary uppercase">
            The same month elsewhere
          </span>
          <ul className="flex flex-col gap-2.5">
            <Bar name="dimmi" total={ours} highest={highest} own />
            {rows.map((row) => (
              <Bar
                key={row.name}
                name={row.name}
                total={row.total}
                highest={highest}
                tip={
                  <PlanTip competitor={row.competitor} usage={{ ...usage, seats }} cheapest={row.total} ours={ours} />
                }
              />
            ))}
          </ul>
          <p className="flex gap-1.5 text-[13px] leading-5 text-text-tertiary">
            <RiInformation2Line className="mt-0.5 size-3.5 shrink-0" aria-hidden />
            <span>
              Each vendor's cheapest plan with an AI agent, at list price. The i prices the plan with everything.
            </span>
          </p>
        </div>
      </div>
    </div>
  );
}

/*
 * The saving against Intercom, on one line: the
 * month in the accent, the year beside it. It never wraps, so the box
 * keeps its height whatever the numbers.
 */
function Savings({ saved, against }: { saved: number; against: string }) {
  return (
    <div className="mt-4 flex h-12 items-center gap-3 rounded-xl border border-accent-500/30 bg-accent-500/[0.07] px-4 text-[15px] leading-6">
      {saved > 0 ? (
        <>
          <span className="min-w-0 flex-1 truncate text-text-secondary">
            Save{" "}
            <span className="font-medium text-accent-700 tabular-nums dark:text-accent-300">
              {dollars(Math.round(saved))}
            </span>{" "}
            a month vs {against}
          </span>
          <span className="shrink-0 text-[13px] text-text-tertiary tabular-nums">
            {dollars(Math.round(saved * 12))} a year
          </span>
        </>
      ) : (
        <span className="text-text-secondary">Seats stay free at any volume.</span>
      )}
    </div>
  );
}

function Line({
  label,
  detail,
  value,
  accent = false,
}: {
  label: string;
  detail: string;
  value: string;
  accent?: boolean;
}) {
  return (
    <div className="flex items-baseline justify-between gap-4 py-3">
      <dt className="flex min-w-0 flex-col">
        <span className="text-text-primary">{label}</span>
        <span className="truncate text-[13px] leading-5 text-text-tertiary">{detail}</span>
      </dt>
      <dd
        className={cx("shrink-0 tabular-nums", accent ? "text-accent-600 dark:text-accent-400" : "text-text-primary")}
      >
        {value}
      </dd>
    </div>
  );
}

function Bar({
  name,
  total,
  highest,
  own = false,
  tip,
}: {
  name: string;
  total: number | null;
  highest: number;
  own?: boolean;
  tip?: ReactNode;
}) {
  const width = total === null ? 0 : Math.max(1.5, (total / highest) * 100);
  return (
    <li className="grid grid-cols-[104px_1fr_auto] items-center gap-3 text-[14px] leading-5">
      <span className="flex min-w-0 items-center gap-1">
        <span className={cx("truncate", own ? "font-medium text-text-primary" : "text-text-secondary")}>{name}</span>
        {tip}
      </span>
      <span className="h-2 overflow-hidden rounded-full bg-background-tertiary-default">
        <span
          className={cx(
            "block h-full rounded-full transition-[width] duration-300",
            own ? "bg-accent-500" : "bg-foreground-icon-tertiary",
          )}
          style={{ width: `${width}%` }}
        />
      </span>
      <span className={cx("w-16 text-end tabular-nums", own ? "font-medium text-text-primary" : "text-text-secondary")}>
        {total === null ? "custom" : dollars(Math.round(total))}
      </span>
    </li>
  );
}
