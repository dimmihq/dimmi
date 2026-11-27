/*
 * dimmi vs Intercom comparison page (`/vs/intercom`).
 * Built on the shared `/vs` kit with Intercom-specific hero visual (Fin $0.99
 * outcome meter), resolution-share slider with $29 list vs $19 promo toggle,
 * real dimmi BYO-key and reports captures, and 4-command migration guide.
 */
import { RiArrowRightLine, RiCheckLine, RiTerminalBoxLine } from "@remixicon/react";
import { useId, useState } from "react";

import { cx } from "@/utils/cx";

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

const intercom = getRival("intercom");

const INTERCOM_MATRIX: VsMatrixGroup[] = [
  {
    title: "Pricing and ownership",
    rows: [
      {
        feature: "Human support seats",
        dimmi: { status: "yes", label: "$0 unlimited" },
        rival: { status: "no", label: "$29 to $132/seat" },
      },
      {
        feature: "Free monthly tier",
        dimmi: { status: "yes", label: `${FREE_CONVERSATIONS} chats/mo` },
        rival: { status: "no", label: "14-day trial only" },
      },
      {
        feature: "AI agent billing model",
        dimmi: { status: "yes", label: "From $0.10/chat" },
        rival: { status: "no", label: "$0.99/Fin outcome" },
      },
      {
        feature: "Bring your own OpenAI or Anthropic key",
        dimmi: { status: "yes", label: "50% to 83% off AI" },
        rival: { status: "no", label: "Not supported" },
      },
      {
        feature: "MIT open-source self-hosting",
        dimmi: { status: "yes", label: "$0 software" },
        rival: { status: "no", label: "Closed cloud" },
      },
    ],
  },
  {
    title: "Inbox, workflows, and SLAs",
    rows: [
      {
        feature: "Visual workflow builder and round robin",
        dimmi: { status: "yes", label: "Every workspace" },
        rival: { status: "partial", label: "Advanced ($85/seat)" },
      },
      {
        feature: "Multiple team inboxes",
        dimmi: { status: "yes", label: "Every workspace" },
        rival: { status: "partial", label: "Advanced ($85/seat)" },
      },
      {
        feature: "Business-hour SLAs and SAML SSO",
        dimmi: { status: "yes", label: "Every workspace" },
        rival: { status: "partial", label: "Expert ($132/seat)" },
      },
      {
        feature: "AI teammate copilot",
        dimmi: { status: "yes", label: "Operator included" },
        rival: { status: "partial", label: "$29/seat add-on" },
      },
      {
        feature: "Policy procedures with simulations",
        dimmi: { status: "yes", label: "Included" },
        rival: { status: "yes", label: "Fin Procedures" },
      },
    ],
  },
  {
    title: "Ecosystem, mobile, and compliance",
    rows: [
      {
        feature: "SOC 2 Type II report",
        requestTitle: "SOC 2 Type II report",
        dimmi: { status: "no" },
        rival: { status: "yes", label: "Certified" },
      },
      {
        feature: "HIPAA BAA and ISO 27001",
        requestTitle: "HIPAA BAA and ISO 27001",
        dimmi: { status: "no" },
        rival: { status: "yes", label: "Expert / Enterprise" },
      },
      {
        feature: "Third-party app marketplace",
        requestTitle: "Third-party app marketplace",
        dimmi: { status: "no" },
        rival: { status: "yes", label: "400+ apps" },
      },
      {
        feature: "Native iOS and Android messenger SDKs",
        requestTitle: "Native iOS and Android messenger SDKs",
        dimmi: { status: "no" },
        rival: { status: "yes", label: "iOS & Android" },
      },
      {
        feature: "REST API, signed webhooks, and MCP server",
        dimmi: { status: "yes", label: "Included" },
        rival: { status: "partial", label: "API + MCP" },
      },
    ],
  },
];

const MIGRATE_COMMANDS = [
  { cmd: "./dimmi migrate connect", note: "Sign in to dimmi and paste your Intercom token in your browser" },
  { cmd: "./dimmi migrate plan", note: "Print exact counts for what moves across and what stays behind" },
  { cmd: "./dimmi migrate run", note: "Import history safely with resume support and zero duplicate records" },
  { cmd: "./dimmi migrate verify", note: "Compare Intercom and dimmi counts kind by kind before switching" },
];

const MOVES_ACROSS = [
  "Teammates (including former authors) and team groups",
  "Contacts, leads, companies, tags, notes, and custom attributes",
  "Full conversation history, internal notes, Fin replies, and attachments",
  "Ticket types, states, tickets, and office hours schedules",
  "Multilingual help center collections, articles, and URL redirects",
  "Macros converted into dimmi saved replies with mapped placeholders",
];

