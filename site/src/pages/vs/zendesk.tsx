/*
 * dimmi vs Zendesk comparison page (`/vs/zendesk`).
 * Unique elements: stacked seat-tier hero visual, interactive feature-to-plan
 * picker backed by `fullBill`, and real dimmi SLA policies + workflow routing
 * captures.
 */
import { useId, useState } from "react";

import { cx } from "@/utils/cx";

import { MonoLink, Section, SplitHead } from "../../components/container";
import { Questions } from "../../components/product-page";
import { ProductShot } from "../../components/product-shot";
import { CHECKED_ON, CHECKED_ON_ISO, competitorBill, fullBill, getRival } from "../../data/rivals";
import { APP_URL, GITHUB_URL } from "../../links";
import { bill, dollars, FREE_CONVERSATIONS } from "../../pricing/prices";
import {
  BillDuel,
  CtaBand,
  ProofBand,
  rivalToneStyle,
  SwitchTriggers,
  TeamPicker,
  VsHero,
  VsMatrix,
  type VsMatrixGroup,
  WinCards,
} from "../../vs";

const zendesk = getRival("zendesk");

const ZENDESK_MATRIX: VsMatrixGroup[] = [
  {
    title: "Seats, tiers, and AI billing",
    rows: [
      {
        feature: "Teammate seat price",
        dimmi: { status: "yes", label: "$0 unlimited" },
        rival: { status: "no", label: "$69 to $149/seat monthly" },
      },
      {
        feature: "Automated AI resolution fee",
        dimmi: { status: "yes", label: "From $0.10/chat or BYO key" },
        rival: { status: "no", label: "$2.00/res after 5-10/seat" },
      },
      {
        feature: "Teammate AI copilot",
        dimmi: { status: "yes", label: "Operator included" },
        rival: { status: "partial", label: "$50/agent/mo add-on" },
      },
      {
        feature: "Open-source self-hosting",
        dimmi: { status: "yes", label: "100% MIT ($0)" },
        rival: { status: "no", label: "Closed SaaS" },
      },
    ],
  },
  {
    title: "Support ops and routing",
    rows: [
      {
        feature: "Business-hour SLA policies",
        dimmi: { status: "yes", label: "Every workspace" },
        rival: { status: "partial", label: "Suite Pro ($149/seat)" },
      },
      {
        feature: "Visual workflow routing and skills",
        dimmi: { status: "yes", label: "Every workspace" },
        rival: { status: "partial", label: "Suite Pro ($149/seat)" },
      },
      {
        feature: "Multilingual help center",
        dimmi: { status: "yes", label: "Every workspace" },
        rival: { status: "partial", label: "Suite Pro ($149/seat)" },
      },
      {
        feature: "Built-in ticket types and incident trackers",
        dimmi: { status: "yes", label: "Included" },
        rival: { status: "yes", label: "Included" },
      },
    ],
  },
  {
    title: "Enterprise scale and compliance",
    rows: [
      {
        feature: "SOC 2, ISO 27001, and HIPAA compliance",
        requestTitle: "SOC 2 and ISO 27001 certification",
        dimmi: { status: "no" },
        rival: { status: "yes", label: "Certified" },
      },
      {
        feature: "1,800+ app marketplace",
        requestTitle: "Third-party app marketplace",
        dimmi: { status: "no" },
        rival: { status: "yes", label: "1,800+ apps" },
      },
      {
        feature: "Enterprise call-center IVR and WFM",
        requestTitle: "Call-center IVR and shift rosters",
        dimmi: { status: "no" },
        rival: { status: "yes", label: "Suite Pro + WFM" },
      },
      {
        feature: "REST API, 12 typed SDKs, and MCP server",
        dimmi: { status: "yes", label: "Included" },
        rival: { status: "partial", label: "REST API" },
      },
    ],
  },
];

