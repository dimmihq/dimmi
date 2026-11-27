/*
 * dimmi vs Crisp comparison page (`/vs/crisp`).
 * Unique elements: flat workspace plan with fixed seat count hero visual,
 * interactive seats-included vs free-seats slider, and real dimmi team members
 * capture (`vs-members`) proving unlimited free seats.
 */
import { useId, useState } from "react";

import { cx } from "@/utils/cx";

import { MonoLink, Section, SplitHead } from "../../components/container";
import { Questions } from "../../components/product-page";
import { ProductShot } from "../../components/product-shot";
import { CHECKED_ON, CHECKED_ON_ISO, getRival } from "../../data/rivals";
import { APP_URL, GITHUB_URL } from "../../links";
import { dollars, FREE_CONVERSATIONS } from "../../pricing/prices";
import {
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

const crisp = getRival("crisp");

const CRISP_MATRIX: VsMatrixGroup[] = [
  {
    title: "Workspace plans and seat caps",
    rows: [
      {
        feature: "Included team seats",
        dimmi: { status: "yes", label: "Unlimited ($0)" },
        rival: { status: "partial", label: "2 / 4 / 10 / 20 (+$10 extra)" },
      },
      {
        feature: "Structured ticketing system",
        dimmi: { status: "yes", label: "Every workspace" },
        rival: { status: "partial", label: "Plus ($295/workspace/mo)" },
      },
      {
        feature: "Workflows and knowledge base",
        dimmi: { status: "yes", label: "Every workspace" },
        rival: { status: "partial", label: "Essentials ($95/mo+)" },
      },
      {
        feature: "Customer profile caps",
        dimmi: { status: "yes", label: "Unlimited contacts" },
        rival: { status: "partial", label: "100 / 5k / 50k / 200k" },
      },
    ],
  },
  {
    title: "AI models, APIs, and self-hosting",
    rows: [
      {
        feature: "Bring your own LLM API key",
        dimmi: { status: "yes", label: "OpenAI, Anthropic & more" },
        rival: { status: "no", label: "Bundled $5-$75 AI credits" },
      },
      {
        feature: "Policy procedures with approval gates",
        dimmi: { status: "yes", label: "Built in" },
        rival: { status: "partial", label: "Bot workflow scenarios" },
      },
      {
        feature: "MIT open-source self-hosting",
        dimmi: { status: "yes", label: "100% MIT ($0 software)" },
        rival: { status: "no", label: "Closed SaaS" },
      },
    ],
  },
  {
    title: "Small-team polish and extras",
    rows: [
      {
        feature: "Built-in co-browsing (MagicBrowse)",
        requestTitle: "Co-browsing and live screen sharing",
        dimmi: { status: "no" },
        rival: { status: "yes", label: "Built in" },
      },
      {
        feature: "100+ prebuilt marketplace apps",
        requestTitle: "Third-party app marketplace",
        dimmi: { status: "no" },
        rival: { status: "yes", label: "100+ integrations" },
      },
      {
        feature: "Built-in product changelog and feedback board",
        dimmi: { status: "yes", label: "Included" },
        rival: { status: "partial", label: "Status page only" },
      },
    ],
  },
];

const CRISP_QUESTIONS = [
  {
    question: "How do Crisp's flat workspace plans compare with dimmi's free seats?",
    answer:
      "Crisp's flat pricing is great when your team stays small: Mini ($45/mo) covers 4 seats and Essentials ($95/mo) covers 10 seats. Every seat above the cap costs $10/month, and unlocking structured ticketing jumps the base fee to Crisp Plus ($295/month). On dimmi, seats and ticketing are always unlocked.",
  },
  {
    question: "When is Crisp a better choice than dimmi?",
    answer:
      "Pick Crisp if you are a 2-to-4 person team answering mostly manual live chats who love Crisp's status page, co-browsing, and flat $45 Mini bill without needing custom ticketing, BYO AI keys, or self-hosting.",
  },
  {
    question: "How does Crisp handle AI automation pricing?",
    answer:
      "Crisp bundles monthly AI credits ($5 on Mini for roughly 90 automated replies, $25 on Essentials for roughly 450, and $75 on Plus for roughly 1,350). dimmi lets you attach your own OpenAI, Anthropic, or OpenRouter API key so you pay raw model rates instead of credit buckets.",
  },
  {
    question: "What plans do 1, 5, and 25 person teams need on Crisp?",
    answer:
      "A solo founder with basic chat can start on Crisp Free ($0 for 2 seats, 100 profiles) or Mini ($45) for email. A 5-person team needing workflows and WhatsApp lands on Essentials ($95). A 25-person team needing ticketing lands on Plus ($295) plus 5 extra seats ($50) for $345/month before AI overages.",
  },
];

function CrispFlatCapHeroVisual() {
  const tiers = [
    { name: "Free", price: "$0/mo", seats: "2 seats", cap: "100 profiles · no email" },
    { name: "Mini", price: "$45/mo", seats: "4 seats", cap: "5k profiles · $5 AI credits" },
    { name: "Essentials", price: "$95/mo", seats: "10 seats", cap: "50k profiles · workflows · $25 AI" },
    { name: "Plus", price: "$295/mo", seats: "20 seats", cap: "Ticketing unlocked · +$10/extra seat" },
  ];

  return (
    <div className="flex flex-col gap-4 rounded-2xl border border-border-button-default bg-background-full p-6 shadow-xs">
      <div className="flex items-center justify-between border-b border-separator-border pb-3 font-mono text-[11px] text-text-tertiary uppercase">
        <span>Crisp workspace seat caps</span>
        <span>Checked {CHECKED_ON}</span>
      </div>

      <div className="flex flex-col gap-2">
        {tiers.map((tier) => (
          <div
            key={tier.name}
            className="flex items-center justify-between gap-3 rounded-xl border border-separator-border bg-background-secondary-default/35 px-4 py-2.5"
          >
            <div className="flex flex-col">
              <span className="text-[14px] font-medium text-text-primary">
                {tier.name} · {tier.seats}
              </span>
              <span className="text-[12px] text-text-secondary">{tier.cap}</span>
            </div>
            <span className="font-mono text-[14px] font-medium text-text-primary tabular-nums">{tier.price}</span>
          </div>
        ))}
      </div>

      <div className="flex items-center justify-between rounded-xl border border-accent-500/40 bg-accent-500/[0.05] px-4 py-3">
        <span className="text-[13px] font-medium text-text-primary">
          dimmi: unlimited seats + ticketing on every tier
        </span>
        <span className="font-mono text-[15px] font-medium text-accent-700 dark:text-accent-300">$0/seat</span>
      </div>
    </div>
  );
}

function SeatsIncludedVsFree() {
  const seatsId = useId();
  const [seats, setSeats] = useState(12);
  const [needTickets, setNeedTickets] = useState(true);
  const [needWorkflows, setNeedWorkflows] = useState(true);

  const planName = needTickets ? "Plus" : needWorkflows ? "Essentials" : seats <= 4 ? "Mini" : "Essentials";
  const basePrice = planName === "Plus" ? 295 : planName === "Essentials" ? 95 : 45;
  const seatCap = planName === "Plus" ? 20 : planName === "Essentials" ? 10 : 4;
  const extraSeats = Math.max(0, seats - seatCap);
  const crispMonthly = basePrice + extraSeats * 10;

  const teamRows = [
    {
      size: "1 person (solo)",
      crispPlan: "Mini ($45) for email + chat",
      dimmiCloud: "$0 (first 100 chats) to ~$25",
      selfHost: "$0 + VPS",
    },
    {
      size: "5 people (SaaS)",
      crispPlan: "Essentials ($95) or Plus ($295 w/ tickets)",
      dimmiCloud: "~$148/mo (2k chats, AI)",
      selfHost: "$0 + VPS",
    },
    {
      size: "25 people (all-hands)",
      crispPlan: "Plus ($295) + 5 seats ($50) = $345+",
      dimmiCloud: "Same conversation bill, $0 seats",
      selfHost: "$0 + VPS",
    },
  ];

  return (
    <Section frameClassName="flex flex-col gap-10">
      <SplitHead
        tag="Seats included vs free seats"
        title="See when flat workspace seat caps start charging $10 per extra teammate."
        body="Adjust your teammate count and toggle ticketing or workflows to see which Crisp workspace tier you land on."
      />

      <div className="flex flex-col gap-6 rounded-2xl border border-border-button-default bg-background-full p-6 sm:p-8">
        <div className="flex flex-col gap-2">
          <div className="flex items-baseline justify-between">
            <label htmlFor={seatsId} className="text-[14px] font-medium text-text-primary">
              People with inbox access (support + engineers + founders)
            </label>
            <span className="font-mono text-[14px] font-medium text-text-primary tabular-nums">{seats} teammates</span>
          </div>
          <input
            id={seatsId}
            type="range"
            min={1}
            max={35}
            step={1}
            value={seats}
            onChange={(e) => setSeats(Number(e.target.value))}
            className="w-full cursor-pointer accent-accent-600"
          />
        </div>

        <div className="flex flex-wrap gap-3">
          <button
            type="button"
            aria-pressed={needWorkflows}
            onClick={() => setNeedWorkflows((v) => !v)}
            className={cx(
              "cursor-pointer rounded-xl border px-4 py-2 text-[14px] font-medium transition-colors",
              needWorkflows
                ? "border-accent-500/50 bg-accent-500/[0.08] text-text-primary"
                : "border-border-button-default text-text-secondary",
            )}
          >
            Workflows, KB, & WhatsApp (Essentials)
          </button>
          <button
            type="button"
            aria-pressed={needTickets}
            onClick={() => setNeedTickets((v) => !v)}
            className={cx(
              "cursor-pointer rounded-xl border px-4 py-2 text-[14px] font-medium transition-colors",
              needTickets
                ? "border-accent-500/50 bg-accent-500/[0.08] text-text-primary"
                : "border-border-button-default text-text-secondary",
            )}
          >
            Structured Ticketing (Plus $295)
          </button>
        </div>

        <div className="grid grid-cols-1 gap-4 border-t border-separator-border pt-5 md:grid-cols-3">
          <div className="flex flex-col gap-1 rounded-xl border border-border-button-default bg-background-secondary-default/40 p-5">
            <span className="font-mono text-[11px] text-text-tertiary uppercase">Crisp {planName}</span>
            <span className="font-mono text-[30px] font-medium text-text-primary tabular-nums">
              {dollars(crispMonthly)}/mo
            </span>
            <span className="text-[12px] text-text-secondary">
              {`${dollars(basePrice)} base (${seatCap} seats) + ${extraSeats} extra x $10`}
            </span>
          </div>

          <div className="flex flex-col gap-1 rounded-xl border border-accent-500/35 bg-accent-500/[0.04] p-5">
            <span className="font-mono text-[11px] text-accent-700 uppercase dark:text-accent-300">
              dimmi seats cost
            </span>
            <span className="font-mono text-[30px] font-medium text-text-primary tabular-nums">$0/mo</span>
            <span className="text-[12px] text-text-secondary">All {seats} teammates included; pay per chat only</span>
          </div>

          <div className="flex flex-col gap-1 rounded-xl border border-accent-500/35 bg-accent-500/[0.04] p-5">
            <span className="font-mono text-[11px] text-accent-700 uppercase dark:text-accent-300">
              dimmi self-hosted
            </span>
            <span className="font-mono text-[30px] font-medium text-text-primary tabular-nums">$0/mo</span>
            <span className="text-[12px] text-text-secondary">MIT software + your VPS and own LLM API</span>
          </div>
        </div>

        <div className="overflow-x-auto rounded-xl border border-separator-border">
          <table className="w-full min-w-[540px] border-collapse text-start text-[13px]">
            <thead>
              <tr className="border-b border-separator-border bg-background-secondary-default/50 font-mono text-[11px] text-text-tertiary uppercase">
                <th className="px-4 py-3 text-start">Team size</th>
                <th className="px-4 py-3 text-start">Crisp workspace plan</th>
                <th className="px-4 py-3 text-start">dimmi Cloud</th>
                <th className="px-4 py-3 text-start">dimmi MIT</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-separator-border">
              {teamRows.map((row) => (
                <tr key={row.size}>
                  <td className="px-4 py-3 font-medium text-text-primary">{row.size}</td>
                  <td className="px-4 py-3 text-text-secondary">{row.crispPlan}</td>
                  <td className="px-4 py-3 text-text-primary">{row.dimmiCloud}</td>
                  <td className="px-4 py-3 text-accent-700 dark:text-accent-300">{row.selfHost}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </Section>
  );
}

export function VsCrispPage() {
  return (
    <div style={rivalToneStyle(crisp)}>
      <VsHero
        rival={crisp}
        eyebrow="dimmi vs Crisp"
        title="Unlimited free seats and built-in ticketing without $295 upgrades."
        verdict={
          <p>
            Pick Crisp if you are a small team under 4 to 10 people who want a polished flat-rate chat widget,
            co-browsing, and status page with predictable monthly billing. Choose dimmi when your whole company needs
            free inbox seats, structured ticketing without a $295 jump, direct OpenAI or Anthropic API keys, and MIT
            self-hosting starting at {FREE_CONVERSATIONS} free chats a month.
          </p>
        }
        authorId="frantz-kati"
        reviewedById="frantz-kati"
        published={CHECKED_ON_ISO}
        updated={CHECKED_ON_ISO}
        primaryCta={{ label: "Invite your team free", href: `${APP_URL}/signup` }}
        secondaryCta={{ label: "Inspect MIT repo", href: GITHUB_URL, github: true }}
        visual={<CrispFlatCapHeroVisual />}
      />

      <WinCards
        tag="Crisp trade-offs"
        title="Where Crisp's flat plans work well, and where dimmi unlocks more."
        body="Checked on crisp.chat/en/pricing late November 2026."
        rivalName="Crisp"
        rivalWins={crisp.rivalWins}
        dimmiWins={crisp.dimmiWins}
      />

      <SeatsIncludedVsFree />

      <ProofBand
        tag="Unlimited free seats"
        title="Invite engineers, founders, and support reps without hitting a seat wall."
        body="Captured straight from a live dimmi workspace. Invite teammates, assign Owner, Admin, or Member roles, and import or export CSV rosters with zero per-seat billing."
        caption="Real dimmi Members settings screen showing active teammates, Invite controls, and roles included at $0/seat."
      >
        <ProductShot
          name="vs-members"
          alt="dimmi Members settings page showing 6 active workspace teammates, Invite button, CSV import/export, and role badges"
        />
        <div className="flex flex-wrap gap-6 pt-3">
          <MonoLink href="/vs">Compare other tools</MonoLink>
          <MonoLink href="/vs/tawk-to">Read Tawk.to breakdown</MonoLink>
          <MonoLink href="/vs/intercom">Read Intercom breakdown</MonoLink>
          <MonoLink href="/pricing">Open full pricing</MonoLink>
        </div>
      </ProofBand>

      <SwitchTriggers
        tag="Crisp switch triggers"
        title="Why growing software teams move from Crisp to dimmi."
        body="Verified directly on Crisp's Free, Mini, Essentials, and Plus pricing table."
        rivalName="Crisp"
        triggers={crisp.switchTriggers}
      />

      <VsMatrix
        title="Workspace limits, AI credits, and features side by side."
        body="Marks co-browsing and marketplace integrations where Crisp leads today."
        rivalName="Crisp"
        groups={CRISP_MATRIX}
      />

      <TeamPicker
        rival={crisp}
        tag="Crisp team fit"
        title="Four real teams and which support tool fits each."
        body="Includes the 3-person manual live-chat shop where Crisp Mini ($45 flat) is the cleaner choice."
        teams={crisp.teams}
        rivalMonthlyForTeam={(team) => {
          if (team.rivalPlanLabel.includes("Mini")) {
            return 45;
          }
          if (team.rivalPlanLabel.includes("Essentials")) {
            return 95;
          }
          if (team.seats > 20) {
            return 295 + (team.seats - 20) * 10;
          }
          return 295;
        }}
      />

      <Questions items={CRISP_QUESTIONS} />

      <CtaBand
        title="Bring your whole team into the inbox without $10 extra seats."
        links={[
          { label: "Create free workspace", href: `${APP_URL}/signup`, variant: "primary", icon: "arrow" },
          { label: "Self-host on GitHub", href: GITHUB_URL, variant: "secondary", icon: "github" },
        ]}
      />
    </div>
  );
}
