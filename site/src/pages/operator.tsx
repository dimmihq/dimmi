import { Brain, ChatsCircle, Lightning, ListMagnifyingGlass, ShieldCheck, Tray } from "@phosphor-icons/react";

import { FeatureGrid, ProductHero, Questions, Rows, Steps } from "../components/product-page";
import {
  BriefFragment,
  ChangeFragment,
  DraftFragment,
  PolicyFragment,
  WorkspaceFragment,
} from "../fragments/operator-fragments";
import { ShotFlow } from "../scenes/shot-flow";
import { FinalCta } from "../sections/final-cta";

const steps = [
  {
    title: "Brief me fast",
    body: "One click sums up a long chat and past tickets into a private note.",
    visual: <BriefFragment />,
  },
  {
    title: "Draft a reply",
    body: "Writes an answer from your docs right in the box. You edit and send it.",
    visual: <DraftFragment />,
  },
  {
    title: "Apply changes safely",
    body: "Proposes tags, owners, or priorities. Approve with one click, or undo after.",
    visual: <ChangeFragment />,
  },
];

const rows = [
  {
    label: "Customer history",
    title: "Ask about the customer in front of you.",
    body: "Has Omar written before? What did we promise last time? Operator checks past chats and answers right beside the thread.",
    visual: (
      <ShotFlow name="operator" alt="Asking Operator about the customer's past issues" maxHeight={430} maxWidth={400} />
    ),
  },
  {
    label: "Workspace",
    title: "Ask about your whole support queue.",
    body: "Check volume spikes, reply times, or what the AI agent answered this week. Operator pulls the numbers and explains what moved.",
    visual: <WorkspaceFragment />,
  },
  {
    label: "Control",
    title: "You pick what it can change.",
    body: "Set each change type to automatic, ask first, or off. Every edit shows in the chat log and can be undone.",
    visual: <PolicyFragment />,
  },
];

const more = [
  {
    icon: Tray,
    title: "Triage on arrival",
    body: "Turn it on to suggest tags, an owner, and a priority for every new chat.",
  },
  {
    icon: ChatsCircle,
    title: "Works in Slack too",
    body: "Ask Operator questions from your team's Slack channels and DMs.",
  },
  {
    icon: ShieldCheck,
    title: "Matches your permissions",
    body: "Operator only sees what the teammate asking is allowed to see.",
  },
  {
    icon: Brain,
    title: "Remembers your style",
    body: "Tell it how you like summaries or drafts, and it keeps that preference for you.",
  },
  {
    icon: Lightning,
    title: "Runs your actions",
    body: "Looks up orders in your backend. Actions that change data always wait for your OK.",
  },
  {
    icon: ListMagnifyingGlass,
    title: "Runs on a schedule with Routines",
    body: "Sync merged GitHub PRs to help articles, catch knowledge gaps, and tune up policies every week.",
    link: { href: "/routines", label: "Explore Routines" },
  },
];

const questions = [
  {
    question: "Can a customer ever see what Operator writes?",
    answer:
      "Never. Operator only talks to your team. A draft reaches a customer only after a teammate reviews it and clicks send.",
  },
  {
    question: "How is Operator different from the dimmi agent?",
    answer:
      "The dimmi agent replies to customers. Operator works backstage for your team, summing up chats, answering questions, and drafting replies.",
  },
  {
    question: "Will it edit my workspace behind my back?",
    answer:
      "Only for change types you allow. By default, every tag, assignment, or setting change waits as a changeset for a teammate to approve.",
  },
  {
    question: "How is Operator billed on dimmi Cloud?",
    answer:
      "Each thread a teammate starts with Operator counts as one AI conversation, no matter how many follow-up questions you ask. You can also skip Operator and connect Claude or Cursor over MCP for $0.",
  },
  {
    question: "Can it undo external API actions?",
    answer:
      "External actions in your own systems cannot be undone once sent, so Operator always asks a teammate to approve them first.",
  },
];

export function OperatorPage() {
  return (
    <>
      <ProductHero
        eyebrow="Operator"
        title="An AI teammate that works for your team, not your customers."
        body="Operator sums up long chats, searches customer history, drafts replies, and proposes edits you can undo. Customers never see it."
        shot="inbox-operator"
        shotAlt="Operator beside a conversation's details, ready to brief, draft a reply or suggest tags and an owner"
      />
      <Steps title="Catch up on any thread in seconds." items={steps} />
      <Rows title="Ask it what you would ask a senior teammate." rows={rows} />
      <FeatureGrid title="More than a chat sidebar." items={more} />
      <Questions body="What teams ask before letting Operator into the inbox." items={questions} />
      <FinalCta />
    </>
  );
}
