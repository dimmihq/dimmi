import { RiArrowRightLine, RiCheckLine, RiCloseLine, RiExternalLinkLine, RiSubtractLine } from "@remixicon/react";
import { Fragment } from "react";

import { ButtonLink } from "@/components/base/buttons/button";
import { cx } from "@/utils/cx";

import { Intro, MonoLink, Section, Tag } from "../components/container";
import { Questions } from "../components/product-page";
import { APP_URL, GITHUB_URL, HELP_URL } from "../links";
import { CompareBars } from "../pricing/comparison";
import { CHECKED_ON, COMPETITORS, RESOLUTION_RATE, SCENARIOS } from "../pricing/competitors";
import { bill, dollars } from "../pricing/prices";
import { SDK_COUNT } from "../sdks";
import { FinalCta } from "../sections/final-cta";
import { track } from "../track";

const mono = "font-mono text-[12px] leading-5 font-medium tracking-[0.04em] uppercase";

function requestUrl(title: string): string {
  return `${HELP_URL}/feedback/new?${new URLSearchParams({ title }).toString()}`;
}

type CellStatus = "yes" | "partial" | "no";
type MatrixCell = { status: CellStatus; label?: string };
type MatrixRow = {
  feature: string;
  requestTitle?: string;
  dimmi: MatrixCell;
  helply: MatrixCell;
  chatbase: MatrixCell;
  intercom: MatrixCell;
};

