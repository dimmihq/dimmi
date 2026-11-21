import { BracketsCurly, ClockCounterClockwise, Funnel, ListChecks, Sparkle, SquaresFour } from "@phosphor-icons/react";

import { FeatureGrid, ProductHero, Questions, Rows, Steps } from "../components/product-page";
import {
  ConvertFragment,
  CustomerStripFragment,
  EscalateFragment,
  IssuesFragment,
  SlaFragment,
  StatesFragment,
  TicketsHeroFragment,
  TrackerFragment,
} from "../fragments/tickets-fragments";
import { FinalCta } from "../sections/final-cta";

const steps = [
  {
    title: "Convert in one click",
    body: "Pick a ticket type and let AI fill fields from the chat. Check it and create.",
    visual: <ConvertFragment />,
  },
  {
    title: "Escalate without losing track",
    body: "Open an internal ticket for billing or engineering while keeping the customer thread linked.",
    visual: <EscalateFragment />,
  },
  {
    title: "Show customers where things stand",
    body: 'Display live status labels right inside the messenger so you get fewer "any update?" pings.',
    visual: <CustomerStripFragment />,
  },
];

const rows = [
  {
    label: "Types and states",
    title: "Match your own workflow.",
    body: "Create ticket types for bugs, billing, or onboarding with custom fields and states. Customers see the labels you choose while internal states stay private.",
    visual: <StatesFragment />,
  },
  {
    label: "SLAs",
    title: "Reply promises with a live clock.",
    body: "Set first reply, next reply, and resolution targets by policy. Clocks respect your office hours, pause while waiting on the customer, and warn before a breach.",
    visual: <SlaFragment />,
  },
  {
    label: "Trackers",
    title: "Update every affected customer at once.",
    body: "Link every chat hit by the same bug or outage to one tracker. Write one update and dimmi sends it to every customer in their own thread.",
    visual: <TrackerFragment />,
  },
  {
    label: "Engineering tools",
    title: "Linked to your engineering tools.",
    body: "Connect issues in Linear, Jira, GitHub, or incidents in incident.io, Rootly, PagerDuty, and more. When the work closes, dimmi drafts the update for you to send.",
    visual: <IssuesFragment />,
  },
];

const more = [
  {
    icon: ListChecks,
    title: "Custom fields",
    body: "Add text, numbers, choices, dates, files, teammates, or companies to any type.",
  },
  {
    icon: Sparkle,
    title: "AI field suggestions",
    body: "dimmi suggests values from the conversation so teammates confirm instead of typing.",
  },
  {
    icon: SquaresFour,
    title: "Request forms",
    body: "Let customers open structured tickets straight from the messenger.",
  },
  {
    icon: Funnel,
    title: "Views per type",
    body: "Filter, search, and manage each ticket type in its own inbox view.",
  },
  {
    icon: ClockCounterClockwise,
    title: "Full timeline",
    body: "Every state change, internal note, and linked issue stays in one audit trail.",
  },
  {
    icon: BracketsCurly,
    title: "API and webhooks",
    body: "Create and update tickets from your code, or react to changes via webhooks.",
  },
];

const questions = [
  {
    question: "How is a ticket different from a conversation?",
    answer:
      "A conversation is the chat thread with the customer. A ticket is the tracked work behind it, with a type, state, fields, and owner that stays linked to the chat.",
  },
  {
    question: "What do customers see on a ticket?",
    answer:
      "Only what you mark visible: the customer label for each state and the fields you expose. Internal tickets and team notes stay hidden.",
  },
  {
    question: "Can tickets replace Linear or Jira for engineering?",
    answer:
      "Keep your engineering tracker for sprints and code. dimmi tickets track the customer side and link straight to your engineering issues so both teams stay in sync.",
  },
  {
    question: "What if our engineers use another tracker?",
    answer:
      "Linear, Jira, GitHub, incident.io, Rootly, and PagerDuty work out of the box, and we are always adding new connectors. Need another tool? Open a feature request and we will ship it as soon as we can, or add your own since dimmi is open source.",
  },
  {
    question: "Does dimmi email customers automatically when a Linear issue closes?",
    answer:
      "Never without your eyes on it. dimmi drafts the update and shows who will get it so a teammate edits and clicks send.",
  },
  {
    question: "Can a workflow create tickets for me?",
    answer:
      "Yes. Add a ticket step to any workflow so bug reports or refund requests open structured tickets on arrival.",
  },
];

export function TicketsPage() {
  return (
    <>
      <ProductHero
        eyebrow="Tickets"
        title="Stop losing bug reports inside long chat threads."
        body="Turn any chat into tracked work with custom states, SLAs, and links to Linear, Jira, GitHub, and more."
        visual={<TicketsHeroFragment />}
      />
      <Steps title="From chat to tracked work, with no retyping." items={steps} />
      <Rows title="Built for the work behind the reply." rows={rows} />
      <FeatureGrid title="Details that keep tickets moving." items={more} />
      <Questions body="What teams ask before setting up tickets." items={questions} />
      <FinalCta />
    </>
  );
}