const STAYS_BEHIND = [
  "Saved segments (rebuild as dimmi views or audiences)",
  "Outbound messages, product tours, banners, posts, and series",
  "Fin settings, custom reports, and historical SLA statistics",
  "Device, browser, UTM fields, and SMS consent flags",
  "Intercom's messenger script (swap to dimmi's snippet when ready)",
];

const INTERCOM_QUESTIONS = [
  {
    question: "Can dimmi replace Intercom?",
    answer:
      "For web SaaS and software teams that need a shared inbox, live chat, email, Slack, WhatsApp, tickets, workflows, help center, and AI agents, yes. If you require a vendor SOC 2 report, HIPAA BAA, native mobile messenger SDKs, or Product Tours today, stay with Intercom until those ship.",
  },
  {
    question: "How much does Fin cost next to dimmi's agent?",
    answer:
      "Intercom bills Fin at $0.99 per outcome on every plan, on top of seat fees ($29 to $132 per seat). dimmi includes AI inside conversation pricing starting at $0.10 per AI conversation after 100 free conversations each month, or drops to the human rate (50% to 83% less) when you plug in your own OpenAI or Anthropic key.",
  },
  {
    question: "Can I self-host dimmi?",
    answer:
      "Yes. dimmi is MIT licensed with no enterprise paywall folder. You can install the exact same codebase as dimmi Cloud on one Linux server in about 5 minutes and pay dimmi $0 forever.",
  },
  {
    question: "Can I use my own OpenAI or Anthropic key?",
    answer:
      "Yes. Add your OpenAI, Anthropic, DeepSeek, Mistral, or OpenRouter key under Settings, AI providers. Your model provider bills you directly at raw API token prices and dimmi bills only the human conversation rate on Cloud, or $0 on self-hosted.",
  },
  {
    question: "Do seats cost money on dimmi?",
    answer:
      "No. Every teammate seat is free on dimmi Cloud and on self-hosted installs. Invite engineers, founders, and finance reviewers into the inbox without changing your monthly bill.",
  },
  {
    question: "Is dimmi open-source?",
    answer:
      "Yes. The Go backend, React dashboard, messenger widget, and CLI live on GitHub under the MIT license. Workflows, SLAs, SAML SSO, procedures, and Operator ship in the open repository.",
  },
];

function FinOutcomeMeterVisual() {
  const sampleSeats = 5;
  const sampleConversations = 2000;
  const sampleOutcomes = 960;
  const seatFees = sampleSeats * 29;
  const finFees = Math.round(sampleOutcomes * 0.99);
  const intercomTotal = seatFees + finFees;
  const dimmiCloud = Math.round(
    bill({ conversations: sampleConversations, aiShare: 0.8, operator: 0, ownModel: false }).total,
  );
  const dimmiByok = Math.round(
    bill({ conversations: sampleConversations, aiShare: 0.8, operator: 0, ownModel: true }).total,
  );

  return (
    <div className="flex flex-col gap-5 rounded-2xl border border-border-button-default bg-background-full p-6 shadow-xs">
      <div className="flex items-center justify-between gap-2 border-b border-separator-border pb-4">
        <span className="font-mono text-[11px] font-medium tracking-[0.08em] text-text-tertiary uppercase">
          5 seats · 2,000 chats · 960 Fin outcomes
        </span>
        <span className="rounded-full bg-background-secondary-default px-2.5 py-0.5 font-mono text-[11px] text-text-secondary">
          +$0.99 / outcome
        </span>
      </div>

      <div className="flex flex-col gap-3">
        <div className="flex items-baseline justify-between">
          <span className="text-[14px] font-medium text-text-primary">Intercom Essential + Fin</span>
          <span className="font-mono text-[22px] font-medium text-text-primary tabular-nums">
            {dollars(intercomTotal)}/mo
          </span>
        </div>
        <div className="flex h-3.5 w-full overflow-hidden rounded-full bg-background-secondary-default">
          <span
            className="h-full"
            style={{ width: `${(seatFees / intercomTotal) * 100}%`, backgroundColor: "var(--rival)", opacity: 0.45 }}
            title={`Seats: ${dollars(seatFees)}`}
          />
          <span
            className="h-full"
            style={{ width: `${(finFees / intercomTotal) * 100}%`, backgroundColor: "var(--rival)" }}
            title={`Fin outcomes: ${dollars(finFees)}`}
          />
        </div>
        <div className="flex flex-wrap items-center justify-between gap-2 font-mono text-[12px] text-text-tertiary tabular-nums">
          <span>5 seats ($29 list): {dollars(seatFees)}</span>
          <span>960 Fin outcomes x $0.99: {dollars(finFees)}</span>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-3 border-t border-separator-border pt-4 sm:grid-cols-2">
        <div className="flex flex-col gap-1 rounded-xl border border-accent-500/35 bg-accent-500/[0.04] p-4">
          <span className="font-mono text-[11px] text-accent-700 uppercase dark:text-accent-300">
            dimmi Cloud · AI included
          </span>
          <span className="font-mono text-[22px] font-medium text-text-primary tabular-nums">
            {dollars(dimmiCloud)}/mo
          </span>
          <span className="text-[12px] text-text-secondary">Unlimited seats, 100 free chats</span>
        </div>
        <div className="flex flex-col gap-1 rounded-xl border border-accent-500/35 bg-accent-500/[0.04] p-4">
          <span className="font-mono text-[11px] text-accent-700 uppercase dark:text-accent-300">
            dimmi Cloud · BYO key
          </span>
          <span className="font-mono text-[22px] font-medium text-text-primary tabular-nums">
            {dollars(dimmiByok)}/mo
          </span>
          <span className="text-[12px] text-text-secondary">Plus raw OpenAI or Anthropic API</span>
        </div>
      </div>

      <p className="text-[12px] leading-5 text-text-tertiary">
        Essential list rate is $29/seat/mo on annual billing ($19/seat/mo new-customer promo cuts seats to $95, leaving
        $950 in Fin outcome charges). Checked {CHECKED_ON}.
      </p>
    </div>
  );
}

