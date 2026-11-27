import { RiCheckLine, RiServerLine } from "@remixicon/react";

import { Intro, MonoLink, Section, Tag } from "../components/container";
import { GitHubStars } from "../components/github-stars";
import { Questions } from "../components/product-page";
import { GITHUB_URL } from "../links";
import { PriceSteps } from "../pricing/book";
import { Calculator } from "../pricing/calculator";
import { CompareBars } from "../pricing/comparison";
import { CHECKED_ON, COMPETITORS, RESOLUTION_RATE, SCENARIOS } from "../pricing/competitors";
import { bill, dollars, EMAIL_PER_THOUSAND, FREE_CONVERSATIONS } from "../pricing/prices";
import { SERVERS } from "../pricing/servers";
import { FinalCta } from "../sections/final-cta";

const mono = "font-mono text-[12px] leading-5 font-medium tracking-[0.04em] uppercase";

function Hero() {
  return (
    <section className="relative bg-background-full">
      <div className="relative mx-auto w-full max-w-[1200px] overflow-hidden border-x border-separator-border">
        <div
          aria-hidden
          className="pointer-events-none absolute inset-0 opacity-60 [background-image:linear-gradient(var(--color-separator-border)_1px,transparent_1px),linear-gradient(90deg,var(--color-separator-border)_1px,transparent_1px)] [background-size:64px_64px] [mask-image:radial-gradient(60%_50%_at_50%_30%,black,transparent)]"
        />
        <div className="relative flex flex-col items-center gap-6 px-5 pt-20 pb-16 text-center sm:px-12 md:pt-28">
          <Tag className="self-center">Pricing</Tag>
          <h1 className="max-w-3xl text-[60px] leading-[1.04] font-medium tracking-[-0.035em] text-balance text-text-primary max-md:text-[46px] max-sm:text-[36px]">
            Pay for conversations. Never for seats.
          </h1>
          <ul className="mt-1 flex flex-wrap justify-center gap-x-6 gap-y-2 text-[15px] leading-6 text-text-primary">
            {["Unlimited seats", "AI included", `First ${FREE_CONVERSATIONS} free, every month`, "No sales call"].map(
              (item) => (
                <li key={item} className="inline-flex items-center gap-1.5">
                  <RiCheckLine className="size-4 text-accent-600 dark:text-accent-400" aria-hidden />
                  {item}
                </li>
              ),
            )}
          </ul>
        </div>
        <div className="relative border-t border-separator-border px-5 py-12 sm:px-12 md:py-16">
          <Calculator />
        </div>
      </div>
    </section>
  );
}

function WhatCounts() {
  const rules = [
    { title: "One thread, one charge", body: "A conversation counts once until it stays quiet for 7 days." },
    { title: "AI or human rate", body: "It counts as AI if the dimmi agent replies at least once." },
    { title: "Long AI threads", body: "Every 10 AI replies in the same thread count as one more." },
    { title: "Messages you send first", body: "Outbound messages stay free until the customer replies." },
  ];
  return (
    <Section frameClassName="flex flex-col gap-12">
      <Intro
        title="What you pay for."
        body="Customer conversations and the Operator threads your team starts. Nothing else."
      />
      <dl className="grid grid-cols-1 gap-px overflow-hidden border border-border-button-default bg-separator-border sm:grid-cols-2 [&>*]:min-w-0">
        {rules.map((rule, i) => (
          <div key={rule.title} className="flex flex-col gap-2 bg-background-full p-7">
            <span className={`text-text-tertiary ${mono}`}>{String(i + 1).padStart(2, "0")}</span>
            <dt className="text-[18px] leading-7 font-medium tracking-[-0.015em] text-text-primary">{rule.title}</dt>
            <dd className="text-[15px] leading-6 text-text-secondary">{rule.body}</dd>
          </div>
        ))}
      </dl>
    </Section>
  );
}