const ZENDESK_QUESTIONS = [
  {
    question: "Why do Zendesk bills jump so fast once you add SLAs?",
    answer:
      "Suite Team ($69/agent monthly, $55 annual) excludes SLA policies, side conversations, skills-based routing, and multilingual help centers. Turning any one of those on forces every agent seat onto Suite Professional ($149/agent monthly, $115 annual).",
  },
  {
    question: "How do Zendesk automated resolutions compare with dimmi AI pricing?",
    answer:
      "Zendesk bundles 5 automated resolutions per agent on Suite Team (10 on Suite Professional) and charges $2.00 pay-as-you-go ($1.50 committed) beyond that. dimmi gives you 100 free conversations a month, starts paid AI chats at $0.10, and lets you connect your own OpenAI or Anthropic key to save 50% to 83%.",
  },
  {
    question: "When should an enterprise stay on Zendesk?",
    answer:
      "Stay on Zendesk if you run a 200+ seat support floor with complex telephony IVR trees, rely on specialized Zendesk Marketplace apps, or require vendor SOC 2, ISO 27001, or HIPAA sign-off today.",
  },
  {
    question: "Does dimmi include ticket types, SLA clocks, and routing on the free tier?",
    answer:
      "Yes. Custom ticket types, business-hour SLA policies, visual workflows, SAML SSO, and the full shared inbox unlock on every dimmi Cloud workspace and on self-hosted MIT servers.",
  },
];

function TierStackHeroVisual() {
  const tiers = [
    { name: "Suite Team", price: "$69/seat", note: "Monthly ($55 annual). No SLAs, no skills routing, 5 AR/seat." },
    {
      name: "Suite Professional",
      price: "$149/seat",
      note: "Monthly ($115 annual). Unlocks SLAs, skills routing, multilingual docs.",
    },
    {
      name: "+ Automated Resolutions",
      price: "+$2.00/res",
      note: "Pay-as-you-go AI resolutions after plan allowance.",
    },
    { name: "+ Copilot add-on", price: "+$50/seat", note: "Billed yearly on top of Suite seats." },
  ];

  return (
    <div className="flex flex-col gap-4 rounded-2xl border border-border-button-default bg-background-full p-6 shadow-xs">
      <div className="flex items-center justify-between border-b border-separator-border pb-3 font-mono text-[11px] text-text-tertiary uppercase">
        <span>How Zendesk Suite stacks up</span>
        <span>Checked {CHECKED_ON}</span>
      </div>

      <div className="flex flex-col gap-2">
        {tiers.map((tier) => (
          <div
            key={tier.name}
            className="flex items-start justify-between gap-3 rounded-xl border border-separator-border bg-background-secondary-default/35 px-4 py-3"
          >
            <div className="flex flex-col gap-0.5">
              <span className="text-[14px] font-medium text-text-primary">{tier.name}</span>
              <span className="text-[12px] text-text-secondary">{tier.note}</span>
            </div>
            <span className="shrink-0 font-mono text-[14px] font-medium text-text-primary tabular-nums">
              {tier.price}
            </span>
          </div>
        ))}
      </div>

      <div className="flex items-center justify-between rounded-xl border border-accent-500/40 bg-accent-500/[0.05] px-4 py-3">
        <div className="flex flex-col">
          <span className="text-[14px] font-medium text-text-primary">dimmi (all features unlocked)</span>
          <span className="text-[12px] text-text-secondary">
            SLAs, workflows, Operator, multilingual docs, MIT self-host
          </span>
        </div>
        <span className="font-mono text-[16px] font-medium text-accent-700 dark:text-accent-300">$0/seat</span>
      </div>
    </div>
  );
}

