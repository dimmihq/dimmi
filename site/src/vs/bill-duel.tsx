/*
 * Live bill comparison driven by `rivals.ts` and `prices.ts`.
 * Prerenders the default monthly scenario in the static HTML and lets visitors
 * adjust seats, volume, AI share, and list vs promotional seat pricing.
 */
import { RiExternalLinkLine } from "@remixicon/react";
import { useState } from "react";

import { cx } from "@/utils/cx";

import { Section, SplitHead } from "../components/container";
import { competitorBill, fullBill, type Rival, type RivalPlan } from "../data/rivals";
import { bill, dollars } from "../pricing/prices";
import { formatIsoDate } from "./rival-tone";

function planMonthlyCost(
  rival: Rival,
  plan: RivalPlan,
  seats: number,
  conversations: number,
  aiShare: number,
  usePromo: boolean,
): number {
  const safeSeats = Math.max(1, seats);
  if (rival.competitor && plan === rival.plans[0] && !usePromo) {
    return competitorBill(rival.competitor, { conversations, aiShare, operator: 0, ownModel: false, seats: safeSeats });
  }
  const seatUnit = usePromo && plan.promoSeatPrice != null ? plan.promoSeatPrice : (plan.seatPrice ?? 0);
  const seatCost =
    plan.seatPrice === null
      ? (plan.flatMonthly ?? 0) +
        Math.max(0, safeSeats - (plan.includedSeats ?? safeSeats)) * (plan.extraSeatPrice ?? 0)
      : safeSeats * seatUnit;
  const aiCount = Math.round(conversations * aiShare);
  const billableAi = rival.aiUnit === "resolution" || rival.aiUnit === "outcome" ? aiCount * 0.6 : aiCount;
  const freeAi = (plan.includedAi ?? 0) * (plan.seatPrice === null ? 1 : safeSeats);
  const aiCost = Math.max(0, billableAi - freeAi) * rival.aiPrice;
  return seatCost + aiCost;
}

export function BillDuel({
  rival,
  tag = "Monthly bill",
  title,
  body = "",
  defaultSeats = 5,
  defaultConversations = 2000,
  defaultAiShare = 0.7,
  note,
}: {
  rival: Rival;
  tag?: string;
  title: string;
  body?: string;
  defaultSeats?: number;
  defaultConversations?: number;
  defaultAiShare?: number;
  note?: string;
}) {
  const [seats, setSeats] = useState(defaultSeats);
  const [conversations, setConversations] = useState(defaultConversations);
  const [aiShare, setAiShare] = useState(defaultAiShare);
  const [usePromo, setUsePromo] = useState(false);

  const entryPlan = rival.plans[0];
  const hasPromo = entryPlan?.promoSeatPrice != null;
  const usage = { conversations, aiShare, operator: 0, ownModel: false };

  const dimmiCloud = bill(usage).total;
  const dimmiByok = bill({ ...usage, ownModel: true }).total;
  const rivalEntry = entryPlan ? planMonthlyCost(rival, entryPlan, seats, conversations, aiShare, usePromo) : 0;
  const rivalFull = rival.competitor ? fullBill(rival.competitor, { ...usage, seats }) : null;

  const maxBill = Math.max(1, rivalFull ?? 0, rivalEntry, dimmiCloud, dimmiByok);
  const savings = Math.round(rivalEntry - dimmiCloud);

  return (
    <Section frameClassName="flex flex-col gap-10">
      <SplitHead tag={tag} title={title} body={body} />

      <div className="flex flex-col gap-8 rounded-2xl border border-border-button-default bg-background-full p-5 sm:p-8">
        <div className="grid grid-cols-1 gap-6 md:grid-cols-3">
          <SliderField
            id={`vs-seats-${rival.slug}`}
            label="Seats"
            valueLabel={String(seats)}
            min={1}
            max={25}
            step={1}
            value={seats}
            onChange={setSeats}
          />
          <SliderField
            id={`vs-conv-${rival.slug}`}
            label="Monthly threads"
            valueLabel={conversations.toLocaleString("en-US")}
            min={200}
            max={10000}
            step={200}
            value={conversations}
            onChange={setConversations}
          />
          <SliderField
            id={`vs-ai-${rival.slug}`}
            label="AI share"
            valueLabel={`${Math.round(aiShare * 100)}%`}
            min={0}
            max={100}
            step={5}
            value={Math.round(aiShare * 100)}
            onChange={(v) => setAiShare(v / 100)}
          />
        </div>

        {hasPromo && (
          <div className="flex flex-wrap items-center justify-between gap-3 border-t border-separator-border pt-4 text-[13px] text-text-secondary">
            <span>{entryPlan.promoNote}</span>
            <div className="inline-flex rounded-lg bg-background-secondary-default p-1">
              <button
                type="button"
                aria-pressed={!usePromo}
                onClick={() => setUsePromo(false)}
                className={cx(
                  "cursor-pointer rounded-md px-3 py-1 font-medium transition-colors",
                  !usePromo ? "bg-background-full text-text-primary shadow-xs" : "text-text-secondary",
                )}
              >
                {`List ($${entryPlan.seatPrice}/seat)`}
              </button>
              <button
                type="button"
                aria-pressed={usePromo}
                onClick={() => setUsePromo(true)}
                className={cx(
                  "cursor-pointer rounded-md px-3 py-1 font-medium transition-colors",
                  usePromo ? "bg-background-full text-text-primary shadow-xs" : "text-text-secondary",
                )}
              >
                {`Promo ($${entryPlan.promoSeatPrice}/seat)`}
              </button>
            </div>
          </div>
        )}

        <div className="flex flex-col gap-3 border-t border-separator-border pt-6">
          <BillBar label="Self-host" total={0} max={maxBill} accent note="MIT" />
          <BillBar label="BYOK" total={dimmiByok} max={maxBill} accent note="+API" />
          <BillBar label="Cloud" total={dimmiCloud} max={maxBill} accent note="AI" />
          <BillBar
            label={`${rival.name} ${entryPlan?.name ?? "entry"}`}
            total={rivalEntry}
            max={maxBill}
            rival
            note={usePromo && entryPlan?.promoSeatPrice ? "Promo" : entryPlan?.billingNote}
          />
          {rivalFull != null && rival.competitor && (
            <BillBar
              label={`${rival.name} ${rival.competitor.full.plan}`}
              total={rivalFull}
              max={maxBill}
              rival
              note={`+${rival.competitor.full.adds}`}
            />
          )}
        </div>

        <div className="flex flex-wrap items-center justify-between gap-3 border-t border-separator-border pt-4 text-[13px] leading-5 text-text-tertiary">
          <span>
            {savings > 0
              ? `Delta: -${dollars(savings)}/mo vs ${rival.name} ${entryPlan?.name ?? "entry"}.`
              : `Delta: +${dollars(Math.abs(savings))}/mo vs ${rival.name} ${entryPlan?.name ?? "entry"}.`}
            {note ? ` ${note}` : ""}
          </span>
          <a
            href={rival.pricingUrl}
            rel="noopener noreferrer"
            target="_blank"
            className="inline-flex items-center gap-1 text-text-secondary underline-offset-4 hover:text-text-primary hover:underline"
          >
            {rival.name} {formatIsoDate(rival.checkedOn)}
            <RiExternalLinkLine className="size-3.5 shrink-0" aria-hidden />
          </a>
        </div>
      </div>
    </Section>
  );
}

