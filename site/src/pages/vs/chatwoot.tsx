/*
 * dimmi vs Chatwoot comparison page (`/vs/chatwoot`).
 * Built on the shared `/vs` kit with Chatwoot-specific stack diagram hero,
 * word-for-word MIT vs enterprise/LICENSE breakdown, timed TerminalScene +
 * SAML SSO & MCP server proof, and interactive self-host cost builder.
 */
import { RiCheckLine, RiExternalLinkLine, RiServerLine, RiTerminalBoxLine } from "@remixicon/react";
import { useId, useState } from "react";

import { cx } from "@/utils/cx";

import { MonoLink, Section, SplitHead } from "../../components/container";
import { Questions } from "../../components/product-page";
import { ProductShot } from "../../components/product-shot";
import { CHECKED_ON, CHECKED_ON_ISO, getRival } from "../../data/rivals";
import { APP_URL, GITHUB_URL } from "../../links";
import { bill, dollars } from "../../pricing/prices";
import { SERVERS } from "../../pricing/servers";
import { TerminalScene } from "../../scenes/terminal-scene";
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

const chatwoot = getRival("chatwoot");

const CHATWOOT_MATRIX: VsMatrixGroup[] = [
  {
    title: "License and self-hosting",
    rows: [
      {
        feature: "License across entire repository",
        dimmi: { status: "yes", label: "100% MIT" },
        rival: { status: "partial", label: "MIT + enterprise/" },
      },
      {
        feature: "Self-hosted SAML SSO and audit logs",
        dimmi: { status: "yes", label: "MIT ($0)" },
        rival: { status: "no", label: "Enterprise ($99/agent)" },
      },
      {
        feature: "Runtime containers to operate",
        dimmi: { status: "yes", label: "Go binary + PG + Valkey" },
        rival: { status: "partial", label: "Rails + Sidekiq + PG + Redis" },
      },
      {
        feature: "One-line Linux installer with auto TLS",
        dimmi: { status: "yes", label: "install.sh + dimmictl" },
        rival: { status: "yes", label: "cwctl Linux script" },
      },
    ],
  },
  {
    title: "AI agents, MCP, and automation",
    rows: [
      {
        feature: "Bring your own OpenAI or Anthropic key",
        dimmi: { status: "yes", label: "Cloud & self-hosted" },
        rival: { status: "partial", label: "Self-hosted OpenAI key" },
      },
      {
        feature: "Cloud AI credit limits",
        dimmi: { status: "yes", label: "Per-chat, no seat fee" },
        rival: { status: "partial", label: "300-800 Captain credits" },
      },
      {
        feature: "Policy procedures with simulations and diffs",
        dimmi: { status: "yes", label: "Included" },
        rival: { status: "no", label: "Not included" },
      },
      {
        feature: "Built-in MCP server for coding agents",
        dimmi: { status: "yes", label: "Included (MIT)" },
        rival: { status: "no", label: "REST API only" },
      },
    ],
  },
  {
    title: "Channels, mobile apps, and maturity",
    rows: [
      {
        feature: "Web chat, email, Slack, WhatsApp, IG, Messenger",
        dimmi: { status: "yes", label: "Included" },
        rival: { status: "yes", label: "Included" },
      },
      {
        feature: "Telegram and LINE channels",
        requestTitle: "Telegram and LINE inbox channels",
        dimmi: { status: "no" },
        rival: { status: "yes", label: "Built in" },
      },
      {
        feature: "Open-source iOS and Android teammate apps",
        dimmi: { status: "partial", label: "Web & Cloud apps" },
        rival: { status: "yes", label: "Open-source mobile apps" },
      },
      {
        feature: "Community size and years in production",
        dimmi: { status: "partial", label: "Launched Nov 2026" },
        rival: { status: "yes", label: "~40.7k GitHub stars" },
      },
    ],
  },
];

