/*
 * dimmi vs Tawk.to comparison page (`/vs/tawk-to`).
 * Unique elements: free-chat + paid-extras hero visual, interactive
 * "What free covers" checklist being 100% fair when Tawk.to Free is enough,
 * and real captures of BYO AI keys, help center, and changelog.
 */
import { useState } from "react";

import { cx } from "@/utils/cx";

import { MonoLink, Section, SplitHead } from "../../components/container";
import { Questions } from "../../components/product-page";
import { ProductShot } from "../../components/product-shot";
import { CHECKED_ON, CHECKED_ON_ISO, getRival } from "../../data/rivals";
import { APP_URL, GITHUB_URL } from "../../links";
import { FREE_CONVERSATIONS } from "../../pricing/prices";
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

const tawkto = getRival("tawk-to");

const TAWKTO_MATRIX: VsMatrixGroup[] = [
  {
    title: "Free tier and paid add-ons",
    rows: [
      {
        feature: "Unlimited human live chat agents",
        dimmi: { status: "yes", label: "$0 unlimited seats" },
        rival: { status: "yes", label: "$0 forever" },
      },
      {
        feature: "Remove vendor branding on widget",
        dimmi: { status: "yes", label: "Included on self-host/paid" },
        rival: { status: "partial", label: "$29/mo add-on" },
      },
      {
        feature: "AI agent using your own OpenAI/Anthropic key",
        dimmi: { status: "yes", label: "Built in (Cloud & MIT)" },
        rival: { status: "no", label: "AI Assist from $29/mo" },
      },
      {
        feature: "Optional outsourced $1/hr live chat agents",
        dimmi: { status: "no", label: "Software only" },
        rival: { status: "yes", label: "From $1/hour" },
      },
    ],
  },
  {
    title: "Product surface and developer stack",
    rows: [
      {
        feature: "Help center and knowledge base",
        dimmi: { status: "yes", label: "Multilingual docs" },
        rival: { status: "yes", label: "Included" },
      },
      {
        feature: "Product changelog and feedback board",
        dimmi: { status: "yes", label: "Built in" },
        rival: { status: "no", label: "Not included" },
      },
      {
        feature: "Policy procedures, workflows, and SLAs",
        dimmi: { status: "yes", label: "Every workspace" },
        rival: { status: "partial", label: "Basic triggers" },
      },
      {
        feature: "100% MIT open-source self-hosting",
        dimmi: { status: "yes", label: "One Linux server ($0)" },
        rival: { status: "no", label: "Closed cloud" },
      },
      {
        feature: "REST API, signed webhooks, 12 SDKs, and MCP",
        dimmi: { status: "yes", label: "Included" },
        rival: { status: "partial", label: "JS widget API" },
      },
    ],
  },
];

const TAWKTO_QUESTIONS = [
  {
    question: "Is Tawk.to actually free forever?",
    answer:
      "Yes. Tawk.to's core live chat, unlimited agents, basic ticketing, and knowledge base are $0 forever. Tawk.to makes money from optional paid add-ons: $29/month to remove 'Powered by tawk.to' branding, paid AI Assist tiers starting at $29/month, and hired live chat agents from $1/hour.",
  },
  {
    question: "When should we just stick with Tawk.to?",
    answer:
      "Stick with Tawk.to if you only need free human live chat on a website, do not mind Tawk.to branding, and do not need BYO AI model keys, a product changelog, workflows, or self-hosted data ownership.",
  },
  {
    question: "Why would a software team move from Tawk.to to dimmi?",
    answer:
      "Teams move when they want AI support agents running on their own OpenAI or Anthropic keys with approval-gated procedures, a unified help center plus product changelog and feedback board, 12 typed SDKs, or MIT open-source self-hosting inside their own Postgres database.",
  },
  {
    question: "How much does dimmi cost for a small team coming from Tawk.to?",
    answer:
      "On dimmi Cloud, your first 100 conversations every month are free with unlimited seats and every feature unlocked. If you want $0 software fees at any volume and no vendor branding, self-host dimmi under MIT on your own Linux server.",
  },
];