function FinResolutionSlider() {
  const sliderId = useId();
  const [resolutionPct, setResolutionPct] = useState(60);
  const [promoSeat, setPromoSeat] = useState(false);

  const seats = 5;
  const conversations = 2000;
  const aiShare = 0.8;
  const aiAnswered = Math.round(conversations * aiShare);
  const outcomes = Math.round(aiAnswered * (resolutionPct / 100));
  const seatUnit = promoSeat ? 19 : 29;
  const seatSpend = seats * seatUnit;
  const finSpend = Math.round(outcomes * 0.99);
  const totalIntercom = seatSpend + finSpend;
  const finSharePct = totalIntercom > 0 ? Math.round((finSpend / totalIntercom) * 100) : 0;
  const dimmiCloud = Math.round(bill({ conversations, aiShare, operator: 0, ownModel: false }).total);
  const dimmiByok = Math.round(bill({ conversations, aiShare, operator: 0, ownModel: true }).total);

  return (
    <Section frameClassName="flex flex-col gap-10">
      <SplitHead
        tag="The Fin meter"
        title="The better Fin gets, the faster your bill climbs."
        body="Slide your AI resolution rate for a 5-person team handling 2,000 monthly chats (80% answered by AI). Toggle between the $29 annual list rate and Intercom's $19 new-customer promo."
      />

      <div className="flex flex-col gap-6 rounded-2xl border border-border-button-default bg-background-full p-6 sm:p-8">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div className="flex min-w-[240px] flex-1 flex-col gap-2">
            <div className="flex items-baseline justify-between gap-2">
              <label htmlFor={sliderId} className="text-[14px] font-medium text-text-primary">
                Fin resolution rate
              </label>
              <span className="font-mono text-[15px] font-medium text-text-primary tabular-nums">
                {resolutionPct}% ({outcomes.toLocaleString("en-US")} outcomes/mo)
              </span>
            </div>
            <input
              id={sliderId}
              type="range"
              min={20}
              max={90}
              step={5}
              value={resolutionPct}
              onChange={(e) => setResolutionPct(Number(e.target.value))}
              className="w-full cursor-pointer accent-accent-600"
            />
          </div>

          <div className="inline-flex rounded-lg bg-background-secondary-default p-1 text-[13px]">
            <button
              type="button"
              aria-pressed={!promoSeat}
              onClick={() => setPromoSeat(false)}
              className={cx(
                "cursor-pointer rounded-md px-3 py-1.5 font-medium transition-colors",
                !promoSeat ? "bg-background-full text-text-primary shadow-xs" : "text-text-secondary",
              )}
            >
              $29 list seat
            </button>
            <button
              type="button"
              aria-pressed={promoSeat}
              onClick={() => setPromoSeat(true)}
              className={cx(
                "cursor-pointer rounded-md px-3 py-1.5 font-medium transition-colors",
                promoSeat ? "bg-background-full text-text-primary shadow-xs" : "text-text-secondary",
              )}
            >
              $19 promo seat
            </button>
          </div>
        </div>

        <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
          <div className="flex flex-col gap-1 rounded-xl border border-border-button-default bg-background-secondary-default/40 p-5">
            <span className="font-mono text-[11px] text-text-tertiary uppercase">Share of bill from Fin</span>
            <span className="font-mono text-[32px] font-medium text-text-primary tabular-nums">{finSharePct}%</span>
            <span className="text-[13px] text-text-secondary">
              {dollars(finSpend)} Fin + {dollars(seatSpend)} seats = {dollars(totalIntercom)}/mo
            </span>
          </div>
          <div className="flex flex-col gap-1 rounded-xl border border-accent-500/35 bg-accent-500/[0.04] p-5">
            <span className="font-mono text-[11px] text-accent-700 uppercase dark:text-accent-300">
              dimmi Cloud · AI included
            </span>
            <span className="font-mono text-[32px] font-medium text-text-primary tabular-nums">
              {dollars(dimmiCloud)}/mo
            </span>
            <span className="text-[13px] text-text-secondary">Same bill whether AI resolves 20% or 90%</span>
          </div>
          <div className="flex flex-col gap-1 rounded-xl border border-accent-500/35 bg-accent-500/[0.04] p-5">
            <span className="font-mono text-[11px] text-accent-700 uppercase dark:text-accent-300">
              dimmi Cloud · BYO key
            </span>
            <span className="font-mono text-[32px] font-medium text-text-primary tabular-nums">
              {dollars(dimmiByok)}/mo
            </span>
            <span className="text-[13px] text-text-secondary">
              Save {dollars(totalIntercom - dimmiByok)}/mo before raw tokens
            </span>
          </div>
        </div>

        <div className="rounded-xl border border-separator-border bg-background-secondary-default/35 p-4 text-[13px] leading-6 text-text-secondary">
          <strong className="font-medium text-text-primary">When dimmi Cloud costs more on paper:</strong> if 2
          teammates answer 2,000 conversations completely by hand with 0% AI, dimmi Cloud bills $60/mo while an entry
          seat plan elsewhere starts at $38/mo (without custom reports, round robin, or multiple SLAs). Self-hosting
          dimmi under MIT stays $0 to dimmi at every volume.
        </div>
      </div>
    </Section>
  );
}