function PriceBook() {
  return (
    <Section frameClassName="grid grid-cols-1 gap-12 lg:grid-cols-[0.8fr_1.2fr] [&>*]:min-w-0">
      <div className="flex flex-col gap-4">
        <h2 className="text-[40px] leading-[1.12] font-medium tracking-[-0.025em] text-text-primary max-sm:text-[30px]">
          The price book.
        </h2>
        <p className="max-w-[340px] text-[16px] leading-7 text-text-secondary">
          Like tax brackets, except the rate goes down as you grow. Past 10,000 a month, we agree a price with you.
        </p>
      </div>
      <div className="flex flex-col gap-8">
        <PriceSteps />
        <dl className="grid grid-cols-1 gap-x-8 gap-y-4 sm:grid-cols-3">
          {[
            { label: "Seats", value: "free, unlimited" },
            { label: "Help center & changelogs", value: "free" },
            { label: "Broadcast & series emails", value: `${dollars(EMAIL_PER_THOUSAND)} per 1,000` },
          ].map((item) => (
            <div key={item.label} className="flex flex-col justify-between gap-0.5">
              <dt className="text-[14px] leading-5 text-text-secondary">{item.label}</dt>
              <dd className="text-[16px] leading-7 text-text-primary">{item.value}</dd>
            </div>
          ))}
        </dl>
      </div>
    </Section>
  );
}

/*
 * The three biggest ways to shrink a dimmi bill, in one place. The figures are
 * worked out from the price book, so they move with it.
 */
function LessMoney() {
  const example = 5000;
  const included = bill({ conversations: example, aiShare: 1, operator: 0, ownModel: false }).total;
  const own = bill({ conversations: example, aiShare: 1, operator: 0, ownModel: true }).total;
  const ways = [
    {
      title: "Bring your own model keys",
      body: "Use Anthropic, OpenAI, or your ChatGPT plan. AI threads drop to the human rate, saving 50% to 83%.",
      note: `${example.toLocaleString("en-US")} AI conversations: ${dollars(own)} instead of ${dollars(included)}`,
    },
    {
      title: "Skip Operator, use MCP",
      body: "Operator threads count as AI conversations. The AI assistant you already pay for over MCP costs $0.",
      note: "MCP calls: $0",
    },
    {
      title: "Self-host it",
      body: "Same full code under MIT on your own server. We get a GitHub star if you feel generous.",
      note: "Paid to dimmi: $0",
      link: { href: "#self-host", label: "See what a server costs" },
      githubStars: true,
    },
  ];
  return (
    <Section frameClassName="flex flex-col gap-12">
      <Intro title="How to give us even less money." body="Most pricing pages hide this part." />
      <ol className="grid grid-cols-1 gap-px overflow-hidden rounded-xl border border-border-button-default bg-separator-border lg:grid-cols-3">
        {ways.map((way, i) => (
          <li key={way.title} className="flex flex-col gap-3 bg-background-full p-7">
            <span className={`text-text-tertiary ${mono}`}>{String(i + 1).padStart(2, "0")}</span>
            <h3 className="text-[20px] leading-7 font-medium tracking-[-0.02em] text-text-primary">{way.title}</h3>
            <p className="text-[15px] leading-6 text-pretty text-text-secondary">{way.body}</p>
            <span className="mt-auto flex flex-col gap-2 pt-3">
              <span className="text-[14px] leading-5 font-medium text-accent-700 tabular-nums dark:text-accent-300">
                {way.note}
              </span>
              {way.link ? <MonoLink href={way.link.href}>{way.link.label}</MonoLink> : null}
              {way.githubStars ? <GitHubStars location="pricing" size="small" className="mt-1 self-start" /> : null}
            </span>
          </li>
        ))}
      </ol>
    </Section>
  );
}

function Comparison() {
  return (
    <Section frameClassName="flex flex-col gap-10">
      <Intro title="What the same support costs elsewhere." body="They're good products. They also charge per seat." />
      <CompareBars />
      <p className="max-w-[760px] text-[14px] leading-6 text-text-tertiary">
        Each vendor's cheapest plan with an AI agent, at list price, checked {CHECKED_ON}:{" "}
        {COMPETITORS.map((c, i) => (
          <span key={c.name}>
            <a href={c.source} title={c.basis} className="underline underline-offset-4 hover:text-text-secondary">
              {c.name}
            </a>
            {i < COMPETITORS.length - 1 ? ", " : ". "}
          </span>
        ))}
        The AI answers {Math.round(SCENARIOS[0].aiShare * 100)}% of conversations and theirs resolves{" "}
        {Math.round(RESOLUTION_RATE * 100)}% of those. Side-by-side:{" "}
        <a href="/vs/intercom" className="underline underline-offset-4 hover:text-text-secondary">
          Intercom
        </a>
        ,{" "}
        <a href="/vs/chatwoot" className="underline underline-offset-4 hover:text-text-secondary">
          Chatwoot
        </a>
        ,{" "}
        <a href="/vs/front" className="underline underline-offset-4 hover:text-text-secondary">
          Front
        </a>
        ,{" "}
        <a href="/vs/freshdesk" className="underline underline-offset-4 hover:text-text-secondary">
          Freshdesk
        </a>
        , or{" "}
        <a href="/vs" className="underline underline-offset-4 hover:text-text-secondary">
          all comparisons
        </a>
        .
      </p>
    </Section>
  );
}