const CHATWOOT_QUESTIONS = [
  {
    question: "What is the exact license difference between dimmi and Chatwoot?",
    answer:
      "Chatwoot's LICENSE states that code outside the enterprise/ directory is MIT Expat, while everything inside enterprise/ is governed by the Chatwoot Enterprise License, which requires a paid Enterprise subscription for the right number of user seats in production. dimmi ships its entire codebase, including SAML SSO, audit logs, custom roles, SLAs, and AI procedures, under MIT with no enterprise directory.",
  },
  {
    question: "How does self-hosting a Go binary compare to Rails and Sidekiq?",
    answer:
      "Chatwoot's production compose stack runs a Ruby on Rails web server, a separate Sidekiq worker container, PostgreSQL with pgvector, and Redis. dimmi compiles web, background worker, and scheduler into one Go binary beside PostgreSQL (pgvector), Valkey, and Caddy, keeping memory usage predictable on a $24 to $48 Linux server.",
  },
  {
    question: "How do Chatwoot Captain credits compare to dimmi BYO model keys?",
    answer:
      "On Chatwoot Cloud, Startups ($19/agent/mo) includes 300 Captain AI credits, Business ($39/agent/mo) includes 500, and Enterprise ($99/agent/mo) includes 800, with extra credits at $20 per 1,000. On dimmi, you plug in your own OpenAI, Anthropic, DeepSeek, Mistral, or OpenRouter key and pay your model provider directly ($0 to dimmi self-hosted, or human conversation rate on Cloud).",
  },
  {
    question: "When should a team pick Chatwoot over dimmi?",
    answer:
      "Pick Chatwoot if you rely on Telegram or LINE channels today, want battle-tested community Helm charts and years of production deployments, or want an open-source Ruby stack your engineering team already maintains.",
  },
];

function StackCompareHeroVisual() {
  const dimmiContainers = [
    { name: "dimmi (single Go binary)", detail: "Web, background worker, scheduler, MCP" },
    { name: "PostgreSQL + pgvector", detail: "Conversations, tickets, vector search" },
    { name: "Valkey + Caddy", detail: "Realtime pub/sub and automatic HTTPS" },
  ];
  const chatwootContainers = [
    { name: "rails (bundle exec rails s)", detail: "Ruby on Rails web and API process" },
    { name: "sidekiq (bundle exec sidekiq)", detail: "Separate Ruby background job worker" },
    { name: "PostgreSQL + Redis", detail: "pgvector database and job/cache broker" },
  ];

  return (
    <div className="flex flex-col gap-4 rounded-2xl border border-border-button-default bg-background-full p-6 shadow-xs">
      <div className="flex items-center justify-between gap-2 border-b border-separator-border pb-3">
        <span className="font-mono text-[11px] font-medium tracking-[0.08em] text-text-tertiary uppercase">
          Production compose stacks
        </span>
        <span className="font-mono text-[11px] text-text-secondary">Checked {CHECKED_ON}</span>
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <div className="flex flex-col gap-2.5 rounded-xl border border-accent-500/35 bg-accent-500/[0.04] p-4">
          <div className="flex items-center justify-between gap-2">
            <span className="text-[14px] font-medium text-text-primary">dimmi</span>
            <span className="rounded-full bg-accent-500/15 px-2 py-0.5 font-mono text-[10px] text-accent-700 uppercase dark:text-accent-300">
              100% MIT
            </span>
          </div>
          <ul className="flex flex-col gap-2">
            {dimmiContainers.map((c) => (
              <li key={c.name} className="rounded-lg border border-separator-border bg-background-full px-3 py-2">
                <div className="font-mono text-[12px] font-medium text-text-primary">{c.name}</div>
                <div className="text-[11px] text-text-tertiary">{c.detail}</div>
              </li>
            ))}
          </ul>
        </div>

        <div className="flex flex-col gap-2.5 rounded-xl border border-border-button-default bg-background-secondary-default/35 p-4">
          <div className="flex items-center justify-between gap-2">
            <span className="text-[14px] font-medium text-text-primary">Chatwoot</span>
            <span className="rounded-full bg-background-secondary-default px-2 py-0.5 font-mono text-[10px] text-text-secondary uppercase">
              MIT + enterprise/
            </span>
          </div>
          <ul className="flex flex-col gap-2">
            {chatwootContainers.map((c) => (
              <li key={c.name} className="rounded-lg border border-separator-border bg-background-full px-3 py-2">
                <div className="font-mono text-[12px] font-medium text-text-primary">{c.name}</div>
                <div className="text-[11px] text-text-tertiary">{c.detail}</div>
              </li>
            ))}
          </ul>
        </div>
      </div>

      <p className="text-[12px] leading-5 text-text-tertiary">
        Read directly from{" "}
        <a
          href="https://github.com/dimmihq/dimmi/blob/main/deploy/selfhost/compose.yaml"
          target="_blank"
          rel="noopener noreferrer"
          className="underline underline-offset-4 hover:text-text-primary"
        >
          deploy/selfhost/compose.yaml
        </a>{" "}
        and{" "}
        <a
          href="https://github.com/chatwoot/chatwoot/blob/develop/docker-compose.production.yaml"
          target="_blank"
          rel="noopener noreferrer"
          className="underline underline-offset-4 hover:text-text-primary"
        >
          chatwoot/docker-compose.production.yaml
        </a>
        .
      </p>
    </div>
  );
}

