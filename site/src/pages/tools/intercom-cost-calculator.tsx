/*
 * Standalone Intercom pricing calculator (`/tools/intercom-cost-calculator`).
 * Prerenders the default monthly and yearly calculation in static HTML, syncs
 * inputs to shareable URL query params while keeping canonical query-free, and
 * compares Intercom seats + Fin + add-ons against dimmi Cloud, BYO keys, and
 * self-hosted MIT servers.
 */
import { RiArrowRightLine, RiCheckLine, RiExternalLinkLine, RiFileCopyLine, RiServerLine } from "@remixicon/react";
import { useEffect, useId, useState } from "react";

import { ButtonLink } from "@/components/base/buttons/button";
import { cx } from "@/utils/cx";

import { MonoLink, Section, SplitHead, Tag } from "../../components/container";
import { Questions } from "../../components/product-page";
import { ProductShot } from "../../components/product-shot";
import { CHECKED_ON, CHECKED_ON_ISO, getRival } from "../../data/rivals";
import { APP_URL, GITHUB_URL } from "../../links";
import { dollars, FREE_CONVERSATIONS } from "../../pricing/prices";
import { SERVERS } from "../../pricing/servers";
import { FinalCta } from "../../sections/final-cta";
import { formatIsoDate } from "../../vs/rival-tone";
import {
  buildCalcQuery,
  COPILOT_ANNUAL_PER_SEAT,
  COPILOT_MONTHLY_PER_SEAT,
  calculateIntercomBill,
  DEFAULT_CALC_INPUT,
  FIN_OUTCOME_PRICE,
  INTERCOM_PLAN_RATES,
  type IntercomCalcInput,
  type IntercomPlanId,
  PRO_ADDON_MONTHLY,
  PROACTIVE_PLUS_MONTHLY,
  parseCalcQuery,
  trackCalcChange,
} from "./intercom-calc-math";

const intercom = getRival("intercom");

const EXCLUDED_COSTS = [
  "WhatsApp Business messages (billed per message sent from December 1, 2026)",
  "SMS segments, phone minutes, and Fin Voice outcomes ($1.99 per voice outcome)",
  "Bulk outbound email campaigns beyond plan allowances",
  "Sales tax, VAT, custom enterprise contracts, and Intercom Early Stage startup discounts",
];

const CALC_QUESTIONS = [
  {
    question: "How does Intercom calculate Fin AI Agent charges?",
    answer:
      "Intercom charges $0.99 per Fin outcome on Essential, Advanced, and Expert. A conversation counts as a billable outcome when Fin answers and the customer confirms or leaves without asking for a teammate, or when a Fin procedure finishes.",
  },
  {
    question: "Why does the calculator have both $29 and $19 for Essential?",
    answer:
      "On November 27, 2026, intercom.com/pricing listed Essential at $19/seat/mo for new annual customers (35% off) while Intercom's own FAQ lists $29/seat/mo. Switching the toggle lets you model both your year-one promo bill and your renewal rate.",
  },
  {
    question: "When is Intercom cheaper than dimmi Cloud?",
    answer:
      "When a small team answers almost every chat by hand with 0% AI. For example, 1 seat on the $19 promo answering 2,000 manual chats costs $19/month on Intercom Essential compared with $57/month on dimmi Cloud. Self-hosting dimmi under MIT stays $0 to dimmi.",
  },
  {
    question: "Does dimmi charge extra for seats, workflows, or SLAs?",
    answer:
      "No. Unlimited seats, workflows, round robin, multiple inboxes, SLAs, SAML SSO, and Operator are included on every dimmi workspace. You pay per conversation after 100 free conversations a month, or $0 to dimmi if you self-host.",
  },
];