function MigrateSection() {
  return (
    <Section frameClassName="flex flex-col gap-10">
      <SplitHead
        tag="Move in 4 commands"
        title="Bring your Intercom history across without retyping a single article."
        body="Run the open-source dimmi CLI on your own computer. Your customer data streams straight from Intercom to dimmi, stays out of AI chats, and never sends duplicate emails or notifications."
      />

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2 [&>*]:min-w-0">
        <div className="flex flex-col gap-3 rounded-2xl border border-border-button-default bg-background-full p-6">
          <div className="flex items-center gap-2 font-mono text-[12px] text-text-tertiary uppercase">
            <RiTerminalBoxLine className="size-4" aria-hidden />
            <span>dimmi CLI migration</span>
          </div>
          <div className="flex flex-col divide-y divide-separator-border font-mono text-[13px]">
            {MIGRATE_COMMANDS.map((step) => (
              <div key={step.cmd} className="flex flex-col gap-1 py-3 first:pt-1 last:pb-1">
                <code className="font-medium text-text-primary">{step.cmd}</code>
                <span className="font-sans text-[13px] text-text-secondary">{step.note}</span>
              </div>
            ))}
          </div>
          <div className="pt-2">
            <MonoLink href="/docs/migrate/intercom">Read the full Intercom migration guide</MonoLink>
          </div>
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div className="flex flex-col gap-3 rounded-2xl border border-accent-500/30 bg-accent-500/[0.03] p-5">
            <h3 className="text-[15px] font-medium text-text-primary">What comes across</h3>
            <ul className="flex flex-col gap-2 text-[13px] leading-5 text-text-secondary">
              {MOVES_ACROSS.map((item) => (
                <li key={item} className="flex items-start gap-2">
                  <RiCheckLine className="mt-0.5 size-4 shrink-0 text-accent-600 dark:text-accent-400" aria-hidden />
                  <span>{item}</span>
                </li>
              ))}
            </ul>
          </div>

          <div className="flex flex-col gap-3 rounded-2xl border border-border-button-default bg-background-full p-5">
            <h3 className="text-[15px] font-medium text-text-primary">What stays behind</h3>
            <ul className="flex flex-col gap-2 text-[13px] leading-5 text-text-secondary">
              {STAYS_BEHIND.map((item) => (
                <li key={item} className="flex items-start gap-2">
                  <RiArrowRightLine className="mt-0.5 size-4 shrink-0 text-text-tertiary" aria-hidden />
                  <span>{item}</span>
                </li>
              ))}
            </ul>
          </div>
        </div>
      </div>
    </Section>
  );
}