const matrixGroups: { title: string; rows: MatrixRow[] }[] = [
  {
    title: "AI agents & operations",
    rows: [
      {
        feature: "Unlimited support agents",
        dimmi: { status: "yes", label: "Unlimited" },
        helply: { status: "partial", label: "Fixed roles" },
        chatbase: { status: "partial", label: "Paid add-on" },
        intercom: { status: "partial", label: "Multi-brand tier" },
      },
      {
        feature: "Agent-to-agent transfers",
        dimmi: { status: "yes", label: "Built in" },
        helply: { status: "partial", label: "Fixed routing" },
        chatbase: { status: "no", label: "No" },
        intercom: { status: "no", label: "Single agent" },
      },
      {
        feature: "Source citations & doc grounding",
        dimmi: { status: "yes", label: "Included" },
        helply: { status: "yes", label: "Included" },
        chatbase: { status: "yes", label: "Included" },
        intercom: { status: "yes", label: "Included" },
      },
      {
        feature: "Multi-language replies",
        dimmi: { status: "yes", label: "Included" },
        helply: { status: "yes", label: "Included" },
        chatbase: { status: "yes", label: "Included" },
        intercom: { status: "yes", label: "Included" },
      },
      {
        feature: "Custom API & tool actions",
        dimmi: { status: "yes", label: "HTTP & MCP" },
        helply: { status: "yes", label: "Custom actions" },
        chatbase: { status: "yes", label: "Custom actions" },
        intercom: { status: "yes", label: "Custom actions" },
      },
      {
        feature: "Policy Procedures & approvals",
        dimmi: { status: "yes", label: "Included" },
        helply: { status: "partial", label: "Workflows" },
        chatbase: { status: "partial", label: "Prompts & diffs" },
        intercom: { status: "yes", label: "Fin Procedures" },
      },
      {
        feature: "Simulations, Draft vs. Live & rollback",
        dimmi: { status: "yes", label: "Included" },
        helply: { status: "partial", label: "Basic" },
        chatbase: { status: "partial", label: "Playground" },
        intercom: { status: "yes", label: "Simulations" },
      },
      {
        feature: "AI copilot & backstage ops",
        dimmi: { status: "yes", label: "Operator" },
        helply: { status: "yes", label: "Assist Agent" },
        chatbase: { status: "yes", label: "Backstage" },
        intercom: { status: "yes", label: "Fin Copilot" },
      },
      {
        feature: "Scheduled routines & PR to docs",
        dimmi: { status: "yes", label: "Inline diffs" },
        helply: { status: "partial", label: "Background bots" },
        chatbase: { status: "partial", label: "Scheduled audits" },
        intercom: { status: "no", label: "No" },
      },
      {
        feature: "Notion, GitHub & Zendesk doc sync",
        dimmi: { status: "yes", label: "Included" },
        helply: { status: "yes", label: "Included" },
        chatbase: { status: "yes", label: "Included" },
        intercom: { status: "yes", label: "Included" },
      },
      {
        feature: "Drop-in AI over Zendesk or SFDC",
        requestTitle: "Drop-in AI layer over Zendesk or Salesforce",
        dimmi: { status: "no" },
        helply: { status: "yes", label: "Zendesk layer" },
        chatbase: { status: "yes", label: "Zendesk & more" },
        intercom: { status: "yes", label: "Zendesk & SFDC" },
      },
    ],
  },
  {
    title: "Inbox & channels",
    rows: [
      {
        feature: "Shared inbox & customer history",
        dimmi: { status: "yes", label: "Native" },
        helply: { status: "yes", label: "Native" },
        chatbase: { status: "yes", label: "Helpdesk" },
        intercom: { status: "yes", label: "Native" },
      },
      {
        feature: "Multiple inboxes, views & round robin",
        dimmi: { status: "yes", label: "Included" },
        helply: { status: "yes", label: "Team tier+" },
        chatbase: { status: "partial", label: "Balanced routing" },
        intercom: { status: "yes", label: "Advanced+" },
      },
      {
        feature: "Notes, snoozes & saved replies",
        dimmi: { status: "yes", label: "Included" },
        helply: { status: "yes", label: "Included" },
        chatbase: { status: "yes", label: "Included" },
        intercom: { status: "yes", label: "Included" },
      },
      {
        feature: "Live inbox translation & undo send",
        dimmi: { status: "yes", label: "Included" },
        helply: { status: "partial", label: "Translation" },
        chatbase: { status: "partial", label: "Translation" },
        intercom: { status: "yes", label: "Included" },
      },
      {
        feature: "iOS & Android teammate inbox apps",
        dimmi: { status: "yes", label: "iOS & Android" },
        helply: { status: "no", label: "Web only" },
        chatbase: { status: "no", label: "Web only" },
        intercom: { status: "yes", label: "iOS & Android" },
      },
      {
        feature: "Web messenger, email & Slack",
        dimmi: { status: "yes", label: "Included" },
        helply: { status: "yes", label: "Included" },
        chatbase: { status: "yes", label: "Included" },
        intercom: { status: "yes", label: "Included" },
      },
      {
        feature: "WhatsApp, Instagram, Messenger & SMS",
        dimmi: { status: "yes", label: "Included" },
        helply: { status: "yes", label: "Team tier+" },
        chatbase: { status: "yes", label: "Paid plans" },
        intercom: { status: "yes", label: "Included" },
      },
      {
        feature: "Microsoft Teams & Discord",
        dimmi: { status: "yes", label: "Included" },
        helply: { status: "yes", label: "Team tier+" },
        chatbase: { status: "no", label: "No" },
        intercom: { status: "partial", label: "Via apps" },
      },
      {
        feature: "Telegram channel",
        requestTitle: "Telegram inbox channel",
        dimmi: { status: "no" },
        helply: { status: "yes", label: "Included" },
        chatbase: { status: "no", label: "No" },
        intercom: { status: "partial", label: "Via apps" },
      },
      {
        feature: "Phone & voice support",
        dimmi: { status: "yes", label: "Included" },
        helply: { status: "yes", label: "Via Aircall" },
        chatbase: { status: "yes", label: "Twilio / SIP" },
        intercom: { status: "yes", label: "Fin Voice" },
      },
      {
        feature: "Call-center dialers, IVR & shift rosters",
        requestTitle: "Call-center dialers, IVR, and shift rosters",
        dimmi: { status: "no" },
        helply: { status: "partial", label: "Via Aircall" },
        chatbase: { status: "no", label: "No" },
        intercom: { status: "yes", label: "Fin Voice & WFM" },
      },
      {
        feature: "Co-browsing & live screen sharing",
        requestTitle: "Co-browsing and live screen sharing",
        dimmi: { status: "no" },
        helply: { status: "no", label: "No" },
        chatbase: { status: "no", label: "No" },
        intercom: { status: "partial", label: "Via app store" },
      },
    ],
  },
  {
    title: "Tickets, automation & reporting",
    rows: [
      {
        feature: "Custom ticket types, states & fields",
        dimmi: { status: "yes", label: "Included" },
        helply: { status: "yes", label: "Included" },
        chatbase: { status: "partial", label: "Basic tickets" },
        intercom: { status: "yes", label: "Included" },
      },
      {
        feature: "Multi-customer incident trackers",
        dimmi: { status: "yes", label: "Built in" },
        helply: { status: "no", label: "No" },
        chatbase: { status: "no", label: "No" },
        intercom: { status: "yes", label: "Tracker tickets" },
      },
      {
        feature: "Linear, Jira, GitHub & PagerDuty",
        dimmi: { status: "yes", label: "Included" },
        helply: { status: "yes", label: "Linear, Jira, GH" },
        chatbase: { status: "partial", label: "Linear" },
        intercom: { status: "yes", label: "Included" },
      },
      {
        feature: "Visual workflows & business-hour SLAs",
        dimmi: { status: "yes", label: "Included" },
        helply: { status: "yes", label: "Team tier+" },
        chatbase: { status: "partial", label: "Routing & actions" },
        intercom: { status: "yes", label: "Advanced+" },
      },
      {
        feature: "Custom attributes, events & audiences",
        dimmi: { status: "yes", label: "Included" },
        helply: { status: "yes", label: "Included" },
        chatbase: { status: "partial", label: "Basic fields" },
        intercom: { status: "yes", label: "Included" },
      },
      {
        feature: "Click-through reports & CSAT",
        dimmi: { status: "yes", label: "Included" },
        helply: { status: "yes", label: "Pro tier+" },
        chatbase: { status: "yes", label: "Included" },
        intercom: { status: "yes", label: "Included" },
      },
      {
        feature: "Side-by-side agent performance reports",
        dimmi: { status: "yes", label: "Included" },
        helply: { status: "no", label: "No" },
        chatbase: { status: "partial", label: "Per agent" },
        intercom: { status: "no", label: "Single agent" },
      },
    ],
  },
  {
    title: "Self-service, outbound & engagement",
    rows: [
      {
        feature: "Custom-domain help center",
        dimmi: { status: "yes", label: "Included" },
        helply: { status: "yes", label: "Included" },
        chatbase: { status: "partial", label: "Help Pages" },
        intercom: { status: "yes", label: "Included" },
      },
      {
        feature: "Markdown docs & stale-translation alerts",
        dimmi: { status: "yes", label: "Included" },
        helply: { status: "no", label: "No" },
        chatbase: { status: "no", label: "No" },
        intercom: { status: "no", label: "No" },
      },
      {
        feature: "Broadcasts & multi-step series",
        dimmi: { status: "yes", label: "Included" },
        helply: { status: "partial", label: "Broadcasts" },
        chatbase: { status: "no", label: "No" },
        intercom: { status: "yes", label: "Series & outbound" },
      },
      {
        feature: "In-app changelog & release notes",
        dimmi: { status: "yes", label: "Included" },
        helply: { status: "no", label: "No" },
        chatbase: { status: "no", label: "No" },
        intercom: { status: "yes", label: "News" },
      },
      {
        feature: "Public feature-request board & roadmap",
        dimmi: { status: "yes", label: "Included" },
        helply: { status: "partial", label: "AI capture only" },
        chatbase: { status: "no", label: "No" },
        intercom: { status: "no", label: "Via apps" },
      },
      {
        feature: "In-app product tours & checklists",
        requestTitle: "In-app product tours and onboarding checklists",
        dimmi: { status: "no" },
        helply: { status: "no", label: "No" },
        chatbase: { status: "no", label: "No" },
        intercom: { status: "yes", label: "Paid add-on" },
      },
      {
        feature: "In-app NPS surveys & banners",
        requestTitle: "In-app NPS surveys and banners",
        dimmi: { status: "no" },
        helply: { status: "no", label: "No" },
        chatbase: { status: "no", label: "No" },
        intercom: { status: "yes", label: "Paid add-on" },
      },
      {
        feature: "A/B testing for broadcasts & series",
        requestTitle: "A/B testing for broadcasts and series",
        dimmi: { status: "no" },
        helply: { status: "no", label: "No" },
        chatbase: { status: "no", label: "No" },
        intercom: { status: "yes", label: "Included" },
      },
      {
        feature: "In-chat meeting booking (Calendly)",
        requestTitle: "Native Calendly and Cal.com booking widget",
        dimmi: { status: "no" },
        helply: { status: "partial", label: "Via actions" },
        chatbase: { status: "yes", label: "Native Calendly" },
        intercom: { status: "yes", label: "App store" },
      },
    ],
  },
  {
    title: "Ecosystem, developers & compliance",
    rows: [
      {
        feature: "Native Shopify & Stripe integrations",
        requestTitle: "Native Shopify and Stripe integrations",
        dimmi: { status: "no" },
        helply: { status: "yes", label: "Stripe native" },
        chatbase: { status: "yes", label: "Native" },
        intercom: { status: "yes", label: "Native" },
      },
      {
        feature: "Native Salesforce & HubSpot CRM sync",
        requestTitle: "Native Salesforce and HubSpot CRM sync",
        dimmi: { status: "no" },
        helply: { status: "yes", label: "Native" },
        chatbase: { status: "yes", label: "Native" },
        intercom: { status: "yes", label: "Native" },
      },
      {
        feature: "Data warehouse sync (Snowflake)",
        requestTitle: "Data warehouse sync (Snowflake, BigQuery)",
        dimmi: { status: "no" },
        helply: { status: "yes", label: "Snowflake" },
        chatbase: { status: "no", label: "No" },
        intercom: { status: "yes", label: "Via apps" },
      },
      {
        feature: "Zapier & Make connectors",
        requestTitle: "Native Zapier and Make connectors",
        dimmi: { status: "no" },
        helply: { status: "yes", label: "Native" },
        chatbase: { status: "yes", label: "Native" },
        intercom: { status: "yes", label: "Native" },
      },
      {
        feature: "Pre-built app marketplace",
        requestTitle: "Third-party app marketplace",
        dimmi: { status: "no" },
        helply: { status: "partial", label: "60+ apps" },
        chatbase: { status: "partial", label: "Core apps" },
        intercom: { status: "yes", label: "400+ app store" },
      },
      {
        feature: "REST API, signed webhooks & OAuth",
        dimmi: { status: "yes", label: "Included" },
        helply: { status: "yes", label: "Included" },
        chatbase: { status: "partial", label: "API & webhooks" },
        intercom: { status: "yes", label: "Included" },
      },
      {
        feature: "Typed backend & web framework SDKs",
        dimmi: { status: "yes", label: `${SDK_COUNT} SDKs` },
        helply: { status: "no", label: "REST only" },
        chatbase: { status: "no", label: "REST only" },
        intercom: { status: "partial", label: "Community SDKs" },
      },
      {
        feature: "Headless Messenger HTTP API",
        dimmi: { status: "yes", label: "Built in" },
        helply: { status: "no", label: "Widget only" },
        chatbase: { status: "yes", label: "Chat API" },
        intercom: { status: "no", label: "SDK only" },
      },
      {
        feature: "Native iOS & Android messenger SDKs",
        requestTitle: "Native iOS and Android messenger SDKs",
        dimmi: { status: "no" },
        helply: { status: "no", label: "Web view" },
        chatbase: { status: "no", label: "Web view" },
        intercom: { status: "yes", label: "iOS & Android" },
      },
      {
        feature: "MCP server & llms.txt",
        dimmi: { status: "yes", label: "MCP + llms.txt" },
        helply: { status: "no", label: "No" },
        chatbase: { status: "no", label: "No" },
        intercom: { status: "partial", label: "MCP" },
      },
      {
        feature: "SAML SSO, passkeys & audit logs",
        dimmi: { status: "yes", label: "Included" },
        helply: { status: "partial", label: "Enterprise" },
        chatbase: { status: "partial", label: "Enterprise" },
        intercom: { status: "partial", label: "Expert / Custom" },
      },
      {
        feature: "Retention, erasures & full ZIP export",
        dimmi: { status: "yes", label: "Included" },
        helply: { status: "partial", label: "API export" },
        chatbase: { status: "partial", label: "Basic export" },
        intercom: { status: "yes", label: "Included" },
      },
      {
        feature: "Regional data hosting (US / EU / AU)",
        requestTitle: "EU and regional data hosting on dimmi Cloud",
        dimmi: { status: "no" },
        helply: { status: "no", label: "US cloud" },
        chatbase: { status: "no", label: "US cloud" },
        intercom: { status: "yes", label: "US, EU & AU" },
      },
      {
        feature: "SOC 2 Type II report",
        requestTitle: "SOC 2 Type II report",
        dimmi: { status: "no" },
        helply: { status: "yes", label: "Certified" },
        chatbase: { status: "yes", label: "Certified" },
        intercom: { status: "yes", label: "Certified" },
      },
      {
        feature: "ISO 27001 certification",
        requestTitle: "ISO 27001 certification",
        dimmi: { status: "no" },
        helply: { status: "no", label: "No" },
        chatbase: { status: "no", label: "No" },
        intercom: { status: "yes", label: "Certified" },
      },
      {
        feature: "HIPAA compliance & Cloud BAA",
        requestTitle: "HIPAA compliance and BAA for dimmi Cloud",
        dimmi: { status: "no" },
        helply: { status: "partial", label: "Scale tier+" },
        chatbase: { status: "partial", label: "Enterprise" },
        intercom: { status: "yes", label: "Enterprise BAA" },
      },
    ],
  },
  {
    title: "Pricing & ownership",
    rows: [
      {
        feature: "Unlimited human seats",
        dimmi: { status: "yes", label: "Free" },
        helply: { status: "yes", label: "Included" },
        chatbase: { status: "yes", label: "Core tiers+" },
        intercom: { status: "no", label: "$29 to $132/seat" },
      },
      {
        feature: "Free tier with every feature",
        dimmi: { status: "yes", label: "100 chats / mo" },
        helply: { status: "no", label: "Trial only" },
        chatbase: { status: "partial", label: "Limited free" },
        intercom: { status: "no", label: "14-day trial" },
      },
      {
        feature: "AI support pricing",
        dimmi: { status: "yes", label: "10¢ to 6¢ / chat" },
        helply: { status: "no", label: "$1 / ticket" },
        chatbase: { status: "partial", label: "Credit tiers" },
        intercom: { status: "no", label: "$0.99 / res" },
      },
      {
        feature: "Bring your own model keys",
        dimmi: { status: "yes", label: "Save 50% to 83%" },
        helply: { status: "no", label: "No" },
        chatbase: { status: "no", label: "No" },
        intercom: { status: "no", label: "No" },
      },
      {
        feature: "Open source (MIT) self-hosting",
        dimmi: { status: "yes", label: "$0 software" },
        helply: { status: "no", label: "Closed" },
        chatbase: { status: "no", label: "Closed" },
        intercom: { status: "no", label: "Closed" },
      },
    ],
  },
];