function SliderRow({
  label,
  valueText,
  min,
  max,
  step,
  value,
  onChange,
}: {
  label: string;
  valueText: string;
  min: number;
  max: number;
  step: number;
  value: number;
  onChange: (next: number) => void;
}) {
  const id = useId();
  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-baseline justify-between gap-2">
        <label htmlFor={id} className="text-[14px] font-medium text-text-primary">
          {label}
        </label>
        <span className="font-mono text-[14px] font-medium text-text-primary tabular-nums">{valueText}</span>
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

function AddOnToggle({
  label,
  priceNote,
  checked,
  onToggle,
}: {
  label: string;
  priceNote: string;
  checked: boolean;
  onToggle: () => void;
}) {
  const id = useId();
  return (
    <label
      htmlFor={id}
      className={cx(
        "flex cursor-pointer items-start gap-3 rounded-xl border p-3.5 transition-colors",
        checked
          ? "border-accent-500/45 bg-accent-500/[0.05]"
          : "border-border-button-default bg-background-full hover:border-text-tertiary/50",
      )}
    >
      <input
        id={id}
        type="checkbox"
        checked={checked}
        onChange={onToggle}
        className="mt-0.5 size-4 cursor-pointer accent-accent-600"
      />
      <span className="flex flex-col gap-0.5 text-[13px] leading-5">
        <span className="font-medium text-text-primary">{label}</span>
        <span className="text-text-tertiary">{priceNote}</span>
      </span>
    </label>
  );
}

function CalcControls({
  input,
  onUpdate,
}: {
  input: IntercomCalcInput;
  onUpdate: <K extends keyof IntercomCalcInput>(key: K, value: IntercomCalcInput[K]) => void;
}) {
  const plans: IntercomPlanId[] = ["essential", "advanced", "expert"];
  const copilotRate = input.billing === "annual" ? COPILOT_ANNUAL_PER_SEAT : COPILOT_MONTHLY_PER_SEAT;

  return (
    <div className="flex flex-col gap-6 rounded-2xl border border-border-button-default bg-background-full p-6 sm:p-7">
      <div className="flex flex-col gap-5">
        <SliderRow
          label="Support seats"
          valueText={`${input.seats} ${input.seats === 1 ? "seat" : "seats"}`}
          min={1}
          max={100}
          step={1}
          value={input.seats}
          onChange={(v) => onUpdate("seats", v)}
        />
        <SliderRow
          label="Monthly conversations"
          valueText={input.conversations.toLocaleString("en-US")}
          min={100}
          max={20000}
          step={100}
          value={input.conversations}
          onChange={(v) => onUpdate("conversations", v)}
        />
        <SliderRow
          label="Conversations answered by AI"
          valueText={`${Math.round(input.aiShare * 100)}%`}
          min={0}
          max={100}
          step={5}
          value={Math.round(input.aiShare * 100)}
          onChange={(v) => onUpdate("aiShare", v / 100)}
        />
        <SliderRow
          label="Fin resolution rate"
          valueText={`${Math.round(input.resolutionRate * 100)}%`}
          min={20}
          max={90}
          step={5}
          value={Math.round(input.resolutionRate * 100)}
          onChange={(v) => onUpdate("resolutionRate", v / 100)}
        />
      </div>

      <div className="flex flex-col gap-3 border-t border-separator-border pt-5">
        <span className="font-mono text-[11px] tracking-[0.06em] text-text-tertiary uppercase">Intercom plan</span>
        <div className="grid grid-cols-3 gap-2">
          {plans.map((planId) => {
            const spec = INTERCOM_PLAN_RATES[planId];
            const seatRate =
              input.billing === "annual"
                ? input.promo && spec.annualPromo != null
                  ? spec.annualPromo
                  : spec.annual
                : spec.monthly;
            return (
              <button
                key={planId}
                type="button"
                aria-pressed={input.plan === planId}
                onClick={() => onUpdate("plan", planId)}
                className={cx(
                  "flex cursor-pointer flex-col gap-0.5 rounded-xl border p-3 text-start transition-colors",
                  input.plan === planId
                    ? "border-accent-500/50 bg-accent-500/[0.06]"
                    : "border-border-button-default bg-background-full hover:border-text-tertiary/50",
                )}
              >
                <span className="text-[14px] font-medium text-text-primary">{spec.name}</span>
                <span className="font-mono text-[12px] text-text-secondary tabular-nums">{`$${seatRate}/seat`}</span>
              </button>
            );
          })}
        </div>
        <p className="text-[12px] leading-5 text-text-tertiary">{INTERCOM_PLAN_RATES[input.plan].summary}</p>
      </div>

      <div className="flex flex-wrap items-center justify-between gap-3 border-t border-separator-border pt-5 text-[13px]">
        <div className="inline-flex rounded-lg bg-background-secondary-default p-1">
          <button
            type="button"
            aria-pressed={input.billing === "annual"}
            onClick={() => onUpdate("billing", "annual")}
            className={cx(
              "cursor-pointer rounded-md px-3 py-1 font-medium transition-colors",
              input.billing === "annual" ? "bg-background-full text-text-primary shadow-xs" : "text-text-secondary",
            )}
          >
            Annual billing
          </button>
          <button
            type="button"
            aria-pressed={input.billing === "monthly"}
            onClick={() => onUpdate("billing", "monthly")}
            className={cx(
              "cursor-pointer rounded-md px-3 py-1 font-medium transition-colors",
              input.billing === "monthly" ? "bg-background-full text-text-primary shadow-xs" : "text-text-secondary",
            )}
          >
            Monthly billing
          </button>
        </div>

        {input.plan === "essential" && input.billing === "annual" ? (
          <div className="inline-flex rounded-lg bg-background-secondary-default p-1">
            <button
              type="button"
              aria-pressed={!input.promo}
              onClick={() => onUpdate("promo", false)}
              className={cx(
                "cursor-pointer rounded-md px-3 py-1 font-medium transition-colors",
                !input.promo ? "bg-background-full text-text-primary shadow-xs" : "text-text-secondary",
              )}
            >
              $29 list
            </button>
            <button
              type="button"
              aria-pressed={input.promo}
              onClick={() => onUpdate("promo", true)}
              className={cx(
                "cursor-pointer rounded-md px-3 py-1 font-medium transition-colors",
                input.promo ? "bg-background-full text-text-primary shadow-xs" : "text-text-secondary",
              )}
            >
              $19 promo
            </button>
          </div>
        ) : null}
      </div>

      <div className="flex flex-col gap-2.5 border-t border-separator-border pt-5">
        <span className="font-mono text-[11px] tracking-[0.06em] text-text-tertiary uppercase">Optional add-ons</span>
        <AddOnToggle
          label="Copilot"
          priceNote={`$${copilotRate}/seat/mo (${dollars(copilotRate * input.seats)}/mo for ${input.seats} seats)`}
          checked={input.copilot}
          onToggle={() => onUpdate("copilot", !input.copilot)}
        />
        <AddOnToggle
          label="Pro (includes Operator)"
          priceNote={`From $${PRO_ADDON_MONTHLY}/workspace/mo`}
          checked={input.proAddOn}
          onToggle={() => onUpdate("proAddOn", !input.proAddOn)}
        />
        <AddOnToggle
          label="Proactive Support Plus"
          priceNote={`From $${PROACTIVE_PLUS_MONTHLY}/workspace/mo (Tours, Surveys, Series)`}
          checked={input.proactivePlus}
          onToggle={() => onUpdate("proactivePlus", !input.proactivePlus)}
        />
      </div>
    </div>
  );
}

function CalcResults({
  input,
  copied,
  onCopyLink,
}: {
  input: IntercomCalcInput;
  copied: boolean;
  onCopyLink: () => void;
}) {
  const r = calculateIntercomBill(input);
  const planName = INTERCOM_PLAN_RATES[input.plan].name;

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col gap-5 rounded-2xl border border-border-button-default bg-background-full p-6 sm:p-7">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <span className="font-mono text-[11px] tracking-[0.06em] text-text-tertiary uppercase">
            Estimated Intercom {planName} bill
          </span>
          <button
            type="button"
            onClick={onCopyLink}
            className="inline-flex cursor-pointer items-center gap-1.5 rounded-lg border border-border-button-default px-2.5 py-1 font-mono text-[11px] text-text-secondary transition-colors hover:text-text-primary"
          >
            {copied ? (
              <RiCheckLine className="size-3.5 text-accent-600" aria-hidden />
            ) : (
              <RiFileCopyLine className="size-3.5" aria-hidden />
            )}
            {copied ? "Link copied" : "Copy share link"}
          </button>
        </div>

        <div className="flex flex-wrap items-baseline gap-x-4 gap-y-1">
          <span className="font-mono text-[42px] leading-none font-medium tracking-[-0.03em] text-text-primary tabular-nums">
            {dollars(Math.round(r.intercomMonthly))}
            <span className="text-[16px] font-normal text-text-secondary">/mo</span>
          </span>
          <span className="font-mono text-[15px] text-text-tertiary tabular-nums">
            {dollars(Math.round(r.intercomYearly))}/year
          </span>
        </div>

        <dl className="flex flex-col divide-y divide-separator-border border-y border-separator-border text-[14px]">
          <div className="flex items-center justify-between py-2.5">
            <dt className="text-text-secondary">{`Seats (${input.seats} x $${r.seatUnit}/mo)`}</dt>
            <dd className="font-mono font-medium text-text-primary tabular-nums">
              {dollars(Math.round(r.seatCost))}/mo
            </dd>
          </div>
          <div className="flex items-center justify-between py-2.5">
            <dt className="text-text-secondary">
              {`Fin AI (${Math.round(r.finOutcomes).toLocaleString("en-US")} outcomes x $${FIN_OUTCOME_PRICE})`}
            </dt>
            <dd className="font-mono font-medium text-text-primary tabular-nums">
              {dollars(Math.round(r.finCost))}/mo
            </dd>
          </div>
          <div className="flex items-center justify-between py-2.5">
            <dt className="text-text-secondary">Add-ons (Copilot, Pro, Proactive Plus)</dt>
            <dd className="font-mono font-medium text-text-primary tabular-nums">
              {dollars(Math.round(r.addOnsCost))}/mo
            </dd>
          </div>
        </dl>

        <p className="text-[13px] leading-6 text-text-tertiary">
          {`Formula: (${input.seats} seats x $${r.seatUnit}) + (${r.aiAnswered.toLocaleString("en-US")} AI chats x ${Math.round(input.resolutionRate * 100)}% resolved x $${FIN_OUTCOME_PRICE}) + ${dollars(Math.round(r.addOnsCost))} add-ons = ${dollars(Math.round(r.intercomMonthly))}/month (${dollars(Math.round(r.intercomYearly))}/year).`}
        </p>
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <div className="flex flex-col gap-2 rounded-2xl border border-accent-500/35 bg-accent-500/[0.04] p-5">
          <span className="font-mono text-[11px] text-accent-700 uppercase dark:text-accent-300">
            dimmi Cloud · AI included
          </span>
          <span className="font-mono text-[30px] leading-9 font-medium text-text-primary tabular-nums">
            {dollars(Math.round(r.dimmiCloudMonthly))}
            <span className="text-[14px] font-normal text-text-secondary">/mo</span>
          </span>
          <span className="font-mono text-[12px] text-text-tertiary tabular-nums">
            {dollars(Math.round(r.dimmiCloudYearly))}/year · unlimited seats
          </span>
        </div>

        <div className="flex flex-col gap-2 rounded-2xl border border-accent-500/35 bg-accent-500/[0.04] p-5">
          <span className="font-mono text-[11px] text-accent-700 uppercase dark:text-accent-300">
            dimmi Cloud · BYO model key
          </span>
          <span className="font-mono text-[30px] leading-9 font-medium text-text-primary tabular-nums">
            {dollars(Math.round(r.dimmiByokMonthly))}
            <span className="text-[14px] font-normal text-text-secondary">/mo</span>
          </span>
          <span className="font-mono text-[12px] text-text-tertiary tabular-nums">
            {dollars(Math.round(r.dimmiByokYearly))}/year + raw API tokens
          </span>
        </div>
      </div>

      <div
        className={cx(
          "rounded-2xl border p-5 text-[14px] leading-6",
          r.dimmiMoreExpensive
            ? "border-border-button-default bg-background-secondary-default/50 text-text-primary"
            : "border-accent-500/30 bg-accent-500/[0.03] text-text-primary",
        )}
      >
        {r.dimmiMoreExpensive ? (
          <p>
            <strong>Honest note:</strong> for these inputs, dimmi Cloud costs{" "}
            <strong>{dollars(Math.round(Math.abs(r.monthlyDiff)))}/month more</strong> than Intercom {planName} because
            your AI share is low and your seat count is tiny. If you want $0 software fees at this volume, self-host
            dimmi under MIT and pay only for your server below.
          </p>
        ) : (
          <p>
            dimmi Cloud saves about <strong>{dollars(Math.round(r.monthlyDiff))}/month</strong> (
            <strong>{dollars(Math.round(r.monthlyDiff * 12))}/year</strong>) compared with Intercom {planName}, with
            workflows, round robin, SLAs, and SAML SSO included for every teammate.
          </p>
        )}
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <ButtonLink href={`${APP_URL}/signup`} trailingIcon={RiArrowRightLine}>
          Start free on dimmi ({FREE_CONVERSATIONS} chats/mo)
        </ButtonLink>
        <ButtonLink href="/vs/intercom" variant="secondary">
          Read dimmi vs Intercom
        </ButtonLink>
      </div>
    </div>
  );
}

