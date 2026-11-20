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
    title: "Convert in one step",
    body: "Pick a type. The AI fills the fields it can from the conversation. You check its work, then create.",
    visual: <ConvertFragment />,
  },
  {
    title: "Pass it to another team",
    body: "Escalate to billing or engineering with an internal ticket. The customer's ticket shows who has it.",
    visual: <EscalateFragment />,
  },
  {
    title: "Customers see where it is",
    body: 'The messenger shows where their request stands, in words you choose. Expect fewer "any update?" messages.',
    visual: <CustomerStripFragment />,
  },
];

const rows = [
  {
    label: "Types and states",
    title: "Your process, not ours.",
    body: "Make a type for each kind of request, with its own fields and states. Each state gets a label customers read. Internal ones stay internal.",
    visual: <StatesFragment />,
  },
  {
    label: "SLAs",
    title: "Promises with a clock on them.",
    body: "Set first reply, next reply, and resolution targets per policy. Clocks count business hours and pause while you wait on the customer. They warn you before a breach.",
    visual: <SlaFragment />,
  },
  {
    label: "Trackers",
    title: "One problem, every customer told.",
    body: "Link everyone who reported the outage or bug to one tracker. Write the update once. Each customer gets it in their own conversation.",
    visual: <TrackerFragment />,
  },
  {
    label: "Engineering tools",
    title: "Linked to the issue, closed with it.",
    body: "Create or link issues in Linear, Jira, and GitHub, or incidents in incident.io, Rootly, and PagerDuty. When the work is done, dimmi drafts the reply. You send it.",
    visual: <IssuesFragment />,
  },
];

const more = [
  {
    icon: ListChecks,
    title: "Custom fields",
    body: "Text, numbers, choices, dates, files, teammates, and companies.",
  },
  {
    icon: Sparkle,
    title: "AI-filled fields",
    body: "Suggestions sit under each field. One button keeps them.",
  },
  {
    icon: SquaresFour,
    title: "Request forms",
    body: "Customers open a request of a type you choose, inside the messenger.",
  },
  {
    icon: Funnel,
    title: "Views per type",
    body: "Each type gets its own view in the inbox, with search and filters.",
  },
  {
    icon: ClockCounterClockwise,
    title: "Full history",
    body: "Every state change, note, and link on the ticket, in order.",
  },
  {
    icon: BracketsCurly,
    title: "API and webhooks",
    body: "Create and update tickets from your code. Hear about every change.",
  },
];

const questions = [
  {
    question: "Isn't a ticket just a conversation with a number?",
    answer:
      "No. A conversation is the thread with the customer. A ticket is the work it needs, with a type, a state, fields, and an owner. The two stay linked.",
  },
  {
    question: "How much of this do customers see?",
    answer:
      "Only what you choose. On a customer ticket, that's the label you give each state and the fields you mark visible. Internal tickets and notes never reach them.",
  },
  {
    question: "Do you link to GitLab or Asana?",
    answer:
      "No. Six tools today: Linear, Jira, and GitHub for issues, and incident.io, Rootly, and PagerDuty for incidents. If your engineers work somewhere else, issue linking won't help you yet.",
  },
  {
    question: "Will dimmi message my customers on its own when an issue closes?",
    answer: "No. It drafts the update and shows who it would reach. A teammate edits it and sends it.",
  },
  {
    question: "Can a workflow open tickets for me?",
    answer:
      "Yes. A workflow step can create a ticket, so every bug report or refund request becomes one without a click.",
  },
];

export function TicketsPage() {
  return (
    <>
      <ProductHero
        eyebrow="Tickets"
        title="Turn a conversation into tracked work."
        body="Every request gets a type, a state, and an owner. Send it to the right team, link the engineering work, and tell the customer where it stands."
        visual={<TicketsHeroFragment />}
      />
      <Steps title="Conversation to ticket, no retyping." items={steps} />
      <Rows title="For the work behind the reply." rows={rows} />
      <FeatureGrid title="Details that keep tickets moving." items={more} />
      <Questions body="Worth reading before you set up tickets." items={questions} />
      <FinalCta />
    </>
  );
}
