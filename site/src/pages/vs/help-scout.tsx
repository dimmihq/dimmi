/*
 * dimmi vs Help Scout comparison page (`/vs/help-scout`).
 * Unique elements: inbox-count surcharge hero visual, interactive inboxes +
 * Envoy AI resolutions slider, and real dimmi help center + inboxes captures.
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

const helpScout = getRival("help-scout");

const HELPSCOUT_MATRIX: VsMatrixGroup[] = [
  {
    title: "Seats, inboxes, and AI",
    rows: [
      {
        feature: "Teammate seats",
        dimmi: { status: "yes", label: "$0 unlimited" },
        rival: { status: "no", label: "$25 to $75/user/mo" },
      },
      {
        feature: "Multiple shared inboxes",
        dimmi: { status: "yes", label: "Unlimited ($0 extra)" },
        rival: { status: "partial", label: "2 on Standard (+$10-$12 each)" },
      },
      {
        feature: "AI agent resolutions",
        dimmi: { status: "yes", label: "From $0.10/chat or BYO key" },
        rival: { status: "no", label: "$0.75/Envoy resolution" },
      },
      {
        feature: "MIT open-source self-hosting",
        dimmi: { status: "yes", label: "100% MIT ($0)" },
        rival: { status: "no", label: "Closed SaaS" },
      },
    ],
  },
  {
    title: "Routing, docs, and channels",
    rows: [
      {
        feature: "Round robin and internal teams",
        dimmi: { status: "yes", label: "Every workspace" },
        rival: { status: "partial", label: "Plus ($45/user/mo)" },
      },
      {
        feature: "SAML SSO",
        dimmi: { status: "yes", label: "Every workspace" },
        rival: { status: "partial", label: "Pro ($75/user, 10 min)" },
      },
      {
        feature: "Slack, WhatsApp, Instagram, and Messenger",
        dimmi: { status: "yes", label: "Built in" },
        rival: { status: "partial", label: "Email + Beacon + apps" },
      },
      {
        feature: "Help center with source-cited AI replies",
        dimmi: { status: "yes", label: "Included" },
        rival: { status: "yes", label: "Docs + Envoy" },
      },
    ],
  },
  {
    title: "Track record and certifications",
    rows: [
      {
        feature: "SOC 2 Type II and HIPAA compliance",
        requestTitle: "SOC 2 Type II and HIPAA compliance",
        dimmi: { status: "no" },
        rival: { status: "yes", label: "Certified (HIPAA on Pro)" },
      },
      {
        feature: "Free tier for up to 5 email users",
        dimmi: { status: "yes", label: `${FREE_CONVERSATIONS} chats/mo, unlimited seats` },
        rival: { status: "yes", label: "5 users, 25 contacts/mo" },
      },
      {
        feature: "Policy procedures with simulations",
        dimmi: { status: "yes", label: "Included" },
        rival: { status: "no", label: "Workflows only" },
      },
    ],
  },
];

const HELPSCOUT_QUESTIONS = [
  {
    question: "How much do extra inboxes cost on Help Scout compared with dimmi?",
    answer:
      "Help Scout Standard ($25/user/mo annual) includes 2 inboxes and charges $10/month annual ($12 monthly) for each additional inbox, plus $20/month per extra Docs site. dimmi lets you run unlimited inboxes and brands at $0 extra.",
  },
  {
    question: "How does Envoy AI pricing compare with dimmi's support agents?",
    answer:
      "Help Scout bills Envoy at $0.75 per resolved conversation after its introductory trial. dimmi includes 100 free conversations monthly, charges from $0.10 per AI conversation on Cloud, and drops AI fees by 50% to 83% when you plug in your own OpenAI or Anthropic key.",
  },
  {
    question: "When is Help Scout the better pick?",
    answer:
      "Pick Help Scout if you run a tiny email-only shop under 25 contacts/month on its Free plan, love its mature email-first interface, or require vendor SOC 2 or HIPAA compliance right now.",
  },
  {
    question: "Can dimmi handle email threads as cleanly as Help Scout?",
    answer:
      "Yes. Connect your own domain with DKIM/SPF verification, set custom signatures, route addresses into dedicated inboxes, and answer email beside web chat, Slack, and WhatsApp in one shared inbox.",
  },
  {
    question: "What happens when a small team needs SAML SSO on Help Scout?",
    answer:
      "Help Scout puts SAML SSO on its Pro plan ($75/user/month billed annually) with a 10-user minimum, starting at $750/month even if you only have 4 support teammates. dimmi includes SAML SSO and audit logs on every workspace, including the free tier and MIT self-hosted installs.",
  },
  {
    question: "How does Help Scout Beacon compare with dimmi's web messenger and help center?",
    answer:
      "Help Scout pairs Beacon with Docs sites (2 Docs sites on Standard, then $20/month per extra site). dimmi bundles a customizable web messenger, multilingual help center, changelog, and feedback board in one workspace with zero per-site add-ons.",
  },
];

function InboxSurchargeHeroVisual() {
  const sampleInboxes = ["support@", "billing@", "security@", "partners@", "eu-help@", "onboarding@"];
  return (
    <div className="flex flex-col gap-4 rounded-2xl border border-border-button-default bg-background-full p-6 shadow-xs">
      <div className="flex items-center justify-between border-b border-separator-border pb-3 font-mono text-[11px] text-text-tertiary uppercase">
        <span>6 shared inboxes · 5 teammates</span>
        <span>Checked {CHECKED_ON}</span>
      </div>

      <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">
        {sampleInboxes.map((addr, idx) => (
          <div
            key={addr}
            className="flex flex-col gap-0.5 rounded-xl border border-separator-border bg-background-secondary-default/35 px-3 py-2.5"
          >
            <span className="font-mono text-[12px] font-medium text-text-primary">{addr}</span>
            <span className="font-mono text-[11px] text-text-tertiary">
              {idx < 2 ? "Included on Standard" : "+$10-$12/mo extra"}
            </span>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-2 gap-3 pt-1">
        <div className="rounded-xl border border-separator-border bg-background-full p-3.5">
          <div className="font-mono text-[11px] text-text-tertiary uppercase">Help Scout Standard</div>
          <div className="font-mono text-[20px] font-medium text-text-primary tabular-nums">$165+/mo</div>
          <div className="text-[12px] text-text-secondary">5 seats + 4 extra inboxes + $0.75/AI res</div>
        </div>
        <div className="rounded-xl border border-accent-500/40 bg-accent-500/[0.05] p-3.5">
          <div className="font-mono text-[11px] text-accent-700 uppercase dark:text-accent-300">dimmi</div>
          <div className="font-mono text-[20px] font-medium text-text-primary tabular-nums">$0/inbox</div>
          <div className="text-[12px] text-text-secondary">Unlimited inboxes, unlimited free seats</div>
        </div>
      </div>
    </div>
  );
}

function InboxesResolutionsSlider() {
  const inboxesId = useId();
  const seatsId = useId();
  const resId = useId();
  const [inboxes, setInboxes] = useState(5);
  const [seats, setSeats] = useState(6);
  const [resolutions, setResolutions] = useState(800);

  const needsPlus = seats > 25;
  const seatRate = needsPlus ? 45 : 25;
  const includedInboxes = needsPlus ? 5 : 2;
  const extraInboxes = Math.max(0, inboxes - includedInboxes);
  const inboxFee = extraInboxes * 10;
  const envoyFee = Math.round(resolutions * 0.75);
  const helpScoutTotal = seats * seatRate + inboxFee + envoyFee;

  // Approximate total conversations when 60% of AI chats resolve (~80% AI share).
  const estConversations = Math.max(200, Math.round(resolutions / 0.48));
  const dimmiCloud = Math.round(
    bill({ conversations: estConversations, aiShare: 0.75, operator: 0, ownModel: false }).total,
  );
  const dimmiByok = Math.round(
    bill({ conversations: estConversations, aiShare: 0.75, operator: 0, ownModel: true }).total,
  );

  return (
    <Section frameClassName="flex flex-col gap-10">
      <SplitHead
        tag="Inboxes and AI slider"
        title="See what extra inboxes and $0.75 Envoy resolutions add to your Help Scout bill."
        body="Help Scout Standard ($25/user annual) includes 2 inboxes and adds $10/month per extra inbox. Crossing 25 users forces Plus ($45/user)."
      />

      <div className="flex flex-col gap-6 rounded-2xl border border-border-button-default bg-background-full p-6 sm:p-8">
        <div className="grid grid-cols-1 gap-6 md:grid-cols-3">
          <div className="flex flex-col gap-2">
            <div className="flex items-baseline justify-between">
              <label htmlFor={inboxesId} className="text-[13px] font-medium text-text-secondary">
                Shared inboxes
              </label>
              <span className="font-mono text-[14px] font-medium text-text-primary tabular-nums">
                {inboxes} inboxes
              </span>
            </div>
            <input
              id={inboxesId}
              type="range"
              min={1}
              max={12}
              step={1}
              value={inboxes}
              onChange={(e) => setInboxes(Number(e.target.value))}
              className="w-full cursor-pointer accent-accent-600"
            />
          </div>

          <div className="flex flex-col gap-2">
            <div className="flex items-baseline justify-between">
              <label htmlFor={seatsId} className="text-[13px] font-medium text-text-secondary">
                Teammates
              </label>
              <span className="font-mono text-[14px] font-medium text-text-primary tabular-nums">{seats} users</span>
            </div>
            <input
              id={seatsId}
              type="range"
              min={1}
              max={40}
              step={1}
              value={seats}
              onChange={(e) => setSeats(Number(e.target.value))}
              className="w-full cursor-pointer accent-accent-600"
            />
          </div>

          <div className="flex flex-col gap-2">
            <div className="flex items-baseline justify-between">
              <label htmlFor={resId} className="text-[13px] font-medium text-text-secondary">
                AI resolutions / mo
              </label>
              <span className="font-mono text-[14px] font-medium text-text-primary tabular-nums">
                {resolutions.toLocaleString("en-US")}
              </span>
            </div>
            <input
              id={resId}
              type="range"
              min={0}
              max={2000}
              step={100}
              value={resolutions}
              onChange={(e) => setResolutions(Number(e.target.value))}
              className="w-full cursor-pointer accent-accent-600"
            />
          </div>
        </div>

        <div className="grid grid-cols-1 gap-4 border-t border-separator-border pt-5 md:grid-cols-3">
          <div className="flex flex-col gap-1 rounded-xl border border-border-button-default bg-background-secondary-default/40 p-5">
            <span className="font-mono text-[11px] text-text-tertiary uppercase">
              Help Scout {needsPlus ? "Plus" : "Standard"}
            </span>
            <span className="font-mono text-[30px] font-medium text-text-primary tabular-nums">
              {dollars(helpScoutTotal)}/mo
            </span>
            <span className="text-[12px] text-text-secondary">
              {`${dollars(seats * seatRate)} seats + ${dollars(inboxFee)} extra inboxes + ${dollars(envoyFee)} Envoy`}
            </span>
          </div>

          <div className="flex flex-col gap-1 rounded-xl border border-accent-500/35 bg-accent-500/[0.04] p-5">
            <span className="font-mono text-[11px] text-accent-700 uppercase dark:text-accent-300">
              Managed Cloud bill
            </span>
            <span className="font-mono text-[30px] font-medium text-text-primary tabular-nums">
              {dollars(dimmiCloud)}/mo
            </span>
            <span className="text-[12px] text-text-secondary">
              {`${inboxes} inboxes ($0) + ${seats} seats ($0) + ~${estConversations.toLocaleString("en-US")} chats`}
            </span>
          </div>

          <div className="flex flex-col gap-1 rounded-xl border border-accent-500/35 bg-accent-500/[0.04] p-5">
            <span className="font-mono text-[11px] text-accent-700 uppercase dark:text-accent-300">
              Direct API keys
            </span>
            <span className="font-mono text-[30px] font-medium text-text-primary tabular-nums">
              {dollars(dimmiByok)}/mo
            </span>
            <span className="text-[12px] text-text-secondary">Zero license charge when self-hosted</span>
          </div>
        </div>
      </div>
    </Section>
  );
}

export function VsHelpScoutPage() {
  return (
    <div style={rivalToneStyle(helpScout)}>
      <VsHero
        rival={helpScout}
        eyebrow="dimmi vs Help Scout"
        title="Unlimited inboxes, free seats, and your own AI model keys."
        verdict={
          <p>
            Pick Help Scout if you want a veteran email-first help desk with SOC 2 and HIPAA compliance or a free tier
            for up to 5 users under 25 contacts a month. Choose dimmi when multiple brand mailboxes, zero seat costs,
            live chat beside email, direct LLM keys, and MIT software fit your support workflow.
          </p>
        }
        authorId="frantz-kati"
        reviewedById="frantz-kati"
        published={CHECKED_ON_ISO}
        updated={CHECKED_ON_ISO}
        primaryCta={{ label: "Open free inbox", href: `${APP_URL}/signup` }}
        secondaryCta={{ label: "Browse GitHub source", href: GITHUB_URL, github: true }}
        visual={<InboxSurchargeHeroVisual />}
      />

      <WinCards
        tag="Help Scout trade-offs"
        title="Where Help Scout shines, and where dimmi saves you from add-ons."
        body="Audited from Help Scout pricing late Nov 2026."
        rivalName="Help Scout"
        rivalWins={helpScout.rivalWins}
        dimmiWins={helpScout.dimmiWins}
      />

      <InboxesResolutionsSlider />

      <BillDuel
        rival={helpScout}
        tag="Mailbox bill duel"
        title="Compare Help Scout Standard and Plus against dimmi."
        body="Standard bills $25/user/mo annual (up to 25 users), Plus bills $45/user/mo for round robin and customer portals, and Envoy adds $0.75 per resolution."
        defaultSeats={6}
        defaultConversations={2000}
        defaultAiShare={0.7}
      />

      <ProofBand
        tag="Help center and inboxes"
        title="Built-in help center and unlimited shared inboxes with source-cited AI."
        body="Publish docs in multiple languages, ground your support agent on real articles, and split incoming email and chat across as many inboxes as you need."
        caption="Left: dimmi Help center editor and published knowledge base. Right: dimmi Inboxes settings with zero per-inbox fees."
      >
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
          <ProductShot
            name="help-center"
            alt="dimmi help center showing organized collections, articles, and source-grounded knowledge"
          />
          <ProductShot
            name="vs-inboxes"
            alt="dimmi inboxes settings page showing shared inbox management and New inbox controls"
          />
        </div>
        <div className="flex flex-wrap gap-6 pt-3">
          <MonoLink href="/vs">View all rival guides</MonoLink>
          <MonoLink href="/vs/zendesk">Read Zendesk breakdown</MonoLink>
          <MonoLink href="/help-center">Explore dimmi Help center</MonoLink>
          <MonoLink href="/pricing">Review Cloud rates</MonoLink>
        </div>
      </ProofBand>

      <SwitchTriggers
        tag="Help Scout triggers"
        title="Why multi-inbox teams move from Help Scout to dimmi."
        body="Sourced directly from Help Scout Standard, Plus, and Pro tiers."
        rivalName="Help Scout"
        triggers={helpScout.switchTriggers}
      />

      <VsMatrix
        title="Inboxes, AI agents, and compliance compared plainly."
        body="Includes the certifications Help Scout holds today that dimmi has not completed yet."
        rivalName="Help Scout"
        groups={HELPSCOUT_MATRIX}
      />

      <TeamPicker
        rival={helpScout}
        tag="Help Scout team fit"
        title="Four support teams and who should pick what."
        body="Includes the small email team that fits Help Scout Free best."
        teams={helpScout.teams}
      />

      <Questions items={HELPSCOUT_QUESTIONS} />

      <CtaBand
        title="Add every teammate and every inbox without paying extra."
        links={[
          { label: "Launch free workspace", href: `${APP_URL}/signup`, variant: "primary", icon: "arrow" },
          { label: "Run MIT self-hosted", href: GITHUB_URL, variant: "secondary", icon: "github" },
        ]}
      />
    </div>
  );
}