function SliderField({
  id,
  label,
  valueLabel,
  min,
  max,
  step,
  value,
  onChange,
}: {
  id: string;
  label: string;
  valueLabel: string;
  min: number;
  max: number;
  step: number;
  value: number;
  onChange: (next: number) => void;
}) {
  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-baseline justify-between gap-2">
        <label htmlFor={id} className="text-[13px] font-medium text-text-secondary">
          {label}
        </label>
        <span className="font-mono text-[14px] font-medium text-text-primary tabular-nums">{valueLabel}</span>
      </div>
      <input
        id={id}
        type="range"
        min={min}
        max={max}
        step={step}
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
        className="w-full cursor-pointer accent-accent-600"
      />
    </div>
  );
}

function BillBar({
  label,
  total,
  max,
  accent = false,
  rival = false,
  note,
}: {
  label: string;
  total: number;
  max: number;
  accent?: boolean;
  rival?: boolean;
  note?: string;
}) {
  const widthPct = total <= 0 ? 1.5 : Math.max(2, Math.min(100, (total / max) * 100));
  return (
    <div className="grid grid-cols-1 items-center gap-2 sm:grid-cols-[230px_1fr_84px] sm:gap-4">
      <div className="flex min-w-0 flex-col">
        <span
          className={cx(
            "truncate text-[14px] leading-5",
            accent ? "font-medium text-text-primary" : "text-text-secondary",
          )}
        >
          {label}
        </span>
        {note ? <span className="truncate text-[12px] leading-4 text-text-tertiary">{note}</span> : null}
      </div>
      <span className="h-3 overflow-hidden rounded-full bg-background-secondary-default">
        <span
          className={cx(
            "block h-full rounded-full transition-[width] duration-300 ease-out",
            accent && "bg-accent-500",
            !accent && !rival && "bg-foreground-icon-tertiary/50",
          )}
          style={{
            width: `${widthPct}%`,
            ...(rival ? { backgroundColor: "var(--rival, #71717a)", opacity: 0.75 } : {}),
          }}
        />
      </span>
      <span
        className={cx(
          "text-end font-mono text-[14px] tabular-nums",
          accent ? "font-medium text-text-primary" : "text-text-secondary",
        )}
      >
        {dollars(Math.round(total))}/mo
      </span>
    </div>
  );
}
