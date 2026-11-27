/*
 * dimmi vs Front comparison page (`/vs/front`).
 * Unique elements: 1-channel-type Starter limit hero visual, interactive
 * channel picker showing forced Front tier jumps, and real dimmi email +
 * shared inbox captures.
 */
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

const front = getRival("front");

const FRONT_MATRIX: VsMatrixGroup[] = [
  {
    title: "Channels and tier limits",
    rows: [
      {
        feature: "Email + web live chat on entry tier",
        dimmi: { status: "yes", label: "Included ($0 seats)" },
        rival: { status: "no", label: "Forces Pro ($65/seat)" },
      },
      {
        feature: "WhatsApp channel fee",
        dimmi: { status: "yes", label: "0% markup from dimmi" },
        rival: { status: "no", label: "Meta + 20% admin fee" },
      },
      {
        feature: "Automation rules limit",
        dimmi: { status: "yes", label: "Unlimited workflows" },
        rival: { status: "partial", label: "10 Starter / 20 Pro" },
      },
      {
        feature: "Custom reports and multilingual docs",
        dimmi: { status: "yes", label: "Every workspace" },
        rival: { status: "partial", label: "Enterprise ($105/seat)" },
      },
    ],
  },
  {
    title: "AI, API, and ownership",
    rows: [
      {
        feature: "AI agent pricing",
        dimmi: { status: "yes", label: "From $0.10/chat or BYO key" },
        rival: { status: "no", label: "From $0.39/chat Autopilot" },
      },
      {
        feature: "Teammate AI copilot",
        dimmi: { status: "yes", label: "Operator included" },
        rival: { status: "partial", label: "$20/seat on Starter/Pro" },
      },
      {
        feature: "Open-source self-hosting",
        dimmi: { status: "yes", label: "100% MIT ($0)" },
        rival: { status: "no", label: "Closed SaaS" },
      },
      {
        feature: "API rate limit increases",
        dimmi: { status: "yes", label: "Included / self-host" },
        rival: { status: "partial", label: "$200 per 100 req/min" },
      },
    ],
  },
  {
    title: "Email ops and compliance",
    rows: [
      {
        feature: "Shared email client for sales and ops teams",
        dimmi: { status: "partial", label: "Support-focused inbox" },
        rival: { status: "yes", label: "Core strength" },
      },
      {
        feature: "SOC 2 Type II certification",
        requestTitle: "SOC 2 Type II report",
        dimmi: { status: "no" },
        rival: { status: "yes", label: "Certified" },
      },
      {
        feature: "Deep Salesforce and HubSpot CRM sync",
        dimmi: { status: "partial", label: "API and webhooks" },
        rival: { status: "yes", label: "Built in" },
      },
    ],
  },
];

const FRONT_QUESTIONS = [
  {
    question: "Why does adding live chat beside email double a Front bill?",
    answer:
      "Front Starter ($25/seat/mo annual, up to 10 seats) only allows a single channel type. The moment you run web live chat or WhatsApp beside shared email, every seat must upgrade to Front Professional at $65/seat/mo.",
  },
  {
    question: "Does dimmi charge a WhatsApp markup like Front?",
    answer:
      "No. Front adds a 20% admin fee on top of Meta WhatsApp charges. On dimmi, you connect your Meta WhatsApp Business account directly and pay Meta's standard rates with 0% markup from dimmi.",
  },
  {
    question: "When should a team choose Front instead of dimmi?",
    answer:
      "Choose Front if your main workflow is internal email collaboration for logistics, account management, or sales teams, or if you need Front's native CRM integrations and SOC 2 compliance today.",
  },
  {
    question: "How does Front Autopilot compare with dimmi's AI support agents?",
    answer:
      "Front Autopilot starts at $0.39 per conversation on top of seat fees and does not accept your own model keys. dimmi includes 100 free conversations monthly, charges from $0.10 per AI chat on Cloud, and lets you plug in your own OpenAI or Anthropic key.",
  },
  {
    question: "What happens if our engineering team hits Front's API rate limits?",
    answer:
      "Front lists an API Rate Limit Increase add-on at $200/month per 100 requests per minute. dimmi includes its REST API, signed webhooks, 12 typed SDKs, and MCP server on every workspace, and self-hosted MIT deployments let you set your own throughput limits on your own server.",
  },
  {
    question: "Can we route multiple customer domains and signatures in dimmi?",
    answer:
      "Yes. Verify each sending domain with DKIM and SPF, route addresses such as support@, billing@, and security@ into separate shared inboxes, and assign per-inbox or per-teammate signatures without paying for extra workspaces.",
  },
];