function ZendeskPlanPicker() {
  const seatsId = useId();
  const [seats, setSeats] = useState(6);
  const [needSlas, setNeedSlas] = useState(true);
  const [needSkills, setNeedSkills] = useState(false);
  const [needMultilingual, setNeedMultilingual] = useState(false);
  const [needCopilot, setNeedCopilot] = useState(false);

  const forcesPro = needSlas || needSkills || needMultilingual;
  const usage = { conversations: 3000, aiShare: 0.75, operator: 0, ownModel: false, seats };
  const competitor = zendesk.competitor;
  const baseZendesk = competitor ? (forcesPro ? fullBill(competitor, usage) : competitorBill(competitor, usage)) : 0;
  const copilotCost = needCopilot ? seats * 50 : 0;
  const totalZendesk = Math.round(baseZendesk + copilotCost);
  const dimmiCloud = Math.round(bill(usage).total);
  const dimmiByok = Math.round(bill({ ...usage, ownModel: true }).total);

  const toggles = [
    {
      label: "Business-hour SLA policies",
      checked: needSlas,
      flip: () => setNeedSlas((v) => !v),
      tag: "Forces Suite Pro",
    },
    {
      label: "Skills-based ticket routing",
      checked: needSkills,
      flip: () => setNeedSkills((v) => !v),
      tag: "Forces Suite Pro",
    },
    {
      label: "Multilingual help center",
      checked: needMultilingual,
      flip: () => setNeedMultilingual((v) => !v),
      tag: "Forces Suite Pro",
    },
    {
      label: "AI teammate copilot",
      checked: needCopilot,
      flip: () => setNeedCopilot((v) => !v),
      tag: "+$50/seat add-on",
    },
  ];

  return (
    <Section frameClassName="flex flex-col gap-10">
      <SplitHead
        tag="Zendesk plan picker"
        title="Tick the ops features you need and watch the required Zendesk tier jump."
        body="Models a team answering 3,000 monthly chats (75% AI-assisted) using Zendesk monthly list rates ($69 Suite Team vs $149 Suite Professional) and $2.00 automated resolution overages."
      />

      <div className="flex flex-col gap-6 rounded-2xl border border-border-button-default bg-background-full p-6 sm:p-8">
        <div className="flex flex-col gap-2">
          <div className="flex items-baseline justify-between">
            <label htmlFor={seatsId} className="text-[14px] font-medium text-text-primary">
              Support seats
            </label>
            <span className="font-mono text-[14px] font-medium text-text-primary tabular-nums">{seats} agents</span>
          </div>
          <input
            id={seatsId}
            type="range"
            min={2}
            max={25}
            step={1}
            value={seats}
            onChange={(e) => setSeats(Number(e.target.value))}
            className="w-full cursor-pointer accent-accent-600"
          />
        </div>

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          {toggles.map((item) => (
            <label
              key={item.label}
              className={cx(
                "flex cursor-pointer items-center justify-between gap-3 rounded-xl border px-4 py-3 transition-colors",
                item.checked ? "border-accent-500/45 bg-accent-500/[0.05]" : "border-border-button-default",
              )}
            >
              <span className="inline-flex items-center gap-2.5 text-[14px] text-text-primary">
                <input
                  type="checkbox"
                  checked={item.checked}
                  onChange={item.flip}
                  className="size-4 cursor-pointer accent-accent-600"
                />
                {item.label}
              </span>
              <span className="font-mono text-[11px] text-text-tertiary">{item.tag}</span>
            </label>
          ))}
        </div>

        <div className="grid grid-cols-1 gap-4 border-t border-separator-border pt-5 md:grid-cols-3">
          <div className="flex flex-col gap-1 rounded-xl border border-border-button-default bg-background-secondary-default/40 p-5">
            <span className="font-mono text-[11px] text-text-tertiary uppercase">
              Required: {forcesPro ? "Zendesk Suite Pro" : "Zendesk Suite Team"}
            </span>
            <span className="font-mono text-[30px] font-medium text-text-primary tabular-nums">
              {dollars(totalZendesk)}/mo
            </span>
            <span className="text-[12px] text-text-secondary">
              {forcesPro ? "$149/seat monthly" : "$69/seat monthly"} + $2.00 AR overages
              {needCopilot ? " + Copilot" : ""}
            </span>
          </div>

          <div className="flex flex-col gap-1 rounded-xl border border-accent-500/35 bg-accent-500/[0.04] p-5">
            <span className="font-mono text-[11px] text-accent-700 uppercase dark:text-accent-300">
              Managed AI tier
            </span>
            <span className="font-mono text-[30px] font-medium text-text-primary tabular-nums">
              {dollars(dimmiCloud)}/mo
            </span>
            <span className="text-[12px] text-text-secondary">All 4 ticked features included, $0 seats</span>
          </div>

          <div className="flex flex-col gap-1 rounded-xl border border-accent-500/35 bg-accent-500/[0.04] p-5">
            <span className="font-mono text-[11px] text-accent-700 uppercase dark:text-accent-300">Own model keys</span>
            <span className="font-mono text-[30px] font-medium text-text-primary tabular-nums">
              {dollars(dimmiByok)}/mo
            </span>
            <span className="text-[12px] text-text-secondary">Run on your own Linux server for zero license cost</span>
          </div>
        </div>
      </div>
    </Section>
  );
}

