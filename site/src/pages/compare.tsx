import { RiArrowRightLine, RiCheckLine, RiCloseLine, RiSubtractLine } from "@remixicon/react";

import { ButtonLink } from "@/components/base/buttons/button";
import { cx } from "@/utils/cx";

import { Intro, MonoLink, Section, Tag } from "../components/container";
import { Questions } from "../components/product-page";
import { APP_URL, GITHUB_URL } from "../links";
import { CompareBars } from "../pricing/comparison";
import { CHECKED_ON, COMPETITORS, RESOLUTION_RATE, SCENARIOS } from "../pricing/competitors";
import { bill, dollars } from "../pricing/prices";
import { FinalCta } from "../sections/final-cta";

const mono = "font-mono text-[12px] leading-5 font-medium tracking-[0.04em] uppercase";

type CellStatus = "yes" | "partial" | "no";
type MatrixCell = { status: CellStatus; label: string };

const matrixRows: {
  feature: string;
  dimmi: MatrixCell;
  helply: MatrixCell;
  chatbase: MatrixCell;
  intercom: MatrixCell;
}[] = [
  {
    feature: "Unlimited human seats",
    dimmi: { status: "yes", label: "Free" },
    helply: { status: "yes", label: "Included" },
    chatbase: { status: "no", label: "Tiered" },
    intercom: { status: "no", label: "$29 to $132 / seat" },
  },
  {
    feature: "Unlimited support agents",
    dimmi: { status: "yes", label: "Unlimited" },
    helply: { status: "partial", label: "Fixed roles" },
    chatbase: { status: "partial", label: "Paid add-on" },
    intercom: { status: "partial", label: "Expert tier" },
  },
  {
    feature: "Agent-to-agent transfers",
    dimmi: { status: "yes", label: "Built in" },
    helply: { status: "partial", label: "Fixed routing" },
    chatbase: { status: "no", label: "No" },
    intercom: { status: "partial", label: "Single agent" },
  },
  {
    feature: "Shared inbox & customer history",
    dimmi: { status: "yes", label: "Native" },
    helply: { status: "yes", label: "Native" },
    chatbase: { status: "no", label: "Basic inbox" },
    intercom: { status: "yes", label: "Native" },
  },
  {
    feature: "Policy Procedures + approvals",
    dimmi: { status: "yes", label: "Included" },
    helply: { status: "partial", label: "Workflows" },
    chatbase: { status: "partial", label: "Prompts only" },
    intercom: { status: "yes", label: "Included" },
  },
  {
    feature: "Scheduled routines & PR to docs",
    dimmi: { status: "yes", label: "Inline diffs" },
    helply: { status: "partial", label: "Fixed agents" },
    chatbase: { status: "no", label: "No" },
    intercom: { status: "no", label: "No" },
  },
  {
    feature: "Simulations & 1-click rollback",
    dimmi: { status: "yes", label: "Included" },
    helply: { status: "partial", label: "Basic" },
    chatbase: { status: "partial", label: "Playground" },
    intercom: { status: "yes", label: "Included" },
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
      "If you run a 200-seat outbound call center needing power dialers and shift rosters, or if you only want a tiny 2-seat manual-only inbox with zero AI or SLAs, another tool will fit better.",
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
            <ButtonLink href={`${APP_URL}/signup`} trailingIcon={RiArrowRightLine}>
              Start free, no card
            </ButtonLink>
            <ButtonLink href={GITHUB_URL} variant="secondary">
              Self-host on GitHub
            </ButtonLink>
          </div>
        </div>
      </div>
    </section>
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
        title="Support platform and agent capabilities, side by side."
        body="Most teams stitch a standalone agent builder onto a per-seat helpdesk. dimmi puts the whole support stack in one open-source workspace."
      />
      <div className="overflow-x-auto rounded-2xl border border-border-button-default bg-background-full">
        <table className="w-full min-w-[780px] table-fixed border-collapse text-start">
          <colgroup>
            <col className="w-[34%]" />
            <col className="w-[16.5%]" />
            <col className="w-[16.5%]" />
            <col className="w-[16.5%]" />
            <col className="w-[16.5%]" />
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
            {matrixRows.map((row) => (
              <tr key={row.feature} className="transition-colors hover:bg-background-secondary-default/40">
                <th scope="row" className="whitespace-nowrap px-6 py-3.5 text-start font-medium text-text-primary">
                  {row.feature}
                </th>
                {columns.map((col) => {
                  const cell = row[col.key];
                  return (
                    <td
                      key={col.key}
                      className={cx(
                        "whitespace-nowrap px-4 py-3.5",
                        col.highlight && "border-x border-accent-500/15 bg-accent-500/[0.03]",
                      )}
                    >
                      <span className="inline-flex items-center gap-2">
                        <StatusMark status={cell.status} />
                        <span
                          className={cx(
                            "tabular-nums",
                            col.highlight ? "font-medium text-text-primary" : "text-text-secondary",
                          )}
                        >
                          {cell.label}
                        </span>
                      </span>
                    </td>
                  );
                })}
              </tr>
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
