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
    body: "Describe the request in a sentence. dimmi matches what the customer asks to the live procedure that fits.",
  },
  {
    title: "Write the steps",
    body: "Three kinds: instructions, conditions that decide what happens next, and actions that run in your systems.",
  },
  {
    title: "Say when to hand over",
    body: "List the cases a person should take. The agent also hands over by itself when a conversation goes in circles.",
  },
  {
    title: "Publish",
    body: "Drafts save as you type. dimmi won't follow a procedure until it's published with nothing left to fix.",
  },
];

const rows = [
  {
    label: "In the conversation",
    title: "The agent follows your steps, in order.",
    body: "When a request matches, the agent works through the procedure step by step. The customer sees where things stand.",
    visual: <FollowFragment />,
  },
  {
    label: "Approvals",
    title: "Risky steps wait for your team.",
    body: "A refund shows up in the inbox as a card. Approve it, decline it with a reason, or take over the conversation yourself.",
    visual: <ApprovalFragment />,
  },
  {
    label: "Actions",
    title: "A step can call your own systems.",
    body: "An action step calls your API or an MCP tool. You choose who approves it and how long the request stays open.",
    visual: <GuardFragment />,
  },
];

const more = [
  {
    icon: ClipboardText,
    title: "Plain words",
    body: "No flowcharts, no code. Write it the way you'd brief a new teammate on day one.",
  },
  {
    icon: Signpost,
    title: "Picks the right one",
    body: "The agent matches the customer's request to the procedure that fits and ignores the rest.",
  },
  {
    icon: FileDashed,
    title: "Drafts stay drafts",
    body: "Edit a live procedure freely. The agent follows the published one until you publish again.",
  },
  {
    icon: ChartBar,
    title: "See how often it runs",
    body: "Each procedure shows how many conversations used it in the last 30 days.",
  },
  {
    icon: Sparkle,
    title: "Operator can draft edits",
    body: "Ask Operator to improve a procedure. Its edits wait for your review, and publishing stays with you.",
  },
  {
    icon: ArrowsClockwise,
    title: "Customers can confirm",
    body: "An action can show the customer a card spelling out what will happen before it runs.",
  },
];

const questions = [
  {
    question: "Do I have to write code?",
    answer:
      "No. Steps are plain sentences. Code only shows up if you want a step to call your own API, through an action.",
  },
  {
    question: "Does this work without the AI agent?",
    answer:
      "No. A procedure is instructions for the agent, so with the agent off, nothing follows it. For rules that run without AI, use workflows.",
  },
  {
    question: "Is the customer stuck while we approve?",
    answer: "No. The agent tells the customer it asked the team and carries on once someone decides.",
  },
  {
    question: "What if nobody approves in time?",
    answer:
      "It lapses at the time limit you set. The agent tells the customer it's taking longer, and the conversation goes to your team with high priority.",
  },
  {
    question: "Is there a cap on steps?",
    answer: "Yes. 30 steps and 10 handover rules. If you need more, it's probably two procedures.",
  },
];

export function ProceduresPage() {
  return (
    <>
      <ProductHero
        eyebrow="Procedures"
        title="Write it down once. The agent does it."
        body="A procedure is your process in plain words, like how to refund a duplicate charge. The agent follows it step by step and asks before the risky parts."
        shot="procedure"
        shotAlt="A procedure in the editor: when dimmi uses it, and five steps to refund a duplicate charge"
      />
      <HowItWorks
        label="How it works"
        title="When, what, and when to stop."
        body="A procedure has three parts: when it applies, the steps in order, and when a person takes over."
        steps={how}
        visual={(step) => <ProcedureEditorFragment step={step} />}
      />
      <Rows title="It does the work and checks with you." rows={rows} />
      <FeatureGrid title="Also worth knowing." items={more} />
      <Questions body="What you'd ask before writing your first one." items={questions} />
      <FinalCta />
    </>
  );
}