function ChannelLimitHeroVisual() {
  const channels = [
    { name: "Shared Email", starter: true },
    { name: "Web Live Chat", starter: false },
    { name: "WhatsApp", starter: false },
    { name: "Slack", starter: false },
    { name: "Instagram & Messenger", starter: false },
  ];

  return (
    <div className="flex flex-col gap-4 rounded-2xl border border-border-button-default bg-background-full p-6 shadow-xs">
      <div className="flex items-center justify-between border-b border-separator-border pb-3 font-mono text-[11px] text-text-tertiary uppercase">
        <span>Channel gate on Front Starter ($25/seat)</span>
        <span>Checked {CHECKED_ON}</span>
      </div>

      <ul className="flex flex-col gap-2">
        {channels.map((ch) => (
          <li
            key={ch.name}
            className="flex items-center justify-between rounded-xl border border-separator-border bg-background-secondary-default/35 px-4 py-2.5 text-[13px]"
          >
            <span className="font-medium text-text-primary">{ch.name}</span>
            <span className="font-mono text-[11px] text-text-secondary">
              {ch.starter ? "1 channel type on Starter" : "Requires Pro ($65/seat)"}
            </span>
          </li>
        ))}
      </ul>

      <div className="flex items-center justify-between rounded-xl border border-accent-500/40 bg-accent-500/[0.05] px-4 py-3">
        <span className="text-[13px] font-medium text-text-primary">dimmi: all 5 channels unlocked on every plan</span>
        <span className="font-mono text-[15px] font-medium text-accent-700 dark:text-accent-300">$0/seat</span>
      </div>
    </div>
  );
}

function FrontChannelPicker() {
  const seatsId = useId();
  const [seats, setSeats] = useState(6);
  const [selected, setSelected] = useState<string[]>(["Email", "Web live chat"]);
  const [customReports, setCustomReports] = useState(false);

  const toggleChannel = (ch: string) => {
    setSelected((prev) =>
      prev.includes(ch) ? (prev.length > 1 ? prev.filter((x) => x !== ch) : prev) : [...prev, ch],
    );
  };

  const tier = customReports ? "Enterprise" : selected.length > 1 || seats > 10 ? "Professional" : "Starter";
  const seatRate = tier === "Enterprise" ? 105 : tier === "Professional" ? 65 : 25;
  const frontMonthly = seats * seatRate + Math.round(2000 * 0.7 * 0.39);
  const dimmiCloud = Math.round(bill({ conversations: 2000, aiShare: 0.7, operator: 0, ownModel: false }).total);
  const dimmiByok = Math.round(bill({ conversations: 2000, aiShare: 0.7, operator: 0, ownModel: true }).total);

  const channelOptions = ["Email", "Web live chat", "WhatsApp", "Instagram", "Slack"];

  return (
    <Section frameClassName="flex flex-col gap-10">
      <SplitHead
        tag="Channel picker"
        title="Pick your channels and see which Front plan you are forced onto."
        body="Adding a second channel type moves every teammate from Starter ($25/seat) to Professional ($65/seat). Models 2,000 monthly conversations with 70% AI Autopilot."
      />

      <div className="flex flex-col gap-6 rounded-2xl border border-border-button-default bg-background-full p-6 sm:p-8">
        <div className="flex flex-col gap-2">
          <div className="flex items-baseline justify-between">
            <label htmlFor={seatsId} className="text-[14px] font-medium text-text-primary">
              Teammates
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

        <div className="flex flex-wrap gap-2">
          {channelOptions.map((ch) => {
            const active = selected.includes(ch);
            return (
              <button
                key={ch}
                type="button"
                aria-pressed={active}
                onClick={() => toggleChannel(ch)}
                className={cx(
                  "cursor-pointer rounded-xl border px-4 py-2 text-[14px] font-medium transition-colors",
                  active
                    ? "border-accent-500/50 bg-accent-500/[0.08] text-text-primary"
                    : "border-border-button-default text-text-secondary",
                )}
              >
                {ch}
              </button>
            );
          })}
          <button
            type="button"
            aria-pressed={customReports}
            onClick={() => setCustomReports((v) => !v)}
            className={cx(
              "cursor-pointer rounded-xl border px-4 py-2 text-[14px] font-medium transition-colors",
              customReports
                ? "border-accent-500/50 bg-accent-500/[0.08] text-text-primary"
                : "border-border-button-default text-text-secondary",
            )}
          >
            + Custom reports / multilingual docs
          </button>
        </div>

        <div className="grid grid-cols-1 gap-4 border-t border-separator-border pt-5 md:grid-cols-3">
          <div className="flex flex-col gap-1 rounded-xl border border-border-button-default bg-background-secondary-default/40 p-5">
            <span className="font-mono text-[11px] text-text-tertiary uppercase">Required: Front {tier}</span>
            <span className="font-mono text-[30px] font-medium text-text-primary tabular-nums">
              {dollars(frontMonthly)}/mo
            </span>
            <span className="text-[12px] text-text-secondary">
              {`${seats} x $${seatRate}/seat + Autopilot ($0.39/chat)`}
            </span>
          </div>

          <div className="flex flex-col gap-1 rounded-xl border border-accent-500/35 bg-accent-500/[0.04] p-5">
            <span className="font-mono text-[11px] text-accent-700 uppercase dark:text-accent-300">Managed Cloud</span>
            <span className="font-mono text-[30px] font-medium text-text-primary tabular-nums">
              {dollars(dimmiCloud)}/mo
            </span>
            <span className="text-[12px] text-text-secondary">All channels included, $0 seats, 0% WhatsApp fee</span>
          </div>

          <div className="flex flex-col gap-1 rounded-xl border border-accent-500/35 bg-accent-500/[0.04] p-5">
            <span className="font-mono text-[11px] text-accent-700 uppercase dark:text-accent-300">
              Own model provider
            </span>
            <span className="font-mono text-[30px] font-medium text-text-primary tabular-nums">
              {dollars(dimmiByok)}/mo
            </span>
            <span className="text-[12px] text-text-secondary">Self-host the MIT binary for zero license cost</span>
          </div>
        </div>
      </div>
    </Section>
  );
}