export function IntercomCostCalculatorPage() {
  const [input, setInput] = useState<IntercomCalcInput>(DEFAULT_CALC_INPUT);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    if (window.location.search) {
      setInput(parseCalcQuery(window.location.search));
    }
  }, []);

  const updateInput = <K extends keyof IntercomCalcInput>(key: K, value: IntercomCalcInput[K]) => {
    setInput((prev) => {
      const next = { ...prev, [key]: value };
      if (typeof window !== "undefined") {
        const qs = buildCalcQuery(next);
        window.history.replaceState(null, "", `${window.location.pathname}${qs}`);
      }
      trackCalcChange(String(key));
      return next;
    });
  };

  const handleCopyLink = () => {
    if (typeof window === "undefined") {
      return;
    }
    const url = `${window.location.origin}${window.location.pathname}${buildCalcQuery(input)}`;
    navigator.clipboard?.writeText(url).catch(() => undefined);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const defaultResult = calculateIntercomBill(DEFAULT_CALC_INPUT);

  return (
    <>
      <Section frameClassName="flex flex-col gap-8">
        <nav aria-label="Breadcrumb" className="flex flex-wrap items-center gap-2 text-[13px] text-text-tertiary">
          <a href="/" className="transition-colors hover:text-text-primary">
            Home
          </a>
          <span aria-hidden>/</span>
          <a href="/pricing" className="transition-colors hover:text-text-primary">
            Tools
          </a>
          <span aria-hidden>/</span>
          <span className="text-text-primary">Intercom pricing calculator</span>
        </nav>

        <div className="flex max-w-3xl flex-col gap-5">
          <Tag>Intercom pricing calculator · 2026</Tag>
          <h1 className="text-[44px] leading-[1.08] font-medium tracking-[-0.03em] text-balance text-text-primary max-sm:text-[32px]">
            Calculate your seat, AI outcome, and add-on support bill.
          </h1>
          <p className="text-[17px] leading-7 text-text-secondary">
            A 5-seat team on Intercom Essential ($29 list) handling 2,000 monthly chats (80% AI, 60% resolved) pays{" "}
            {dollars(Math.round(defaultResult.intercomMonthly))}/month (
            {dollars(Math.round(defaultResult.intercomYearly))}
            /year): {dollars(Math.round(defaultResult.seatCost))} for seats and{" "}
            {dollars(Math.round(defaultResult.finCost))} for Fin outcomes. The same month on dimmi Cloud costs{" "}
            {dollars(Math.round(defaultResult.dimmiCloudMonthly))}/month, or{" "}
            {dollars(Math.round(defaultResult.dimmiByokMonthly))}/month with your own AI key.
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-x-4 gap-y-1.5 border-t border-separator-border pt-4 font-mono text-[12px] text-text-tertiary">
          <span>
            Updated <time dateTime={CHECKED_ON_ISO}>{formatIsoDate(CHECKED_ON_ISO)}</time>
          </span>
          <a
            href={intercom.pricingUrl}
            rel="noopener noreferrer"
            target="_blank"
            className="inline-flex items-center gap-1 text-text-secondary underline-offset-4 hover:text-text-primary hover:underline"
          >
            Intercom prices from intercom.com/pricing, checked {CHECKED_ON}
            <RiExternalLinkLine className="size-3.5 shrink-0" aria-hidden />
          </a>
        </div>
      </Section>

      <Section frameClassName="flex flex-col gap-10">
        <div className="grid grid-cols-1 items-start gap-6 lg:grid-cols-[minmax(0,0.95fr)_minmax(0,1.05fr)]">
          <CalcControls input={input} onUpdate={updateInput} />
          <CalcResults input={input} copied={copied} onCopyLink={handleCopyLink} />
        </div>
      </Section>

      <Section frameClassName="flex flex-col gap-10">
        <SplitHead
          tag="Self-hosted option"
          title="Or self-host dimmi under MIT and pay us $0."
          body="Run the identical codebase on one Linux server. You pay your VPS provider and your own model API directly."
        />
        <div className="grid grid-cols-1 gap-5 md:grid-cols-3 [&>*]:min-w-0">
          {SERVERS.map((server) => (
            <div
              key={server.name}
              className="flex flex-col gap-4 rounded-xl border border-border-button-default bg-background-full p-6"
            >
              <div className="flex items-center justify-between gap-2">
                <span className="font-mono text-[12px] text-text-tertiary uppercase">{server.name}</span>
                <RiServerLine className="size-4 text-foreground-icon-tertiary" aria-hidden />
              </div>
              <div className="flex items-baseline gap-1.5">
                <span className="font-mono text-[28px] font-medium text-text-primary tabular-nums">{server.price}</span>
                <span className="text-[14px] text-text-secondary">/ month + $0 to dimmi</span>
              </div>
              <p className="text-[13px] leading-5 text-text-secondary">
                {server.where}. {server.fits}
              </p>
            </div>
          ))}
        </div>
        <div className="flex flex-wrap gap-6">
          <MonoLink href="/pricing">See the full dimmi price book</MonoLink>
          <MonoLink href="/vs/intercom">Read dimmi vs Intercom</MonoLink>
          <MonoLink href="/docs/self-hosting">Read the self-hosting docs</MonoLink>
        </div>
      </Section>

      <Section frameClassName="flex flex-col gap-10">
        <SplitHead
          tag="Real product proof"
          title="Live workspace reporting and BYO model keys, included on every plan."
          body="Both screenshots come from a real dimmi workspace. Track volume, response speed, and plug in your own OpenAI or Anthropic key with zero enterprise upgrade."
        />
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
          <ProductShot
            name="vs-reports"
            alt="dimmi workspace overview reports showing conversation volume, first response time, resolution speed, and first contact resolution"
          />
          <ProductShot
            name="vs-byo-key"
            alt="dimmi AI providers settings page showing connected dimmi models and Add provider button for your own API keys"
          />
        </div>
      </Section>

      <Section frameClassName="flex flex-col gap-8">
        <SplitHead
          tag="What this leaves out"
          title="Usage fees and contract terms not included in the estimate."
          body="Keep these separate line items in mind when auditing an Intercom invoice."
        />
        <ul className="grid grid-cols-1 gap-3 md:grid-cols-2">
          {EXCLUDED_COSTS.map((item) => (
            <li
              key={item}
              className="rounded-xl border border-border-button-default bg-background-full px-5 py-4 text-[14px] leading-6 text-text-secondary"
            >
              {item}
            </li>
          ))}
        </ul>
      </Section>

      <Questions items={CALC_QUESTIONS} />

      <FinalCta
        title="Skip the seat multiplier and the $0.99 outcome meter."
        links={[
          { label: "Start free, 100 chats/mo", href: `${APP_URL}/signup`, variant: "primary", icon: "arrow" },
          { label: "Self-host on GitHub", href: GITHUB_URL, variant: "secondary", icon: "github" },
        ]}
      />
    </>
  );
}