function TawktoExtrasHeroVisual() {
  const rows = [
    { item: "Core live chat + unlimited agents", price: "$0 forever", note: "Shows 'Powered by tawk.to' badge" },
    { item: "Remove branding add-on", price: "+$29/mo", note: "White-label widget & emails" },
    { item: "AI Assist (paid plans)", price: "+$29+/mo", note: "Closed models, no BYO API key" },
    { item: "Self-hosting & MIT source code", price: "Unavailable", note: "Closed cloud only" },
  ];

  return (
    <div className="flex flex-col gap-4 rounded-2xl border border-border-button-default bg-background-full p-6 shadow-xs">
      <div className="flex items-center justify-between border-b border-separator-border pb-3 font-mono text-[11px] text-text-tertiary uppercase">
        <span>Free core chat vs paid extras</span>
        <span>Checked {CHECKED_ON}</span>
      </div>

      <div className="flex flex-col gap-2">
        {rows.map((row) => (
          <div
            key={row.item}
            className="flex items-start justify-between gap-3 rounded-xl border border-separator-border bg-background-secondary-default/35 px-4 py-2.5"
          >
            <div className="flex flex-col">
              <span className="text-[14px] font-medium text-text-primary">{row.item}</span>
              <span className="text-[12px] text-text-secondary">{row.note}</span>
            </div>
            <span className="shrink-0 font-mono text-[13px] font-medium text-text-primary">{row.price}</span>
          </div>
        ))}
      </div>

      <div className="flex items-center justify-between rounded-xl border border-accent-500/40 bg-accent-500/[0.05] px-4 py-3">
        <span className="text-[13px] font-medium text-text-primary">
          dimmi MIT self-hosted: white-label + BYO keys + changelog
        </span>
        <span className="font-mono text-[15px] font-medium text-accent-700 dark:text-accent-300">$0 software</span>
      </div>
    </div>
  );
}

const CHECKLIST_ITEMS = [
  {
    id: "chat",
    label: "Free human live chat on my website",
    coveredByFree: true,
    note: "Included free on Tawk.to forever.",
  },
  {
    id: "branding",
    label: "Remove 'Powered by' branding",
    coveredByFree: false,
    note: "Costs $29/mo on Tawk.to; free to self-host on dimmi.",
  },
  {
    id: "byok",
    label: "AI support agents on my own OpenAI/Anthropic key",
    coveredByFree: false,
    note: "Not available on Tawk.to; built into dimmi.",
  },
  {
    id: "changelog",
    label: "Product changelog + feature request board",
    coveredByFree: false,
    note: "Not part of Tawk.to; native in dimmi.",
  },
  {
    id: "selfhost",
    label: "MIT open-source code inside my own Postgres",
    coveredByFree: false,
    note: "Closed SaaS on Tawk.to; MIT on dimmi.",
  },
] as const;

function WhatFreeCoversChecklist() {
  const [picked, setPicked] = useState<string[]>(["chat", "byok", "changelog"]);

  const toggle = (id: string) => {
    setPicked((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]));
  };

  const unmet = CHECKLIST_ITEMS.filter((item) => picked.includes(item.id) && !item.coveredByFree);
  const freeIsEnough = unmet.length === 0;

  return (
    <Section frameClassName="flex flex-col gap-10">
      <SplitHead
        tag="What free covers"
        title="Tick what your team actually needs to see if Tawk.to Free is enough."
        body="We will tell you honestly when Tawk.to Free is the right tool, and when you outgrow its free tier."
      />

      <div className="flex flex-col gap-6 rounded-2xl border border-border-button-default bg-background-full p-6 sm:p-8">
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          {CHECKLIST_ITEMS.map((item) => {
            const active = picked.includes(item.id);
            return (
              <label
                key={item.id}
                className={cx(
                  "flex cursor-pointer items-start gap-3 rounded-xl border p-4 transition-colors",
                  active ? "border-accent-500/45 bg-accent-500/[0.05]" : "border-border-button-default",
                )}
              >
                <input
                  type="checkbox"
                  checked={active}
                  onChange={() => toggle(item.id)}
                  className="mt-0.5 size-4 cursor-pointer accent-accent-600"
                />
                <span className="flex flex-col gap-0.5">
                  <span className="text-[14px] font-medium text-text-primary">{item.label}</span>
                  <span className="text-[12px] text-text-secondary">{item.note}</span>
                </span>
              </label>
            );
          })}
        </div>

        <div
          className={cx(
            "rounded-2xl border p-5 text-[15px] leading-6",
            freeIsEnough
              ? "border-border-button-default bg-background-secondary-default/50 text-text-primary"
              : "border-accent-500/35 bg-accent-500/[0.04] text-text-primary",
          )}
        >
          {freeIsEnough ? (
            <p>
              <strong>Verdict: Tawk.to Free covers everything you ticked.</strong> If free human live chat is all you
              need right now, save your time and use Tawk.to. Come back to dimmi when you want AI agents on your own
              keys, a changelog, or open-source self-hosting.
            </p>
          ) : (
            <p>
              <strong>Verdict: {unmet.length} requirement(s) sit outside Tawk.to Free.</strong> You ticked{" "}
              {unmet.map((u) => u.label.toLowerCase()).join(", ")}. dimmi ships all of those in one MIT codebase on
              Cloud ({FREE_CONVERSATIONS} free chats/mo) or self-hosted ($0 software).
            </p>
          )}
        </div>
      </div>
    </Section>
  );
}

