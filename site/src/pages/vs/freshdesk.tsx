/*
 * dimmi vs Freshdesk comparison page (`/vs/freshdesk`).
 * Unique elements: stacked 100-session Freddy pack hero visual, interactive
 * sessions-to-conversations converter, and real dimmi Policy Procedures +
 * SLA policies captures.
 */
import { useId, useState } from "react";

import { MonoLink, Section, SplitHead } from "../../components/container";
import { Questions } from "../../components/product-page";
import { ProductShot } from "../../components/product-shot";
import { CHECKED_ON, CHECKED_ON_ISO, getRival } from "../../data/rivals";
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

const freshdesk = getRival("freshdesk");

const FRESHDESK_MATRIX: VsMatrixGroup[] = [
  {
    title: "Seats, AI packs, and add-ons",
    rows: [
      {
        feature: "Agent seat pricing",
        dimmi: { status: "yes", label: "$0 unlimited" },
        rival: { status: "no", label: "$19 / $55 / $89 annual" },
      },
      {
        feature: "AI agent billing unit",
        dimmi: { status: "yes", label: "Per conversation or BYO key" },
        rival: { status: "no", label: "$49 per 100 expiring sessions" },
      },
      {
        feature: "Teammate AI copilot",
        dimmi: { status: "yes", label: "Operator included" },
        rival: { status: "partial", label: "$29/agent/mo (Pro+)" },
      },
      {
        feature: "MCP server actions",
        dimmi: { status: "yes", label: "$0 built in" },
        rival: { status: "partial", label: "$15 per 1,000 actions" },
      },
    ],
  },
  {
    title: "SLAs, procedures, and ownership",
    rows: [
      {
        feature: "Multiple SLA policies and multilingual docs",
        dimmi: { status: "yes", label: "Every workspace" },
        rival: { status: "partial", label: "Pro ($55/agent)" },
      },
      {
        feature: "Audit logs and sandbox/simulations",
        dimmi: { status: "yes", label: "Every workspace" },
        rival: { status: "partial", label: "Enterprise ($89/agent)" },
      },
      {
        feature: "Policy procedures with approval gates",
        dimmi: { status: "yes", label: "Built in" },
        rival: { status: "partial", label: "Workflow automator" },
      },
      {
        feature: "Open-source self-hosting",
        dimmi: { status: "yes", label: "100% MIT ($0)" },
        rival: { status: "no", label: "Closed SaaS" },
      },
    ],
  },
  {
    title: "Suite breadth and compliance",
    rows: [
      {
        feature: "SOC 2 and ISO 27001 certifications",
        requestTitle: "SOC 2 and ISO 27001 certification",
        dimmi: { status: "no" },
        rival: { status: "yes", label: "Certified" },
      },
      {
        feature: "Same-vendor CRM, ITSM, and call center",
        dimmi: { status: "no", label: "Support platform focus" },
        rival: { status: "yes", label: "Freshworks suite" },
      },
      {
        feature: "Entry manual-only seat rate (2 seats, 0% AI)",
        dimmi: { status: "partial", label: "$60 Cloud / $0 self-host" },
        rival: { status: "yes", label: "$38/mo Growth annual" },
      },
    ],
  },
];

const FRESHDESK_QUESTIONS = [
  {
    question: "How do Freshdesk Freddy AI session packs work?",
    answer:
      "Freshdesk includes 500 Freddy AI Agent sessions once per account, then bills $49 for each pack of 100 sessions ($0.49 per session). For email, a single session covers a 72-hour window, so a multi-week ticket can consume multiple sessions, and unused packs expire at the end of the billing cycle.",
  },
  {
    question: "When is Freshdesk Growth cheaper than dimmi Cloud?",
    answer:
      "If 2 agents answer 2,000 tickets entirely by hand with 0% AI and do not need custom reports or multiple SLA policies, Freshdesk Growth costs $38/month billed annually compared with $60/month on dimmi Cloud. Self-hosting dimmi under MIT stays $0 to dimmi.",
  },
  {
    question: "What does Freshdesk lock behind Pro ($55) and Enterprise ($89)?",
    answer:
      "Custom dashboards, multilingual helpdesks, intelligent routing, and multiple SLA policies require Pro ($55/agent/mo annual). Audit logs, skills-based routing, and sandboxes require Enterprise ($89/agent/mo annual). All of those ship on every dimmi workspace.",
  },
  {
    question: "Does dimmi charge extra for MCP calls or AI copilot seats?",
    answer:
      "No. Freshdesk bills Freddy Copilot at $29/agent/mo and MCP actions at $15 per 1,000 calls. dimmi includes its MCP server at $0 per call and bills Operator only through the standard conversation meter.",
  },
];

