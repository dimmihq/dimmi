import {
  ChatCircleText,
  ClockCounterClockwise,
  GitBranch,
  HandWaving,
  Scales,
  ShieldCheck,
} from "@phosphor-icons/react";

import { FeatureGrid, HowItWorks, ProductHero, Questions, Rows } from "../components/product-page";
import { ShotStill } from "../components/shot-still";
import { ApprovalFragment, FollowFragment, ProcedureEditorFragment } from "../fragments/procedure-fragments";
import { FinalCta } from "../sections/final-cta";

const how = [
  {
    title: "Pick the request it handles",
    body: "Describe the customer problem, such as a duplicate charge or damaged order, so the agent knows when this policy applies.",
  },
  {
    title: "Spell out every step",
    body: "Write what the agent should ask, check in your systems, and say next, including when to tag a team or open a ticket.",
  },
  {
    title: "Set hard policy limits",
    body: "Put firm rules on refund caps, plan tiers, and time windows so the agent cannot bend policy or skip a required check.",
  },
  {
    title: "Test before customers see it",
    body: "Try your draft on real customer scenarios and confirm every check passes before publishing to your live queue.",
  },
];

const rows = [
  {
    label: "Zero policy drift",
    title: "Your agent follows every step and cannot bend the rules.",
    body: "Most support bots cave when a customer pushes back or skip a check three messages in. dimmi enforces your order and limits automatically, so an out-of-policy refund never goes through.",
    visual: <FollowFragment />,
  },
  {
    label: "Safe launch",
    title: "Test every policy change before a customer ever sees it.",
    body: "Practice against real past conversations or made-up customers without sending a message or touching live accounts. Save tricky scenarios and replay them automatically so a quick edit never breaks what already works.",
    visual: (
      <ShotStill
        name="procedure-simulations"
        alt="A saved customer test scenario showing the conversation and every passed policy check before publishing"
      />
    ),
  },
  {
    label: "Human sign-off",
    title: "Keep your team in charge when money or accounts move.",
    body: "Pause high-stakes steps for a teammate's approval right inside the inbox, or ask the customer to confirm first. Approve in one click, decline, or take over.",
    visual: <ApprovalFragment />,
  },
  {
    label: "Continuous QA",
    title: "Catch slipping policies early and roll back in one click.",
    body: "Track resolution rates, handovers, and CSAT for every policy. Compare changes across versions, roll back any edit in seconds, and let Operator flag weekly failures with suggested fixes ready to review.",
    visual: (
      <ShotStill
        name="procedure-history"
        alt="Policy version history showing published versions and a step-by-step comparison of what changed"
      />
    ),
  },
];

const more = [
  {
    icon: ChatCircleText,
    title: "One question at a time",
    body: "The agent waits for each customer answer before moving on, and skips questions the customer already answered.",
  },
  {
    icon: GitBranch,
    title: "Reuse shared checks",
    body: "Write steps like verifying an account once, then drop them into every billing, cancellation, or return policy.",
  },
  {
    icon: ShieldCheck,
    title: "Zero risk while testing",
    body: "Test runs never email customers, issue real refunds, or touch your live billing and order systems.",
  },
  {
    icon: Scales,
    title: "Unbiased quality checks",
    body: "A separate checker scores your test conversations against your standards, so the agent never grades its own work.",
  },
  {
    icon: ClockCounterClockwise,
    title: "Instant version rollback",
    body: "Every publish keeps a full version history. Compare what changed step by step and restore an older policy anytime.",
  },
  {
    icon: HandWaving,
    title: "Clean handovers with context",
    body: "When a chat hits a sensitive rule like a bank dispute, the agent routes it with a clear team summary.",
  },
];

const questions = [
  {
    question: "Can a customer talk the AI into breaking our refund policy?",
    answer:
      "No. Hard rules like refund limits and time windows are enforced outside the AI's reply, and dimmi blocks the agent from issuing a refund before required account checks succeed.",
  },
  {
    question: "Will this feel robotic or trap customers in a rigid script?",
    answer:
      "Quite the opposite. Customers write naturally in any language, the agent asks one follow-up at a time, and if a customer already gave their order number in message one, it never asks twice.",
  },
  {
    question: "How do I know a policy edit will not hurt CSAT?",
    answer:
      "Edits stay in a private draft until you publish. Run your saved customer test scenarios right beside the Publish button, check every pass, and if live numbers dip later, roll back to the previous version in one click.",
  },
  {
    question: "Can my support leads manage policies without waiting on engineering?",
    answer:
      "Yes. Engineering only connects your billing or order actions once. From there, support leads write policies, set limits, test changes, monitor resolution rates, and publish on their own.",
  },
  {
    question: "What happens when a customer gets upset or threatens a chargeback?",
    answer:
      "You set clear handover triggers on every policy, such as chargebacks, high-value refunds, or older charges. The moment one hits, the agent stops replying and routes the thread to your team with a private summary.",
  },
  {
    question: "When should we use a workflow instead of a procedure?",
    answer:
      "Use a workflow for background rules that never need a back-and-forth chat, such as routing, tagging, or SLA alerts. Use a procedure when the AI agent needs to talk with the customer and resolve a multi-step request.",
  },
];

export function ProceduresPage() {
  return (
    <>
      <ProductHero
        eyebrow="Procedures"
        title="Support policies your AI agent cannot break."
        body="Teach the dimmi agent how your team handles refunds, cancellations, and account changes. It follows every step in order, respects your limits, and never goes off-script."
        shot="procedure"
        shotAlt="A duplicate-charge refund policy in dimmi showing required account checks, a $500 refund cap, and automatic Billing handover"
      />
      <HowItWorks
        label="How it works"
        title="Set your policy, test it safely, go live with confidence."
        body="Turn the refund, billing, and cancellation playbooks your team already uses into clear steps your AI agent follows every single time."
        steps={how}
        visual={(step) => <ProcedureEditorFragment step={step} />}
      />
      <Rows title="Trust AI with the work that actually fills your queue." rows={rows} />
      <FeatureGrid title="Higher resolution rates without risking your brand." items={more} />
      <Questions body="What support leads ask before automating complex requests." items={questions} />
      <FinalCta />
    </>
  );
}
