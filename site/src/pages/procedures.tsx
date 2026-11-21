import { ArrowsClockwise, ChartBar, ClipboardText, FileDashed, Signpost, Sparkle } from "@phosphor-icons/react";

import { FeatureGrid, HowItWorks, ProductHero, Questions, Rows } from "../components/product-page";
import {
  ApprovalFragment,
  FollowFragment,
  GuardFragment,
  ProcedureEditorFragment,
} from "../fragments/procedure-fragments";
import { FinalCta } from "../sections/final-cta";

const how = [
  {
    title: "Say when it applies",
    body: "Describe the customer request in one sentence so the agent knows when to pull up this playbook.",
  },
  {
    title: "List the steps in order",
    body: "Mix plain instructions, conditions to branch on, and API actions that talk to your backend.",
  },
  {
    title: "Draw the handover line",
    body: "Name the cases where a human must step in. The agent also hands off if a chat goes in circles.",
  },
  {
    title: "Publish when ready",
    body: "Edits save as private drafts while live chats keep using your published copy until you publish.",
  },
];

const rows = [
  {
    label: "In the chat",
    title: "Runs your steps in order.",
    body: "When a chat matches, the agent follows your playbook from top to bottom instead of guessing what your policy says.",
    visual: <FollowFragment />,
  },
  {
    label: "Approvals",
    title: "Risky steps wait for your team.",
    body: "Actions like refunds land in the inbox as an approval card. Approve, decline with a note, or take over the chat.",
    visual: <ApprovalFragment />,
  },
  {
    label: "Actions",
    title: "Talks to your own backend.",
    body: "Connect any step to your REST API or MCP server. You choose who approves changes and how long requests stay open.",
    visual: <GuardFragment />,
  },
];

const more = [
  {
    icon: ClipboardText,
    title: "Plain English",
    body: "No fragile flowcharts. Write each playbook the way you brief a new hire.",
  },
  {
    icon: Signpost,
    title: "Picks the right playbook",
    body: "The agent matches the customer's question to the right procedure and ignores the rest.",
  },
  {
    icon: FileDashed,
    title: "Safe drafts",
    body: "Edit a live procedure anytime without changing what customers see until you publish.",
  },
  {
    icon: ChartBar,
    title: "Usage counts",
    body: "See how many chats followed each procedure over the last 30 days.",
  },
  {
    icon: Sparkle,
    title: "Draft edits with Operator",
    body: "Ask Operator to tighten a playbook. Its edits wait for your review before going live.",
  },
  {
    icon: ArrowsClockwise,
    title: "Customer confirmation",
    body: "Show customers a confirmation card spelling out what will change before an action runs.",
  },
];

const questions = [
  {
    question: "Do I need an engineer to write a procedure?",
    answer:
      "Not for the steps themselves. You write instructions and conditions in plain English. An engineer only helps if you want a step to call your own API.",
  },
  {
    question: "Should I use a procedure or a workflow?",
    answer:
      "Use a procedure when the AI agent needs to talk with a customer and use judgment. Use a workflow for fixed routing, tagging, or SLA rules that run without AI.",
  },
  {
    question: "What happens while the customer waits for approval?",
    answer:
      "The agent tells the customer it checked with your team, then finishes the steps once someone approves or declines the card.",
  },
  {
    question: "What if nobody approves the card in time?",
    answer:
      "The request expires at the limit you set. The agent updates the customer and routes the chat to your team with high priority.",
  },
  {
    question: "Does this work if I turn the AI agent off?",
    answer:
      "Procedures are playbooks for the dimmi agent, so nothing runs them while the agent is off. If you want automation without AI, use workflows instead.",
  },
];

export function ProceduresPage() {
  return (
    <>
      <ProductHero
        eyebrow="Procedures"
        title="Write how your team handles a request. The AI follows it."
        body="Turn policies like refunding a duplicate charge into clear steps in plain English. The agent runs them and asks before risky actions."
        shot="procedure"
        shotAlt="A procedure in the editor: when dimmi uses it, and five steps to refund a duplicate charge"
      />
      <HowItWorks
        label="How it works"
        title="Say when, list the steps, set the stop line."
        body="Write each playbook the way you would brief a smart new hire on day one."
        steps={how}
        visual={(step) => <ProcedureEditorFragment step={step} />}
      />
      <Rows title="Real work, with your team in charge." rows={rows} />
      <FeatureGrid title="Built to stay easy to manage." items={more} />
      <Questions body="What teams ask before writing their first playbook." items={questions} />
      <FinalCta />
    </>
  );
}