export function VsTawkToPage() {
  return (
    <div style={rivalToneStyle(tawkto)}>
      <VsHero
        rival={tawkto}
        eyebrow="dimmi vs Tawk.to"
        title="Own your AI support agents, help center, and changelog under MIT."
        verdict={
          <p>
            Pick Tawk.to if you only need free human live chat for unlimited teammates and do not mind vendor branding
            on the widget. Choose dimmi when you want AI agents running on your own model keys, a help center and
            product changelog in one tool, developer SDKs, and MIT open-source code you can self-host.
          </p>
        }
        authorId="frantz-kati"
        reviewedById="frantz-kati"
        published={CHECKED_ON_ISO}
        updated={CHECKED_ON_ISO}
        primaryCta={{ label: "Try dimmi Cloud free", href: `${APP_URL}/signup` }}
        secondaryCta={{ label: "Deploy MIT self-hosted", href: GITHUB_URL, github: true }}
        visual={<TawktoExtrasHeroVisual />}
      />

      <WinCards
        tag="Tawk.to trade-offs"
        title="Where Tawk.to genuinely wins, and where dimmi goes further."
        body="Checked on tawk.to/pricing late November 2026."
        rivalName="Tawk.to"
        rivalWins={tawkto.rivalWins}
        dimmiWins={tawkto.dimmiWins}
      />

      <WhatFreeCoversChecklist />

      <ProofBand
        tag="All in one workspace"
        title="BYO AI model keys, multilingual help center, and product changelog together."
        body="Instead of bolting branding add-ons, closed AI tiers, and a third-party changelog onto a chat widget, dimmi ships all three in one open-source workspace."
        caption="Top left: dimmi AI providers settings for connecting your own API keys. Top right: built-in multilingual help center. Bottom: built-in product changelog workspace."
      >
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
          <ProductShot
            name="vs-byo-key"
            alt="dimmi AI providers settings page showing managed models and Add provider controls for your own API keys"
          />
          <ProductShot
            name="help-center"
            alt="dimmi multilingual help center with structured collections and articles"
          />
        </div>
        <div className="pt-2">
          <ProductShot
            name="vs-changelog"
            alt="dimmi product changelog workspace with labels, drafts, scheduled updates, and subscriber controls"
          />
        </div>
        <div className="flex flex-wrap gap-6 pt-3">
          <MonoLink href="/vs">Browse all comparisons</MonoLink>
          <MonoLink href="/vs/crisp">Read Crisp breakdown</MonoLink>
          <MonoLink href="/vs/chatwoot">Read Chatwoot breakdown</MonoLink>
          <MonoLink href="/pricing">Check dimmi Cloud rates</MonoLink>
        </div>
      </ProofBand>

      <SwitchTriggers
        tag="Tawk.to switch triggers"
        title="Why software teams graduate from Tawk.to to dimmi."
        body="Verified directly on tawk.to/pricing."
        rivalName="Tawk.to"
        triggers={tawkto.switchTriggers}
      />

      <VsMatrix
        title="Free chat vs full open-source support platform."
        body="Clear view of where Tawk.to Free is plenty and where dimmi replaces 3 tools."
        rivalName="Tawk.to"
        groups={TAWKTO_MATRIX}
      />

      <TeamPicker
        rival={tawkto}
        tag="Tawk.to team fit"
        title="Four teams and which option makes the most sense."
        body="Starts with the local business that should pick Tawk.to Free without hesitation."
        teams={tawkto.teams}
        rivalMonthlyForTeam={(team) => {
          if (team.rivalPlanLabel.includes("$0")) {
            return 0;
          }
          if (team.rivalPlanLabel.includes("$58")) {
            return 58;
          }
          return 58;
        }}
      />

      <Questions items={TAWKTO_QUESTIONS} />

      <CtaBand
        title="Keep your support stack open, white-labeled, and on your own keys."
        links={[
          { label: "Start free workspace", href: `${APP_URL}/signup`, variant: "primary", icon: "arrow" },
          { label: "Clone MIT repository", href: GITHUB_URL, variant: "secondary", icon: "github" },
        ]}
      />
    </div>
  );
}