const questions = [
  {
    question: "How does dimmi keep prices this low compared to Intercom and Helply?",
    answer:
      "We do not run an enterprise sales floor, and we price close to real infrastructure cost. Bring your own OpenAI or Anthropic key and you pay your model provider directly while dimmi charges only the human conversation rate.",
  },
  {
    question: "Can we migrate our help center, customers, and conversations into dimmi?",
    answer:
      "Import customers, companies, and help articles through the app or REST API, keep your existing email addresses and domain, and test your support agents on past conversations before switching traffic.",
  },
  {
    question: "Is anything locked behind an Enterprise plan on dimmi?",
    answer:
      "Unlimited seats, unlimited support agents, Procedures, Operator Routines, Workflows, SAML SSO, audit logs, and the full REST API and MCP server are included on every workspace and in the MIT-licensed self-hosted build.",
  },
  {
    question: "When is dimmi not the right choice?",
    answer:
      "If you need a vendor-hosted SOC 2 or ISO 27001 report or HIPAA BAA today, want to bolt AI onto Zendesk or Salesforce without migrating, need native iOS and Android messenger SDKs, or run a call-center phone floor with shift rosters, Intercom or another incumbent will fit better right now.",
  },
];

function StatusMark({ status }: { status: CellStatus }) {
  if (status === "yes") {
    return (
      <span className="inline-flex size-5 shrink-0 items-center justify-center rounded-full bg-accent-500/15 text-accent-600 dark:text-accent-400">
        <RiCheckLine className="size-3.5" aria-hidden />
      </span>
    );
  }
  if (status === "partial") {
    return (
      <span className="inline-flex size-5 shrink-0 items-center justify-center rounded-full bg-background-secondary-default text-text-tertiary">
        <RiSubtractLine className="size-3.5" aria-hidden />
      </span>
    );
  }
  return (
    <span className="inline-flex size-5 shrink-0 items-center justify-center rounded-full bg-background-secondary-default text-text-tertiary/65">
      <RiCloseLine className="size-3.5" aria-hidden />
    </span>
  );
}

