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
    title: "Start from an event",
    body: "Trigger on a first message, tag, app event, quiet thread, SLA alert, or timer.",
  },
  {
    title: "Check your rules",
    body: "Branch on plan, channel, tag, SLA state, keywords, or an AI topic check.",
  },
  {
    title: "Take action",
    body: "Route to a team, reply, open a ticket, call your API, or hand to the dimmi agent.",
  },
];

const stepKinds = [
  { icon: ChatText, title: "Send messages", body: "Send replies, help articles, or quick reply buttons." },
  { icon: ClipboardText, title: "Collect details", body: "Gather order IDs or screenshots before a teammate joins." },
  { icon: Signpost, title: "Sort by topic", body: "Let AI read the message and pick the right branch." },
  { icon: Hash, title: "Assign and tag", body: "Route to the right person or team with tags already added." },
  { icon: Alarm, title: "Apply SLAs", body: "Set reply targets and alert your team before a breach." },
  { icon: Hourglass, title: "Wait and follow up", body: "Pause for a reply, then nudge, snooze, or close the chat." },
  { icon: Ticket, title: "Create tickets", body: "Turn bug reports or requests into tickets with zero clicks." },
  { icon: Globe, title: "Call your API", body: "Send HTTP requests to your backend and use the response." },
  { icon: Sparkle, title: "Hand to the AI agent", body: "Let the dimmi agent take over when a path calls for AI." },
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
    question: "Do I need a developer to build a workflow?",
    answer:
      "Not unless a step calls your own API. Pick a template, drag steps on the canvas, or describe what you want in plain English and let dimmi draft it.",
  },
  {
    question: "Can workflows run on a schedule or office hours?",
    answer:
      "Yes. Run them every Monday at 9:00, every hour, or only inside or outside your workspace office hours.",
  },
  {
    question: "So can I cancel Zapier?",
    answer:
      "Probably not, unless you only used Zapier for helpdesk rules. Workflows route chats, trigger SLAs, and call your API, but keep Zapier for syncing your CRM to spreadsheets.",
  },
  {
    question: "Can a workflow talk to our own backend?",
    answer:
      "Yes. Add an HTTP request step to call any API with auth headers, then branch later steps on the JSON it returns.",
  },
  {
    question: "What happens if no workflow matches a chat?",
    answer:
      "Nothing breaks. The conversation lands in your inbox like normal, and every branch on the canvas supports a fallback path.",
  },
  {
    question: "How do I debug what ran at 3am?",
    answer:
      "Open the run log on the workflow or right on the conversation. Every step and branch choice is recorded in order.",
  },
];

export function WorkflowsPage() {
  return (
    <>
      <ProductHero
        eyebrow="Workflows"
        title="Automate routing, SLAs, and follow-ups on every plan."
        body="Set a trigger, check your rules, and let dimmi sort, reply, open tickets, or call your API. No enterprise upgrade required."
        shot="workflow"
        shotAlt="The workflow builder: a first message sorted by topic into billing, bug and everything else"
      />
      <HowItWorks
        label="How it works"
        title="Wait for an event, check rules, take action."
        body="Every run logs each step in order so you can trace what fired at 3am."
        steps={how}
        visual={(step) => <WorkflowStack step={step} />}
      />
      <FeatureGrid
        title="Everything you can drop on the canvas."
        body="Mix messages, logic, team routing, and API calls in one visual flow."
        items={stepKinds}
      />
      <LinkList
        title="Start from a template, or describe what you want."
        body="Open a template on the canvas to tweak it, or write one sentence and let dimmi draft the flow."
        items={templates}
      />
      <Questions body="What teams ask before building their first workflow." items={questions} />
      <FinalCta />
    </>
  );
}