export function VsZendeskPage() {
  return (
    <div style={rivalToneStyle(zendesk)}>
      <VsHero
        rival={zendesk}
        eyebrow="dimmi vs Zendesk"
        title="Skip $69 to $149 seat tiers and $2.00 resolution fees."
        verdict={
          <p>
            Pick Zendesk if you run a 200+ seat call center needing SOC 2, ISO 27001, HIPAA, IVR trees, and 1,800+
            marketplace apps today. Pick dimmi if you want business-hour SLAs, visual routing, multilingual docs,
            unlimited free seats, and BYO AI keys on MIT code starting at $0 for {FREE_CONVERSATIONS} chats a month.
          </p>
        }
        authorId="frantz-kati"
        reviewedById="frantz-kati"
        published={CHECKED_ON_ISO}
        updated={CHECKED_ON_ISO}
        primaryCta={{ label: "Try dimmi Cloud free", href: `${APP_URL}/signup` }}
        secondaryCta={{ label: "Inspect MIT repo", href: GITHUB_URL, github: true }}
        visual={<TierStackHeroVisual />}
      />

      <WinCards
        tag="Zendesk trade-offs"
        title="Where Zendesk still wins, and where dimmi cuts the complexity."
        body="Verified against zendesk.com/pricing on November 27, 2026."
        rivalName="Zendesk"
        rivalWins={zendesk.rivalWins}
        dimmiWins={zendesk.dimmiWins}
      />

      <ZendeskPlanPicker />

      <BillDuel
        rival={zendesk}
        tag="Suite cost duel"
        title="Compare monthly bills at your team size."
        body="Includes Zendesk Suite Team ($69 monthly / $55 annual) and Suite Professional ($149 monthly / $115 annual) with $2.00 automated resolution overages."
        defaultSeats={6}
        defaultConversations={3000}
        defaultAiShare={0.75}
      />

      <ProofBand
        tag="SLAs and routing included"
        title="SLA policies and visual workflow routing on every dimmi plan."
        body="You never pay $149 a seat to turn on open-hour SLA clocks or conditional routing. Both screens ship on the free tier and on self-hosted MIT servers."
        caption="Left: dimmi SLA policies settings page. Right: dimmi visual workflow routing canvas."
      >
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
          <ProductShot
            name="vs-sla"
            alt="dimmi SLA policies settings screen showing open-hour response and resolution target controls"
          />
          <ProductShot
            name="workflow-canvas"
            alt="dimmi visual workflow canvas routing incoming support conversations across teams and SLAs"
          />
        </div>
        <div className="flex flex-wrap gap-6 pt-3">
          <MonoLink href="/vs">Browse comparison hub</MonoLink>
          <MonoLink href="/vs/intercom">Read Intercom breakdown</MonoLink>
          <MonoLink href="/vs/help-scout">Read Help Scout breakdown</MonoLink>
          <MonoLink href="/pricing">Open price book</MonoLink>
        </div>
      </ProofBand>

      <SwitchTriggers
        tag="Zendesk triggers"
        title="Four billing walls that push growing teams off Zendesk."
        body="Checked against zendesk.com/pricing on November 27, 2026."
        rivalName="Zendesk"
        triggers={zendesk.switchTriggers}
      />

      <VsMatrix
        title="Capabilities and real compliance gaps side by side."
        body="Every enterprise certification we do not hold yet is marked plainly."
        rivalName="Zendesk"
        groups={ZENDESK_MATRIX}
      />

      <TeamPicker
        rival={zendesk}
        tag="Zendesk team fit"
        title="Four support teams and which help desk fits each."
        body="Includes the enterprise call center that should stay on Zendesk today."
        teams={zendesk.teams}
      />

      <Questions items={ZENDESK_QUESTIONS} />

      <CtaBand
        title="Get SLAs, routing, and AI agents without $149 seats."
        links={[
          { label: "Create free workspace", href: `${APP_URL}/signup`, variant: "primary", icon: "arrow" },
          { label: "Deploy MIT binary", href: GITHUB_URL, variant: "secondary", icon: "github" },
        ]}
      />
    </div>
  );
}