function SelfHost() {
  return (
    <Section anchor="self-host" frameClassName="flex flex-col gap-12">
      <div className="grid grid-cols-1 gap-x-16 gap-y-6 lg:grid-cols-[1fr_minmax(0,460px)] [&>*]:min-w-0">
        <div className="flex flex-col gap-4">
          <Tag>Self-host</Tag>
          <h2 className="max-w-[560px] text-[40px] leading-[1.12] font-medium tracking-[-0.025em] text-balance text-text-primary max-sm:text-[30px]">
            Run it yourself. We take nothing.
          </h2>
        </div>
        <div className="flex flex-col gap-4 lg:pt-11">
          <p className="text-[16px] leading-7 text-pretty text-text-primary">
            You pay your server, your AI provider, and your email provider. We are not in that sentence.
          </p>
          <MonoLink href={GITHUB_URL}>Install from GitHub</MonoLink>
        </div>
      </div>
      <div className="grid grid-cols-1 gap-6 md:grid-cols-3 [&>*]:min-w-0">
        {SERVERS.map((server) => (
          <div
            key={server.name}
            className="flex flex-col gap-5 rounded-xl border border-border-button-default bg-background-full p-7"
          >
            <div className="flex items-center justify-between gap-3">
              <span className={`text-text-tertiary ${mono}`}>{server.name}</span>
              <RiServerLine className="size-5 text-foreground-icon-tertiary" aria-hidden />
            </div>
            <div className="flex flex-col gap-1">
              <span className="text-[30px] leading-9 font-medium tracking-[-0.02em] text-text-primary tabular-nums">
                {server.price}
                <span className="text-[16px] font-normal text-text-secondary"> a month</span>
              </span>
              <span className="text-[14px] leading-5 text-text-secondary">{server.where}</span>
            </div>
            <dl className="flex flex-col divide-y divide-separator-border border-y border-separator-border text-[15px] leading-6">
              {server.specs.map(([label, value]) => (
                <div key={label} className="flex justify-between gap-4 py-2">
                  <dt className="text-text-secondary">{label}</dt>
                  <dd className="text-end text-text-primary">{value}</dd>
                </div>
              ))}
            </dl>
          </div>
        ))}
      </div>
    </Section>
  );
}

const questions = [
  {
    question: "Then how do you make money?",
    answer: "From conversations on dimmi Cloud. If you self-host, you pay us nothing and we're fine with it.",
  },
  {
    question: "Can my bill surprise me?",
    answer: "Not if you set a spend cap. We alert you before you reach it. A month under $10 isn't charged at all.",
  },
  {
    question: "Is dimmi ever the more expensive one?",
    answer:
      "Sometimes, on paper. Two people answering 2,000 conversations by hand pay $60 on dimmi and $38 on the cheapest entry plan. That plan has no custom reports or multiple SLA policies. On plans with those, they pay $90 or more.",
  },
  {
    question: "Do WhatsApp, SMS, and phone calls cost extra?",
    answer: "Not from us. You connect your own carrier account and they bill you for messages and minutes.",
  },
  {
    question: "When do I need a card?",
    answer: `At conversation ${FREE_CONVERSATIONS + 1} in a month, or before your first email broadcast. It isn't a trial.`,
  },
];

export function PricingPage() {
  return (
    <>
      <Hero />
      <WhatCounts />
      <PriceBook />
      <LessMoney />
      <Comparison />
      <SelfHost />
      <Questions items={questions} />
      <FinalCta />
    </>
  );
}