function Hero() {
  return (
    <section className="relative bg-background-full">
      <div className="relative mx-auto w-full max-w-[1200px] overflow-hidden border-x border-separator-border">
        <div
          aria-hidden
          className="pointer-events-none absolute inset-0 opacity-60 [background-image:linear-gradient(var(--color-separator-border)_1px,transparent_1px),linear-gradient(90deg,var(--color-separator-border)_1px,transparent_1px)] [background-size:64px_64px] [mask-image:radial-gradient(60%_50%_at_50%_30%,black,transparent)]"
        />
        <div className="relative flex flex-col items-center gap-6 px-5 pt-20 pb-16 text-center sm:px-12 md:pt-28 md:pb-20">
          <Tag className="self-center">Compare</Tag>
          <h1 className="max-w-3xl text-[60px] leading-[1.04] font-medium tracking-[-0.035em] text-balance text-text-primary max-md:text-[46px] max-sm:text-[36px]">
            Modern customer support, priced without seat or resolution taxes.
          </h1>
          <p className="max-w-xl text-[18px] leading-7 text-pretty text-text-secondary">
            See how dimmi's specialized support agents, policy procedures, scheduled routines, and real monthly pricing
            stack up against Helply, Chatbase, and Intercom Fin.
          </p>
          <div className="mt-2 flex flex-wrap justify-center gap-3">
            <ButtonLink
              href={`${APP_URL}/signup`}
              trailingIcon={RiArrowRightLine}
              onClick={() => track("cta_start_free", { location: "compare_hero" })}
            >
              Start free, no card
            </ButtonLink>
            <ButtonLink
              href={GITHUB_URL}
              variant="secondary"
              onClick={() => track("github_click", { location: "compare_hero" })}
            >
              Self-host on GitHub
            </ButtonLink>
          </div>
        </div>
      </div>
    </section>
  );
}

