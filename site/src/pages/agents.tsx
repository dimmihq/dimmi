import {
  Buildings,
  ChartLineUp,
  ChatsCircle,
  Code,
  Compass,
  CreditCard,
  CurrencyDollar,
  Headset,
  Package,
  Phone,
  Sliders,
  Sparkle,
  Translate,
} from "@phosphor-icons/react";

import { Intro, MonoLink, Section } from "../components/container";
import { FeatureGrid, ProductHero, Questions, Rows } from "../components/product-page";
import { ShotStill } from "../components/shot-still";
import { ShotVideo } from "../components/shot-video";
import { ModelFragment } from "../fragments/agent-fragments";
import { AgentDemoCard, AgentStill } from "../scenes/agent-demo";
import { useAgentStories } from "../scenes/use-agent-stories";
import { FinalCta } from "../sections/final-cta";

const specialists = [
  {
    icon: Headset,
    name: "Frontline support",
    surface: "App, email, Slack & phone",
    body: "Resolves everyday product questions from your help center and opens tracked tickets when engineering needs to step in.",
  },
  {
    icon: CreditCard,
    name: "Billing & refunds",
    surface: "Billing inbox & signed-in chat",
    body: "Checks verified accounts, enforces refund caps and time windows, and pauses credits for teammate approval.",
  },
  {
    icon: Code,
    name: "Technical & docs",
    surface: "Docs site & Discord",
    body: "Answers developer questions with snippets from your docs and API reference without inventing flags or endpoints.",
  },
  {
    icon: Package,
    name: "Orders & returns",
    surface: "Storefront, WhatsApp & Instagram",
    body: "Looks up parcel status first and enforces return windows across every storefront channel.",
  },
];

const moreTemplates = [
  {
    icon: Buildings,
    name: "Multi-brand support",
    body: "Separate help centers, tone, and widgets per brand in one workspace.",
  },
  {
    icon: Compass,
    name: "Onboarding guidance",
    body: "Walks new signups through setup steps inside your app.",
  },
  {
    icon: ChatsCircle,
    name: "Pre-sales questions",
    body: "Answers pricing and plan questions on your site from public docs.",
  },
  {
    icon: Sparkle,
    name: "Start from scratch",
    body: "Build a custom support agent for any product, region, or team.",
  },
];

const rows = [
  {
    label: "Grounded answers",
    title: "Every reply cites its source, and stops when docs run out.",
    body: "Answers link straight to the help article or doc the agent read. When your docs do not cover a question, it hands the thread to your team instead of guessing.",
    visual: <AgentStill story={0} />,
  },
  {
    label: "Strict boundaries & transfers",
    title: "Keep refund actions away from general chat, and transfer threads quietly.",
    body: "Lock each agent to its own help centers, procedures, and actions. When a billing dispute lands in general support, the agent hands it quietly to your Billing specialist with full context.",
    visual: (
      <ShotStill
        name="agent-studio-guidance"
        alt="Agent Studio Build settings locking an agent to its own knowledge sources, procedures, actions, and peer transfer targets"
      />
    ),
  },
  {
    label: "Procedures & sign-off",
    title: "Follows your refund and account policies with human approval.",
    body: "Turn multi-step support playbooks into ordered rules. dimmi checks hard limits automatically and pauses for teammate sign-off before anything you cannot undo.",
    visual: (
      <ShotStill
        name="procedure"
        alt="A duplicate-charge refund policy in dimmi showing required account checks, a $500 refund cap, and automatic Billing handover"
      />
    ),
  },
  {
    label: "Agent Studio",
    title: "Catch flaky replies and compare Draft against Live before publishing.",
    body: "Test edits in Preview, run multi-run simulations, compare Draft against Live side by side, and ask Operator to diagnose any failing test inside Agent Studio.",
    visual: (
      <ShotVideo
        name="agent-studio-compare"
        poster="agent-studio-compare"
        alt="Comparing Draft against Live side by side in Agent Studio before publishing"
        maxWidth={560}
      />
    ),
  },
  {
    label: "Clean handovers",
    title: "Steps aside with a private summary when a human should take over.",
    body: "When a customer asks for a person or hits a handover rule, the agent routes the thread to the right team with a concise internal note and stays quiet.",
    visual: <AgentStill story={2} />,
  },
  {
    label: "Your model keys",
    title: "Bring your own model keys and save 50% to 83% on dimmi AI fees.",
    body: "Connect Anthropic, OpenAI, or any OpenAI-compatible provider directly and pay your model provider yourself.",
    visual: <ModelFragment />,
  },
];

const more = [
  {
    icon: ChatsCircle,
    title: "Every inbox channel",
    body: "Answers across your app messenger, email, Slack, Teams, Discord, WhatsApp, SMS, and API on the hours you choose.",
  },
  {
    icon: Phone,
    title: "Answers the phone too",
    body: "Picks up calls using the same docs and policies, then transfers live calls to your team with a written summary.",
  },
  {
    icon: ChartLineUp,
    title: "Side-by-side agent reports",
    body: "Compare resolution rates, handovers, ratings, and monthly spend across every support agent in one table.",
  },
  {
    icon: Sliders,
    title: "Per-agent models and caps",
    body: "Use a stronger reasoning model for billing disputes and a faster model for everyday FAQs, with optional spend caps.",
  },
  {
    icon: Translate,
    title: "Speaks your customers' language",
    body: "Write your help center in one language and your agents reply naturally in whatever language the customer writes.",
  },
  {
    icon: CurrencyDollar,
    title: "Unlimited agents on every plan",
    body: "Zero per-agent fees and zero $132/seat multi-brand paywalls. First 100 conversations are free every month.",
  },
];