export function VsIntercomPage() {
  return (
    <div style={rivalToneStyle(intercom)}>
      <VsHero
        rival={intercom}
        eyebrow="dimmi vs Intercom"
        title="Free seats, your own AI key, and zero $0.99 outcome fees."
        verdict={
          <p>
            Pick Intercom if you need a mature platform with SOC 2, a large app marketplace, and you are fine paying per
            seat plus $0.99 per AI outcome. Pick dimmi if you want free seats, your own AI key, MIT code you can
            self-host, and a bill starting at $0 for {FREE_CONVERSATIONS} conversations every month.
          </p>
        }
        authorId="frantz-kati"
        reviewedById="frantz-kati"
        published={CHECKED_ON_ISO}
        updated={CHECKED_ON_ISO}
        visual={<FinOutcomeMeterVisual />}
      />

      <WinCards
        title="Where Intercom beats dimmi, and where dimmi takes the lead."
        body="We checked every Intercom plan on November 27, 2026 and named our own gaps plainly."
        rivalName="Intercom"
        rivalWins={intercom.rivalWins}
        dimmiWins={intercom.dimmiWins}
      />

      <FinResolutionSlider />

      <BillDuel
        rival={intercom}
        title="Run your own team numbers side by side."
        body="Intercom Essential lists at $29/seat/mo on annual billing ($19/seat/mo new-customer promo), Advanced is $85/seat for workflows and round robin, and Expert is $132/seat for SLAs and SSO. dimmi unlocks all of those with $0 seats."
        defaultSeats={5}
        defaultConversations={2000}
        defaultAiShare={0.8}
        note="See the full price book on /pricing."
      />

      <ProofBand
        tag="Real product screens"
        title="Plug in your own OpenAI or Anthropic key and track real response speed."
        body="Both captures come straight from a live dimmi workspace. No mockups, and no enterprise upgrade required to unlock your own model providers or reporting."
        caption="Left: Settings, AI providers with BYO keys billed at your model provider's API rates. Right: Workspace overview reports showing response times and first-contact resolution."
      >
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
          <ProductShot
            name="vs-byo-key"
            alt="dimmi AI providers settings page showing dimmi managed models connected and Add provider button for bringing your own API keys"
          />
          <ProductShot
            name="vs-reports"
            alt="dimmi workspace reports overview showing first response time, resolution time, first contact resolution rate, and conversation volume charts"
          />
        </div>
        <div className="flex flex-wrap gap-6 pt-3">
          <MonoLink href="/pricing">Explore dimmi pricing</MonoLink>
          <MonoLink href="/agents">See dimmi AI support agents</MonoLink>
          <MonoLink href="/procedures">See policy procedures</MonoLink>
          <MonoLink href="/inbox">See the shared inbox</MonoLink>
        </div>
      </ProofBand>

      <SwitchTriggers
        title="Four billing traps that push software teams off Intercom."
        body="Public pricing tiers verified on intercom.com/pricing on November 27, 2026."
        rivalName="Intercom"
        triggers={intercom.switchTriggers}
      />

      <VsMatrix
        title="Feature by feature, with every real gap marked."
        body="Items marked Request open our feedback board so you can track what ships next."
        rivalName="Intercom"
        groups={INTERCOM_MATRIX}
      />

      <TeamPicker
        rival={intercom}
        title="Four real team profiles, and who should pick what."
        body="We include a team that should pick Intercom today so you do not waste an afternoon testing the wrong tool."
        teams={intercom.teams}
      />

      <MigrateSection />

      <Questions items={INTERCOM_QUESTIONS} />

      <CtaBand
        title="Keep your history. Drop the seat tax and the $0.99 outcome meter."
        links={[
          { label: "Start free, 100 chats/mo", href: `${APP_URL}/signup`, variant: "primary", icon: "arrow" },
          { label: "Self-host on GitHub", href: GITHUB_URL, variant: "secondary", icon: "github" },
        ]}
      />
    </div>
  );
}