export function VsFrontPage() {
  return (
    <div style={rivalToneStyle(front)}>
      <VsHero
        rival={front}
        eyebrow="dimmi vs Front"
        title="Every support channel unlocked from day one, with $0 seats."
        verdict={
          <p>
            Pick Front if you need a mature shared email workspace across sales, dispatch, and account teams with native
            CRM sync and SOC 2 compliance. Switch to dimmi when shared email, live chat, Slack, WhatsApp, and Instagram
            belong inside one inbox with zero seat charges, direct model keys, and {FREE_CONVERSATIONS} free monthly
            threads on MIT software.
          </p>
        }
        authorId="frantz-kati"
        reviewedById="frantz-kati"
        published={CHECKED_ON_ISO}
        updated={CHECKED_ON_ISO}
        primaryCta={{ label: "Connect channels free", href: `${APP_URL}/signup` }}
        secondaryCta={{ label: "Clone MIT codebase", href: GITHUB_URL, github: true }}
        visual={<ChannelLimitHeroVisual />}
      />

      <WinCards
        tag="Front trade-offs"
        title="Where Front excels, and where dimmi unlocks multichannel support."
        body="Audited from Front pricing tables late Nov 2026."
        rivalName="Front"
        rivalWins={front.rivalWins}
        dimmiWins={front.dimmiWins}
      />

      <FrontChannelPicker />

      <BillDuel
        rival={front}
        tag="Channel cost duel"
        title="Compare Front Starter and Enterprise against dimmi."
        body="Front Starter bills $25/seat annual for one channel type (up to 10 seats), Professional bills $65/seat for omnichannel, and Enterprise bills $105/seat for custom reports."
        defaultSeats={6}
        defaultConversations={2000}
        defaultAiShare={0.7}
      />

      <ProofBand
        tag="Email and live chat together"
        title="Email threads and live chat side by side in one shared inbox."
        body="No channel-type upgrade required. Answer customer emails with full DKIM/SPF domains right beside live web chat and Slack threads."
        caption="Left: dimmi email conversation thread inside the inbox. Right: full dimmi shared inbox across channels."
      >
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
          <ProductShot
            name="channel-email-thread"
            alt="dimmi email support thread showing customer message, internal notes, and rich reply composer"
          />
          <ProductShot
            name="inbox"
            alt="dimmi omnichannel shared inbox combining live chat, email, and customer details sidebar"
          />
        </div>
        <div className="flex flex-wrap gap-6 pt-3">
          <MonoLink href="/vs">Compare other tools</MonoLink>
          <MonoLink href="/vs/freshdesk">Read Freshdesk breakdown</MonoLink>
          <MonoLink href="/channels/email">Explore dimmi Email</MonoLink>
          <MonoLink href="/pricing">View full pricing</MonoLink>
        </div>
      </ProofBand>

      <SwitchTriggers
        tag="Front triggers"
        title="Why multichannel support teams move off Front."
        body="Sourced directly from Front Starter, Pro, and Enterprise tiers."
        rivalName="Front"
        triggers={front.switchTriggers}
      />

      <VsMatrix
        title="Channels, automation rules, and compliance side by side."
        body="Includes where Front's email-ops focus and SOC 2 compliance remain stronger today."
        rivalName="Front"
        groups={FRONT_MATRIX}
      />

      <TeamPicker
        rival={front}
        tag="Front team fit"
        title="Four real teams and which workspace fits each."
        body="Includes the email-first B2B logistics team that should pick Front."
        teams={front.teams}
      />

      <Questions items={FRONT_QUESTIONS} />

      <CtaBand
        title="Run email, chat, Slack, and WhatsApp without $65 seats."
        links={[
          { label: "Start omnichannel free", href: `${APP_URL}/signup`, variant: "primary", icon: "arrow" },
          { label: "View source on GitHub", href: GITHUB_URL, variant: "secondary", icon: "github" },
        ]}
      />
    </div>
  );
}
