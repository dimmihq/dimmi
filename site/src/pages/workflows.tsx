import {
  Alarm,
  ChatText,
  ClipboardText,
  Globe,
  Hash,
  Hourglass,
  Signpost,
  Sparkle,
  Ticket,
} from "@phosphor-icons/react";

import { FeatureGrid, HowItWorks, LinkList, ProductHero, Questions } from "../components/product-page";
import { WorkflowStack } from "../fragments/workflow-fragments";
import { FinalCta } from "../sections/final-cta";

const how = [
  {
    title: "Something happens",
    body: "A first message, a tag, a product event, a customer gone quiet, an SLA about to breach, or a schedule you set.",
  },
  {
    title: "Your conditions get checked",
    body: "Plan, channel, tag, SLA state, a keyword, or a question the AI answers by reading the conversation.",
  },
  {
    title: "dimmi does the work",
    body: "It routes, replies, escalates, creates a ticket, calls your system, or hands the conversation to the agent.",
  },
];

const stepKinds = [
  { icon: ChatText, title: "Send messages", body: "Replies, articles, and buttons the customer can press." },
  { icon: ClipboardText, title: "Collect details", body: "Ask what your team would ask, before a human joins." },
  { icon: Signpost, title: "Sort by topic", body: "The AI reads the message and picks the path." },
  { icon: Hash, title: "Assign and tag", body: "Send it to the right person or team, already labeled." },
  { icon: Alarm, title: "Apply SLAs", body: "Set reply targets, then act before one slips." },
  { icon: Hourglass, title: "Wait and follow up", body: "Pause for a reply. Then nudge, snooze, or close." },
  { icon: Ticket, title: "Create tickets", body: "Open a ticket without anyone clicking." },
  { icon: Globe, title: "Call your systems", body: "A step can send a request to your API." },
  { icon: Sparkle, title: "Hand to the agent", body: "dimmi answers when the path calls for it." },
];

const templates = [
  "Route billing questions to the billing team",
  "Collect details before a human joins",
  "Detect urgent conversations and notify Slack",
  "Brief the next owner when a conversation is escalated",
  "Close conversations when the customer says it's fixed",
  "Follow up after three days of silence",
  "Create a ticket for every bug report",
  "Set SLAs by plan and warn before a breach",
];

const questions = [
  {
    question: "Do I need a developer to build one?",
    answer:
      "No. Start from a template, draw it on the canvas, or describe what you want and let dimmi draft it. You only need code if a step calls your own API.",
  },
  {
    question: "Can one run on a timer, like every Monday?",
    answer:
      "Yes. Run one on a schedule: every Monday at 9:00, the first of the month, or every hour. You can also limit any workflow to office hours, or to outside them.",
  },
  {
    question: "So can I cancel Zapier?",
    answer:
      "No. Workflows are about conversations and customers. They route, reply, escalate, and call your API. If you need your CRM to update a spreadsheet, keep the tool built for that.",
  },
  {
    question: "Can it touch anything outside dimmi?",
    answer: "Yes. A step can send a request to any system with an API, then use the answer in later steps.",
  },
  {
    question: "What if no workflow matches?",
    answer:
      "Nothing breaks. The conversation carries on and lands in the inbox as usual. Every branch can have a fallback path too.",
  },
  {
    question: "How do I see what one did at 3am?",
    answer: "Read the log. Every run is recorded step by step, on the workflow and on the conversation it ran for.",
  },
];

export function WorkflowsPage() {
  return (
    <>
      <ProductHero
        eyebrow="Workflows"
        title="Automate the dull support work."
        body="A workflow waits for a trigger, checks your conditions, then acts. It routes, replies, escalates, or hands the conversation to the agent."
        shot="workflow"
        shotAlt="The workflow builder: a first message sorted by topic into billing, bug and everything else"
      />
      <HowItWorks
        label="How it works"
        title="Trigger, check, act."
        body="A workflow reads the message, checks it against your rules, then does the work."
        steps={how}
        visual={(step) => <WorkflowStack step={step} />}
      />
      <FeatureGrid
        title="The steps you get."
        body="Mix messages, logic, team actions, and your own systems on one canvas."
        items={stepKinds}
      />
      <LinkList
        title="Eight templates to start from."
        body="Pick one and it opens on the canvas, ready to edit. Or describe what you want in a sentence and dimmi drafts it."
        items={templates}
      />
      <Questions body="Worth reading before you build your first workflow." items={questions} />
      <FinalCta />
    </>
  );
}