const questions = [
  {
    question: "Can we start with just one Support agent?",
    answer:
      "Start with a single frontline support agent on day one. Add dedicated agents for billing, technical docs, orders, or extra brands whenever your queue grows.",
  },
  {
    question: "How many support agents can we create?",
    answer:
      "As many as you need, even on the 100-conversation free tier. You pay per conversation on dimmi Cloud, never per agent and never per seat.",
  },
  {
    question: "What happens if a customer writes to the wrong agent?",
    answer:
      "Pick which peer agents each specialist can transfer to. When a billing dispute or technical bug hits general support, the agent transfers the thread quietly with full context for your team.",
  },
  {
    question: "What if a teammate jumps into the conversation?",
    answer:
      "By default, the agent steps back while an assigned teammate is online or replied in the last 24 hours. You can also set it to stop the moment anyone takes the thread.",
  },
  {
    question: "Will an agent ever make up a policy or invent an answer?",
    answer:
      "Any model can miss an edge case, and vendors claiming 100% perfection are lying. That is why dimmi requires real doc sources, enforces hard policy rules, pauses risky actions for approval, and lets you test changes first.",
  },
  {
    question: "Do you charge $0.99 per AI resolution like Intercom?",
    answer:
      "AI is included in our standard conversation pricing, with 100 free conversations each month. Bring your own OpenAI or Anthropic model keys and save 50% to 83% on dimmi AI fees.",
  },
];

function HeroDemo() {
  const stories = useAgentStories();
  return <AgentDemoCard stories={stories} className="max-w-[640px]" />;
}

function SupportSpecialists() {
  return (
    <Section frameClassName="flex flex-col gap-12">
      <Intro
        title="One inbox. Dedicated support agents for every need."
        body="Give each part of customer support its own docs, tone, policies, and channels instead of stuffing every rule into one prompt."
      />
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4 [&>*]:min-w-0">
        {specialists.map((item) => (
          <div
            key={item.name}
            className="flex flex-col gap-4 rounded-xl border border-border-button-default bg-background-full p-6"
          >
            <span className="flex size-10 items-center justify-center rounded-xl bg-accent-500/10 ring-1 ring-accent-500/15 ring-inset">
              <item.icon weight="duotone" className="size-5 text-accent-600 dark:text-accent-400" aria-hidden />
            </span>
            <div className="flex flex-col gap-1.5">
              <h3 className="text-[18px] leading-6 font-medium tracking-[-0.02em] text-text-primary">{item.name}</h3>
              <p className="text-[14px] leading-6 text-pretty text-text-secondary">{item.body}</p>
            </div>
            <span className="mt-auto border-t border-separator-border pt-3.5 font-mono text-[11px] tracking-[0.04em] text-text-tertiary uppercase">
              {item.surface}
            </span>
          </div>
        ))}
      </div>
      <div className="overflow-hidden rounded-2xl border border-border-button-default bg-background-full">
        <div className="flex flex-wrap items-center justify-between gap-4 border-b border-separator-border px-6 py-4">
          <div className="flex flex-col gap-0.5">
            <span className="text-[15px] leading-6 font-medium text-text-primary">More starter templates</span>
            <span className="text-[14px] leading-5 text-text-secondary">
              Launch additional specialists from a template or start from scratch.
            </span>
          </div>
          <div className="flex flex-wrap items-center gap-5">
            <MonoLink href="/agent-studio">Explore Agent Studio</MonoLink>
            <MonoLink href="/compare">See how dimmi compares</MonoLink>
          </div>
        </div>
        <div className="grid grid-cols-1 divide-y divide-separator-border sm:grid-cols-2 sm:divide-y-0 lg:grid-cols-4 lg:divide-x">
          {moreTemplates.map((template) => (
            <div key={template.name} className="flex flex-col gap-2 px-6 py-5">
              <div className="flex items-center gap-2.5">
                <template.icon
                  weight="duotone"
                  className="size-5 shrink-0 text-accent-600 dark:text-accent-400"
                  aria-hidden
                />
                <span className="text-[15px] leading-6 font-medium text-text-primary">{template.name}</span>
              </div>
              <p className="text-[14px] leading-5 text-pretty text-text-secondary">{template.body}</p>
            </div>
          ))}
        </div>
      </div>
    </Section>
  );
}

export function AgentsPage() {
  return (
    <>
      <ProductHero
        eyebrow="Agents"
        title="Dedicated support agents for every customer need."
        body="Resolve questions faster without guessing. Run one frontline AI support agent or build dedicated specialists for billing, docs, orders, and brands in one shared inbox."
        visual={<HeroDemo />}
      />
      <SupportSpecialists />
      <Rows title="Built for support teams who refuse to lose control." rows={rows} />
      <FeatureGrid title="Everything a modern support team needs in one place." items={more} />
      <Questions body="What support leads ask before turning on specialized agents." items={questions} />
      <FinalCta />
    </>
  );
}