function SessionPacksHeroVisual() {
  const packs = Array.from({ length: 10 }, (_, idx) => idx + 1);
  return (
    <div className="flex flex-col gap-4 rounded-2xl border border-border-button-default bg-background-full p-6 shadow-xs">
      <div className="flex items-center justify-between border-b border-separator-border pb-3 font-mono text-[11px] text-text-tertiary uppercase">
        <span>1,000 AI sessions = 10 expiring packs</span>
        <span>Checked {CHECKED_ON}</span>
      </div>

      <div className="grid grid-cols-5 gap-2">
        {packs.map((pack) => (
          <div
            key={pack}
            className="flex flex-col items-center rounded-lg border border-separator-border bg-background-secondary-default/45 px-2 py-2.5 text-center font-mono"
          >
            <span className="text-[11px] text-text-tertiary">100</span>
            <span className="text-[13px] font-medium text-text-primary">$49</span>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-2 gap-3 pt-1">
        <div className="rounded-xl border border-separator-border bg-background-full p-3.5">
          <div className="font-mono text-[11px] text-text-tertiary uppercase">Freddy AI packs</div>
          <div className="font-mono text-[20px] font-medium text-text-primary tabular-nums">$490/mo</div>
          <div className="text-[12px] text-text-secondary">Plus $19 to $89/agent seats; expire monthly</div>
        </div>
        <div className="rounded-xl border border-accent-500/40 bg-accent-500/[0.05] p-3.5">
          <div className="font-mono text-[11px] text-accent-700 uppercase dark:text-accent-300">dimmi BYO key</div>
          <div className="font-mono text-[20px] font-medium text-text-primary tabular-nums">$44/mo</div>
          <div className="text-[12px] text-text-secondary">1,000 chats on Cloud + raw API tokens ($0 seats)</div>
        </div>
      </div>
    </div>
  );
}

function SessionsConverter() {
  const chatsId = useId();
  const reopenId = useId();
  const seatsId = useId();
  const [aiConversations, setAiConversations] = useState(1400);
  const [multiWindowPct, setMultiWindowPct] = useState(25);
  const [seats, setSeats] = useState(5);

  const totalSessions = Math.round(aiConversations * (1 + multiWindowPct / 100));
  const packsNeeded = Math.ceil(totalSessions / 100);
  const packSpend = packsNeeded * 49;
  const freshdeskGrowth = seats * 19 + packSpend;
  const freshdeskPro = seats * 55 + packSpend;

  const totalConv = Math.round(aiConversations / 0.7);
  const dimmiCloud = Math.round(bill({ conversations: totalConv, aiShare: 0.7, operator: 0, ownModel: false }).total);
  const dimmiByok = Math.round(bill({ conversations: totalConv, aiShare: 0.7, operator: 0, ownModel: true }).total);

  return (
    <Section frameClassName="flex flex-col gap-10">
      <SplitHead
        tag="Sessions to conversations converter"
        title="Convert real support threads into expiring 100-session Freddy packs."
        body="Because Freshdesk counts each 72-hour email window as a new AI session and rounds up in packs of 100 ($49/pack), multi-day threads expand your bill faster than flat conversations."
      />

      <div className="flex flex-col gap-6 rounded-2xl border border-border-button-default bg-background-full p-6 sm:p-8">
        <div className="grid grid-cols-1 gap-6 md:grid-cols-3">
          <div className="flex flex-col gap-2">
            <div className="flex items-baseline justify-between">
              <label htmlFor={chatsId} className="text-[13px] font-medium text-text-secondary">
                AI-handled conversations / mo
              </label>
              <span className="font-mono text-[14px] font-medium text-text-primary tabular-nums">
                {aiConversations.toLocaleString("en-US")}
              </span>
            </div>
            <input
              id={chatsId}
              type="range"
              min={200}
              max={5000}
              step={200}
              value={aiConversations}
              onChange={(e) => setAiConversations(Number(e.target.value))}
              className="w-full cursor-pointer accent-accent-600"
            />
          </div>

          <div className="flex flex-col gap-2">
            <div className="flex items-baseline justify-between">
              <label htmlFor={reopenId} className="text-[13px] font-medium text-text-secondary">
                Threads crossing 72h window
              </label>
              <span className="font-mono text-[14px] font-medium text-text-primary tabular-nums">
                {multiWindowPct}%
              </span>
            </div>
            <input
              id={reopenId}
              type="range"
              min={0}
              max={60}
              step={5}
              value={multiWindowPct}
              onChange={(e) => setMultiWindowPct(Number(e.target.value))}
              className="w-full cursor-pointer accent-accent-600"
            />
          </div>

          <div className="flex flex-col gap-2">
            <div className="flex items-baseline justify-between">
              <label htmlFor={seatsId} className="text-[13px] font-medium text-text-secondary">
                Agents
              </label>
              <span className="font-mono text-[14px] font-medium text-text-primary tabular-nums">{seats} seats</span>
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
        </div>

        <div className="grid grid-cols-1 gap-4 border-t border-separator-border pt-5 md:grid-cols-4">
          <div className="flex flex-col gap-1 rounded-xl border border-border-button-default bg-background-secondary-default/40 p-4">
            <span className="font-mono text-[11px] text-text-tertiary uppercase">Freddy sessions needed</span>
            <span className="font-mono text-[26px] font-medium text-text-primary tabular-nums">
              {packsNeeded} packs
            </span>
            <span className="text-[12px] text-text-secondary">
              {`${totalSessions.toLocaleString("en-US")} sessions = ${dollars(packSpend)}/mo`}
            </span>
          </div>

          <div className="flex flex-col gap-1 rounded-xl border border-border-button-default bg-background-full p-4">
            <span className="font-mono text-[11px] text-text-tertiary uppercase">Freshdesk Growth / Pro</span>
            <span className="font-mono text-[26px] font-medium text-text-primary tabular-nums">
              {dollars(freshdeskGrowth)} / {dollars(freshdeskPro)}
            </span>
            <span className="text-[12px] text-text-secondary">Growth ($19) or Pro ($55) + packs</span>
          </div>

          <div className="flex flex-col gap-1 rounded-xl border border-accent-500/35 bg-accent-500/[0.04] p-4">
            <span className="font-mono text-[11px] text-accent-700 uppercase dark:text-accent-300">
              Cloud managed AI
            </span>
            <span className="font-mono text-[26px] font-medium text-text-primary tabular-nums">
              {dollars(dimmiCloud)}/mo
            </span>
            <span className="text-[12px] text-text-secondary">Quiet 7-day thread window, $0 seats</span>
          </div>

          <div className="flex flex-col gap-1 rounded-xl border border-accent-500/35 bg-accent-500/[0.04] p-4">
            <span className="font-mono text-[11px] text-accent-700 uppercase dark:text-accent-300">Bring your key</span>
            <span className="font-mono text-[26px] font-medium text-text-primary tabular-nums">
              {dollars(dimmiByok)}/mo
            </span>
            <span className="text-[12px] text-text-secondary">Host on your own box for zero software cost</span>
          </div>
        </div>
      </div>
    </Section>
  );
}

export function VsFreshdeskPage() {
  return (
    <div style={rivalToneStyle(freshdesk)}>
      <VsHero
        rival={freshdesk}
        eyebrow="dimmi vs Freshdesk"
        title="Clear per-conversation AI pricing with zero expiring session packs."
        verdict={
          <p>
            Pick Freshdesk if you want a low $19/agent annual seat price for manual ticketing, already use Freshworks
            CRM and phone, or need SOC 2 and ISO 27001 today. Pick dimmi if you want unlimited free seats, multiple SLA
            policies, audit logs, BYO AI keys without $49 expiring session packs, and MIT open-source code starting at
            $0 for {FREE_CONVERSATIONS} chats a month.
          </p>
        }
        authorId="frantz-kati"
        reviewedById="frantz-kati"
        published={CHECKED_ON_ISO}
        updated={CHECKED_ON_ISO}
        primaryCta={{ label: "Try dimmi free", href: `${APP_URL}/signup` }}
        secondaryCta={{ label: "Get MIT repo", href: GITHUB_URL, github: true }}
        visual={<SessionPacksHeroVisual />}
      />

      <WinCards
        tag="Freshdesk trade-offs"
        title="Where Freshdesk is strong, and where dimmi strips out the pack math."
        body="Verified against freshworks.com/freshdesk/pricing/ on November 27, 2026."
        rivalName="Freshdesk"
        rivalWins={freshdesk.rivalWins}
        dimmiWins={freshdesk.dimmiWins}
      />

      <SessionsConverter />

      <BillDuel
        rival={freshdesk}
        tag="Session pack duel"
        title="Compare Freshdesk Growth and Pro against dimmi."
        body="Freshdesk Growth bills $19/agent annual, Pro bills $55/agent for multiple SLAs and multilingual docs, and Freddy AI Agent adds $49 per 100 sessions."
        defaultSeats={5}
        defaultConversations={2000}
        defaultAiShare={0.75}
      />

      <ProofBand
        tag="Procedures and SLAs included"
        title="Approval-gated policy procedures and SLA clocks on every dimmi plan."
        body="Pause risky refund or account actions for human sign-off, simulate policy changes before publishing, and run multiple SLA policies without upgrading to Pro or Enterprise."
        caption="Left: dimmi Policy Procedures builder with explicit approval steps. Right: dimmi SLA policies settings screen."
      >
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
          <ProductShot
            name="procedure-hero"
            alt="dimmi Policy Procedures editor showing structured support rules, approval gates, and test simulations"
          />
          <ProductShot
            name="vs-sla"
            alt="dimmi SLA policies settings screen showing open-hour response and resolution targets"
          />
        </div>
        <div className="flex flex-wrap gap-6 pt-3">
          <MonoLink href="/vs">Full comparison directory</MonoLink>
          <MonoLink href="/vs/front">Read Front breakdown</MonoLink>
          <MonoLink href="/procedures">Explore Policy Procedures</MonoLink>
          <MonoLink href="/pricing">Check conversation rates</MonoLink>
        </div>
      </ProofBand>

      <SwitchTriggers
        tag="Freshdesk triggers"
        title="Four Freshdesk add-ons that send teams looking for an alternative."
        body="Checked against freshworks.com/freshdesk/pricing/ on November 27, 2026."
        rivalName="Freshdesk"
        triggers={freshdesk.switchTriggers}
      />

      <VsMatrix
        title="Pricing, AI policies, and compliance side by side."
        body="Includes the certifications and Freshworks suite integrations where Freshdesk wins today."
        rivalName="Freshdesk"
        groups={FRESHDESK_MATRIX}
      />

      <TeamPicker
        rival={freshdesk}
        tag="Freshdesk team fit"
        title="Four teams and which help desk makes the most sense."
        body="Includes both the manual-only Growth team and the Freshworks-suite team where Freshdesk can win."
        teams={freshdesk.teams}
      />

      <Questions items={FRESHDESK_QUESTIONS} />

      <CtaBand
        title="Ditch the expiring AI session packs and per-agent upgrades."
        links={[
          { label: "Sign up free today", href: `${APP_URL}/signup`, variant: "primary", icon: "arrow" },
          { label: "Self-host MIT stack", href: GITHUB_URL, variant: "secondary", icon: "github" },
        ]}
      />
    </div>
  );
}