const SERVER_MONTHLY_USD = [24, 48, 96] as const;

function SelfHostCostBuilder() {
  const seatsId = useId();
  const aiChatsId = useId();
  const [serverIdx, setServerIdx] = useState(1);
  const [seats, setSeats] = useState(5);
  const [aiChats, setAiChats] = useState(1500);
  const [needSso, setNeedSso] = useState(false);

  const serverCost = SERVER_MONTHLY_USD[serverIdx];
  // Rough raw API estimate (~$0.01 per AI chat on small/mid models via BYO key).
  const rawApiEstimate = Math.round(aiChats * 0.01);
  const dimmiSelfHostTotal = serverCost + rawApiEstimate;

  const chatwootCloudSeat = needSso ? 99 : 39;
  const chatwootIncludedCredits = needSso ? 800 : 500;
  const extraCredits = Math.max(0, aiChats - chatwootIncludedCredits);
  const extraCreditCost = Math.ceil(extraCredits / 1000) * 20;
  const chatwootCloudTotal = seats * chatwootCloudSeat + extraCreditCost;

  const chatwootSelfHostTotal = needSso ? serverCost + seats * 99 + rawApiEstimate : serverCost + rawApiEstimate;
  const dimmiCloudByok = Math.round(
    bill({ conversations: Math.round(aiChats / 0.75), aiShare: 0.75, operator: 0, ownModel: true }).total,
  );

  return (
    <Section frameClassName="flex flex-col gap-10">
      <SplitHead
        tag="Self-host cost builder"
        title="Compare server size, BYO keys, and Captain AI credits."
        body="Pick your VPS size, team size, and monthly AI chats. Toggle SAML SSO and audit logs to see where Chatwoot's enterprise/LICENSE kicks in."
      />

      <div className="flex flex-col gap-6 rounded-2xl border border-border-button-default bg-background-full p-6 sm:p-8">
        <div className="grid grid-cols-1 gap-5 md:grid-cols-3">
          <div className="flex flex-col gap-2">
            <span className="text-[13px] font-medium text-text-secondary">Linux server size</span>
            <div className="grid grid-cols-3 gap-1.5">
              {SERVERS.map((s, idx) => (
                <button
                  key={s.name}
                  type="button"
                  aria-pressed={serverIdx === idx}
                  onClick={() => setServerIdx(idx)}
                  className={cx(
                    "cursor-pointer rounded-lg border px-2 py-2 text-center font-mono text-[12px] transition-colors",
                    serverIdx === idx
                      ? "border-accent-500/50 bg-accent-500/[0.08] font-medium text-text-primary"
                      : "border-border-button-default text-text-secondary",
                  )}
                >
                  {s.price}
                </button>
              ))}
            </div>
            <span className="text-[12px] text-text-tertiary">{SERVERS[serverIdx].where}</span>
          </div>

          <div className="flex flex-col gap-2">
            <div className="flex items-baseline justify-between">
              <label htmlFor={seatsId} className="text-[13px] font-medium text-text-secondary">
                Teammates
              </label>
              <span className="font-mono text-[14px] font-medium text-text-primary tabular-nums">{seats} seats</span>
            </div>
            <input
              id={seatsId}
              type="range"
              min={1}
              max={25}
              step={1}
              value={seats}
              onChange={(e) => setSeats(Number(e.target.value))}
              className="w-full cursor-pointer accent-accent-600"
            />
          </div>

          <div className="flex flex-col gap-2">
            <div className="flex items-baseline justify-between">
              <label htmlFor={aiChatsId} className="text-[13px] font-medium text-text-secondary">
                AI-handled chats / mo
              </label>
              <span className="font-mono text-[14px] font-medium text-text-primary tabular-nums">
                {aiChats.toLocaleString("en-US")}
              </span>
            </div>
            <input
              id={aiChatsId}
              type="range"
              min={200}
              max={6000}
              step={200}
              value={aiChats}
              onChange={(e) => setAiChats(Number(e.target.value))}
              className="w-full cursor-pointer accent-accent-600"
            />
          </div>
        </div>

        <div className="flex flex-wrap items-center justify-between gap-3 border-t border-separator-border pt-4">
          <label className="inline-flex cursor-pointer items-center gap-2.5 text-[14px] text-text-primary">
            <input
              type="checkbox"
              checked={needSso}
              onChange={() => setNeedSso((prev) => !prev)}
              className="size-4 cursor-pointer accent-accent-600"
            />
            <span>Require SAML SSO, audit logs, and custom roles</span>
          </label>
          <span className="font-mono text-[12px] text-text-tertiary">
            Estimated raw model API (~$0.01/chat): {dollars(rawApiEstimate)}/mo
          </span>
        </div>

        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-4">
          <div className="flex flex-col gap-1.5 rounded-xl border border-accent-500/40 bg-accent-500/[0.04] p-5">
            <span className="font-mono text-[11px] text-accent-700 uppercase dark:text-accent-300">
              dimmi self-hosted (MIT)
            </span>
            <span className="font-mono text-[28px] font-medium text-text-primary tabular-nums">
              {dollars(dimmiSelfHostTotal)}/mo
            </span>
            <span className="text-[12px] text-text-secondary">
              {`$${serverCost} VPS + ~$${rawApiEstimate} API + $0 to dimmi (SSO included)`}
            </span>
          </div>

          <div className="flex flex-col gap-1.5 rounded-xl border border-accent-500/30 bg-accent-500/[0.02] p-5">
            <span className="font-mono text-[11px] text-accent-700 uppercase dark:text-accent-300">
              dimmi Cloud · BYO key
            </span>
            <span className="font-mono text-[28px] font-medium text-text-primary tabular-nums">
              {dollars(dimmiCloudByok + rawApiEstimate)}/mo
            </span>
            <span className="text-[12px] text-text-secondary">
              {`${dollars(dimmiCloudByok)} Cloud + ~$${rawApiEstimate} API, unlimited seats`}
            </span>
          </div>

          <div className="flex flex-col gap-1.5 rounded-xl border border-border-button-default bg-background-full p-5">
            <span className="font-mono text-[11px] text-text-tertiary uppercase">
              Chatwoot self-hosted {needSso ? "(Enterprise)" : "(Community)"}
            </span>
            <span className="font-mono text-[28px] font-medium text-text-primary tabular-nums">
              {dollars(chatwootSelfHostTotal)}/mo
            </span>
            <span className="text-[12px] text-text-secondary">
              {needSso
                ? `$${serverCost} VPS + ${seats} x $99 Enterprise license + API`
                : `$${serverCost} VPS + ~$${rawApiEstimate} API (no SSO or audit logs)`}
            </span>
          </div>

          <div className="flex flex-col gap-1.5 rounded-xl border border-border-button-default bg-background-full p-5">
            <span className="font-mono text-[11px] text-text-tertiary uppercase">
              Chatwoot Cloud ({needSso ? "Enterprise" : "Business"})
            </span>
            <span className="font-mono text-[28px] font-medium text-text-primary tabular-nums">
              {dollars(chatwootCloudTotal)}/mo
            </span>
            <span className="text-[12px] text-text-secondary">
              {`${seats} x $${chatwootCloudSeat}/agent + ${dollars(extraCreditCost)} extra Captain credits`}
            </span>
          </div>
        </div>
      </div>
    </Section>
  );
}