function RequestLink({ title }: { title: string }) {
  return (
    <a
      href={requestUrl(title)}
      target="_blank"
      rel="noreferrer"
      className="inline-flex items-center gap-1 font-medium text-accent-600 underline-offset-4 hover:underline dark:text-accent-400"
    >
      Request
      <RiExternalLinkLine className="size-3.5" aria-hidden />
    </a>
  );
}

function MatrixDataCell({
  cell,
  highlight,
  requestTitle,
}: {
  cell: MatrixCell;
  highlight: boolean;
  requestTitle?: string;
}) {
  const showRequest = highlight && requestTitle ? requestTitle : null;
  const labelClass = highlight && cell.status === "yes" ? "font-medium text-text-primary" : "text-text-secondary";

  return (
    <td
      className={cx("whitespace-nowrap px-4 py-3.5", highlight && "border-x border-accent-500/15 bg-accent-500/[0.03]")}
    >
      <span className="inline-flex items-center gap-2">
        <StatusMark status={cell.status} />
        {showRequest ? (
          <RequestLink title={showRequest} />
        ) : cell.label ? (
          <span className={cx("tabular-nums", labelClass)}>{cell.label}</span>
        ) : null}
      </span>
    </td>
  );
}

function CapabilityMatrix() {
  const columns = [
    { key: "dimmi", label: "dimmi", highlight: true },
    { key: "helply", label: "Helply", highlight: false },
    { key: "chatbase", label: "Chatbase", highlight: false },
    { key: "intercom", label: "Intercom Fin", highlight: false },
  ] as const;

  return (
    <Section frameClassName="flex flex-col gap-12">
      <Intro
        title="What we share, where we differ, and what we do not build."
        body="Every tool here has real strengths. Here is an honest look at where dimmi wins, where competitors match us, and where you should pick someone else."
      />
      <div className="overflow-x-auto rounded-2xl border border-border-button-default bg-background-full">
        <table className="w-full min-w-[880px] table-fixed border-collapse text-start">
          <colgroup>
            <col className="w-[36%]" />
            <col className="w-[16%]" />
            <col className="w-[16%]" />
            <col className="w-[16%]" />
            <col className="w-[16%]" />
          </colgroup>
          <thead>
            <tr className="border-b border-separator-border">
              <th className={`px-6 py-4 text-start text-text-tertiary ${mono}`}>Capability</th>
              {columns.map((col) => (
                <th
                  key={col.key}
                  className={cx(
                    "whitespace-nowrap px-4 py-4 text-start text-[14px] font-medium",
                    col.highlight
                      ? "border-x border-accent-500/20 bg-accent-500/[0.08] text-accent-700 dark:text-accent-300"
                      : "text-text-primary",
                  )}
                >
                  {col.label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-separator-border text-[14px] leading-5">
            {matrixGroups.map((group) => (
              <Fragment key={group.title}>
                <tr className="bg-background-secondary-default/60">
                  <th colSpan={5} scope="colgroup" className={`px-6 py-2.5 text-start text-text-tertiary ${mono}`}>
                    {group.title}
                  </th>
                </tr>
                {group.rows.map((row) => (
                  <tr key={row.feature} className="transition-colors hover:bg-background-secondary-default/40">
                    <th
                      scope="row"
                      className="whitespace-nowrap px-6 py-3.5 text-start align-top font-medium text-text-primary"
                    >
                      {row.feature}
                    </th>
                    {columns.map((col) => (
                      <MatrixDataCell
                        key={col.key}
                        cell={row[col.key]}
                        highlight={col.highlight}
                        requestTitle={row.requestTitle}
                      />
                    ))}
                  </tr>
                ))}
              </Fragment>
            ))}
          </tbody>
        </table>
      </div>
    </Section>
  );
}

function SupportCostScenario() {
  /*
   * Concrete support team: 10 teammates running Frontline, Billing, and Docs
   * agents across 2,500 monthly support conversations (1,600 AI-handled).
   */
  const dimmiCloud = bill({ conversations: 2500, aiShare: 0.64, operator: 40, ownModel: false }).total;
  const dimmiByok = bill({ conversations: 2500, aiShare: 0.64, operator: 40, ownModel: true }).total;
  const intercomExpert = 10 * 132 + Math.round(1600 * RESOLUTION_RATE) * 0.99;
  const helplyFlat = 2500;

  const cards = [
    {
      name: "dimmi · BYO keys",
      price: dollars(dimmiByok),
      note: "Unlimited seats, support agents, and routines. Plus your own AI API spend.",
      accent: true,
    },
    {
      name: "dimmi · AI included",
      price: dollars(dimmiCloud),
      note: "Unlimited seats, support agents, Procedures, and Routines included.",
      accent: true,
    },
    {
      name: "Intercom Fin",
      price: dollars(intercomExpert),
      note: "10 Expert seats for multi-brand and SLAs, plus $0.99 per resolution.",
      accent: false,
    },
    {
      name: "Helply",
      price: dollars(helplyFlat),
      note: "Unlimited seats billed at $1 per ticket across 2,500 support chats.",
      accent: false,
    },
  ];

  return (
    <Section frameClassName="flex flex-col gap-12">
      <Intro
        title="What a real support month actually costs."
        body="10 teammates running Support, Billing, and Docs agents across 2,500 monthly conversations (1,600 AI-handled)."
      />
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-4 [&>*]:min-w-0">
        {cards.map((card) => (
          <div
            key={card.name}
            className={cx(
              "grid grid-rows-[auto_auto_1fr] gap-3 rounded-xl border p-6",
              card.accent
                ? "border-accent-500/40 bg-accent-500/[0.04]"
                : "border-border-button-default bg-background-full",
            )}
          >
            <span
              className={cx(
                "truncate",
                mono,
                card.accent ? "text-accent-700 dark:text-accent-300" : "text-text-tertiary",
              )}
            >
              {card.name}
            </span>
            <div className="flex items-baseline gap-1.5">
              <span className="text-[34px] leading-10 font-medium tracking-[-0.025em] text-text-primary tabular-nums">
                {card.price}
              </span>
              <span className="text-[14px] text-text-secondary">/mo</span>
            </div>
            <p className="text-[14px] leading-6 text-pretty text-text-secondary">{card.note}</p>
          </div>
        ))}
      </div>
      <div className="flex flex-col gap-8 pt-4">
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div className="flex flex-col gap-1">
            <h3 className="text-[22px] leading-7 font-medium tracking-[-0.02em] text-text-primary">
              Full helpdesk comparison across team sizes
            </h3>
            <p className="text-[15px] leading-6 text-text-secondary">
              Cheapest AI-capable helpdesk plans at list price, checked {CHECKED_ON}.
            </p>
          </div>
          <MonoLink href="/pricing">Open interactive calculator</MonoLink>
        </div>
        <CompareBars />
        <p className="max-w-[780px] text-[14px] leading-6 text-text-tertiary">
          Sources:{" "}
          {COMPETITORS.map((c, i) => (
            <span key={c.name}>
              <a href={c.source} title={c.basis} className="underline underline-offset-4 hover:text-text-secondary">
                {c.name}
              </a>
              {i < COMPETITORS.length - 1 ? ", " : ". "}
            </span>
          ))}
          Assumes {Math.round(SCENARIOS[0].aiShare * 100)}% AI involvement and {Math.round(RESOLUTION_RATE * 100)}%
          billed resolution rate on competitors. Self-hosting dimmi under MIT has $0 software fees.
        </p>
      </div>
    </Section>
  );
}

export function ComparePage() {
  return (
    <>
      <Hero />
      <CapabilityMatrix />
      <SupportCostScenario />
      <Questions
        body="Common questions from support teams switching from Helply, Chatbase, or Intercom."
        items={questions}
      />
      <FinalCta />
    </>
  );
}