function LicenseSection() {
  return (
    <Section frameClassName="flex flex-col gap-8">
      <SplitHead
        tag="Read the LICENSE files"
        title="One MIT file vs MIT plus a commercial enterprise folder."
        body="Both projects live on GitHub, but their production license terms split as soon as your security team asks for SAML SSO or audit logs."
      />

      <div className="grid grid-cols-1 gap-5 md:grid-cols-2">
        <div className="flex flex-col gap-3 rounded-2xl border border-accent-500/35 bg-accent-500/[0.03] p-6">
          <div className="flex items-center justify-between gap-2">
            <h3 className="text-[18px] font-medium text-text-primary">dimmihq/dimmi LICENSE</h3>
            <span className="font-mono text-[11px] text-accent-700 uppercase dark:text-accent-300">
              Single MIT file
            </span>
          </div>
          <p className="text-[14px] leading-6 text-text-secondary">
            Every directory in the repository is licensed under MIT. Self-hosted servers run the exact same binary as
            dimmi Cloud with SAML SSO, audit logs, custom roles, SLAs, workflows, procedures, and Operator unlocked at
            $0 to dimmi.
          </p>
          <a
            href="https://github.com/dimmihq/dimmi/blob/main/LICENSE"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1 self-start font-mono text-[12px] text-text-primary underline-offset-4 hover:underline"
          >
            Read dimmi LICENSE
            <RiExternalLinkLine className="size-3.5" aria-hidden />
          </a>
        </div>

        <div className="flex flex-col gap-3 rounded-2xl border border-border-button-default bg-background-full p-6">
          <div className="flex items-center justify-between gap-2">
            <h3 className="text-[18px] font-medium text-text-primary">chatwoot/chatwoot LICENSE</h3>
            <span className="font-mono text-[11px] text-text-tertiary uppercase">MIT + enterprise/LICENSE</span>
          </div>
          <p className="text-[14px] leading-6 text-text-secondary">
            Chatwoot's root LICENSE puts everything outside <code className="font-mono text-[13px]">enterprise/</code>{" "}
            under MIT Expat. All files under <code className="font-mono text-[13px]">enterprise/</code> fall under the
            Chatwoot Enterprise License, which allows development and testing without payment but requires a valid paid
            Chatwoot Enterprise subscription for the right number of user seats in production.
          </p>
          <a
            href="https://github.com/chatwoot/chatwoot/blob/develop/enterprise/LICENSE"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1 self-start font-mono text-[12px] text-text-secondary underline-offset-4 hover:text-text-primary hover:underline"
          >
            Read Chatwoot enterprise/LICENSE
            <RiExternalLinkLine className="size-3.5" aria-hidden />
          </a>
        </div>
      </div>
    </Section>
  );
}

export function VsChatwootPage() {
  return (
    <div style={rivalToneStyle(chatwoot)}>
      <VsHero
        rival={chatwoot}
        eyebrow="dimmi vs Chatwoot"
        title="Pure MIT open source in one Go binary, with no enterprise folder."
        verdict={
          <p>
            Pick Chatwoot if you need Telegram or LINE channels today, native open-source mobile apps, or a Ruby
            ecosystem with years of community templates. Pick dimmi if you want SAML SSO and audit logs under pure MIT,
            a single Go binary on a lean Linux box, and BYO AI keys without Captain credit limits.
          </p>
        }
        authorId="frantz-kati"
        reviewedById="frantz-kati"
        published={CHECKED_ON_ISO}
        updated={CHECKED_ON_ISO}
        primaryCta={{ label: "Self-host dimmi on GitHub", href: GITHUB_URL }}
        secondaryCta={{ label: "Try dimmi Cloud free", href: `${APP_URL}/signup` }}
        visual={<StackCompareHeroVisual />}
      />

      <WinCards
        title="Where Chatwoot leads today, and where dimmi takes over."
        body="Chatwoot has ~40.7k GitHub stars and years in production. Here is where that maturity wins, and where a pure-MIT Go stack changes the math."
        rivalName="Chatwoot"
        rivalWins={chatwoot.rivalWins}
        dimmiWins={chatwoot.dimmiWins}
      />

      <LicenseSection />

      <SelfHostCostBuilder />

      <ProofBand
        tag="Install and MCP proof"
        title="Five-minute Linux install, MIT SAML SSO, and a built-in MCP server."
        body="Install dimmi with one shell command, configure SAML SSO without paying per-seat enterprise fees, and connect coding agents over MCP."
        caption="Top left: live installer sequence via install.sh and dimmictl. Top right: built-in MCP server configuration for Claude Code, Cursor, and Codex. Bottom: real dimmi SAML SSO settings screen unlocked under MIT."
      >
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
          <div className="overflow-hidden rounded-2xl border border-border-button-default bg-background-full">
            <TerminalScene />
          </div>
          <div className="flex flex-col justify-between gap-4 rounded-2xl border border-border-button-default bg-background-full p-6">
            <div className="flex flex-col gap-3">
              <div className="flex items-center gap-2 font-mono text-[12px] text-text-tertiary uppercase">
                <RiTerminalBoxLine className="size-4" aria-hidden />
                <span>Built-in MCP server (MIT)</span>
              </div>
              <p className="text-[14px] leading-6 text-text-secondary">
                Point Claude Code, Cursor, or Codex directly at your dimmi workspace over MCP to read open bugs, draft
                replies, and inspect help docs without paying for Operator seats or Captain credits.
              </p>
              <pre className="overflow-x-auto rounded-xl border border-separator-border bg-background-secondary-default/50 p-4 font-mono text-[12px] leading-5 text-text-primary">
                {`{
  "mcpServers": {
    "dimmi": {
      "url": "https://support.example.com/mcp",
      "headers": {
        "Authorization": "Bearer dmi_..."
      }
    }
  }
}`}
              </pre>
            </div>
            <ul className="flex flex-wrap gap-x-5 gap-y-2 text-[13px] text-text-secondary">
              <li className="inline-flex items-center gap-1.5">
                <RiCheckLine className="size-4 text-accent-600 dark:text-accent-400" aria-hidden />
                Zero MCP action fee
              </li>
              <li className="inline-flex items-center gap-1.5">
                <RiServerLine className="size-4 text-accent-600 dark:text-accent-400" aria-hidden />
                Runs inside the same Go binary
              </li>
            </ul>
          </div>
        </div>
        <div className="pt-2">
          <ProductShot
            name="vs-sso"
            alt="dimmi Single sign-on settings screen showing identity provider connections and verified domain controls included under MIT"
          />
        </div>
        <div className="flex flex-wrap gap-6 pt-3">
          <MonoLink href="/vs">See all comparisons</MonoLink>
          <MonoLink href="/vs/intercom">Compare dimmi vs Intercom</MonoLink>
          <MonoLink href="/vs/crisp">Compare dimmi vs Crisp</MonoLink>
          <MonoLink href="/vs/tawk-to">Compare dimmi vs Tawk.to</MonoLink>
          <MonoLink href="/pricing">Explore dimmi pricing</MonoLink>
        </div>
      </ProofBand>

      <SwitchTriggers
        title="Why self-hosting teams look beyond Chatwoot."
        body="Verified directly from chatwoot.com/pricing and github.com/chatwoot/chatwoot on November 27, 2026."
        rivalName="Chatwoot"
        triggers={chatwoot.switchTriggers}
      />

      <VsMatrix
        title="Architecture, licensing, and channels side by side."
        body="Honest marks across both repositories, including channels Chatwoot ships today that dimmi does not yet."
        rivalName="Chatwoot"
        groups={CHATWOOT_MATRIX}
      />

      <TeamPicker
        rival={chatwoot}
        title="Four support teams, and which open-source stack fits each."
        body="Includes the social-channels team that should pick Chatwoot right now."
        teams={chatwoot.teams}
        rivalMonthlyForTeam={(team) => {
          if (team.profile.startsWith("Solo")) {
            return 24;
          }
          if (team.profile.startsWith("Regulated")) {
            return team.seats * 99;
          }
          return team.seats * 39;
        }}
      />

      <Questions items={CHATWOOT_QUESTIONS} />

      <CtaBand
        title="Self-host the full stack under MIT, with zero enterprise folder."
        links={[
          { label: "Self-host on GitHub", href: GITHUB_URL, variant: "primary", icon: "github" },
          { label: "Start free on dimmi Cloud", href: `${APP_URL}/signup`, variant: "secondary", icon: "arrow" },
        ]}
      />
    </div>
  );
}
