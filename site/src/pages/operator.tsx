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
    title: "Brief me",
    body: "One click, and it reads the thread and the customer's history so you don't have to. The brief stays as a note.",
    visual: <BriefFragment />,
  },
  {
    title: "Draft a reply",
    body: "Operator writes it from your knowledge and drops it in the reply box. You edit it. You send it.",
    visual: <DraftFragment />,
  },
  {
    title: "Apply its changes",
    body: "Tags, an owner, a priority. Each change waits for your OK and can be undone after.",
    visual: <ChangeFragment />,
  },
];

const rows = [
  {
    label: "Questions",
    title: "Ask about the customer in front of you.",
    body: "Has Omar written before? What did we promise him last time? Operator searches past conversations and answers right in the thread.",
    visual: (
      <ShotFlow name="operator" alt="Asking Operator about the customer's past issues" maxHeight={430} maxWidth={400} />
    ),
  },
  {
    label: "Workspace",
    title: "Or ask about the whole workspace.",
    body: "Volume, response times, what the AI agent got up to. Operator pulls the report and tells you what changed.",
    visual: <WorkspaceFragment />,
  },
  {
    label: "Control",
    title: "You set what it can change.",
    body: "Each kind of change gets a setting: on its own, ask first, or off. What it does on its own still shows in the chat and can be undone.",
    visual: <PolicyFragment />,
  },
];

const more = [
  {
    icon: Tray,
    title: "Triage on arrival",
    body: "Switch it on and Operator proposes tags, an owner, and a priority for each new conversation.",
  },
  {
    icon: ChatsCircle,
    title: "In Slack too",
    body: "Ask it from your team's internal Slack channels and direct messages.",
  },
  {
    icon: ShieldCheck,
    title: "It sees what you see",
    body: "Operator reads the workspace as the teammate who asked. Same role, same rules, no extra access.",
  },
  {
    icon: Brain,
    title: "Remembers how you work",
    body: "Tell it how you like your answers and it remembers. Only you see what it keeps.",
  },
  {
    icon: Lightning,
    title: "Runs your actions",
    body: "It can look up an order in your own systems. Changing anything there always waits for you.",
  },
  {
    icon: ListMagnifyingGlass,
    title: "Shows its work",
    body: "Every answer comes with a log of what Operator read and searched to get there.",
  },
];

const questions = [
  {
    question: "Will a customer ever see what it writes?",
    answer:
      "No. Operator talks to your team, never to customers. A draft only reaches a customer when a teammate sends it.",
  },
  {
    question: "Isn't this just the AI agent again?",
    answer:
      "No. The agent answers customers. Operator works for your team: it briefs you, answers your questions, and drafts replies.",
  },
  {
    question: "Will it change things behind my back?",
    answer: "Only where you allow it. Until you say otherwise, every change waits for a teammate to apply it.",
  },
  {
    question: "Does it run when nobody asked it to?",
    answer: "No, unless you switch on briefs as conversations open or triage of new conversations. Both start off.",
  },
  {
    question: "Can it undo everything it does?",
    answer: "No. Actions in your own systems can't be undone, so they always wait for you.",
  },
];

export function OperatorPage() {
  return (
    <>
      <ProductHero
        eyebrow="Operator"
        title="An AI teammate beside every conversation."
        body="Operator briefs you, digs up the customer's history, drafts replies, and proposes changes. Your customers never see it."
        shot="inbox-operator"
        shotAlt="Operator beside a conversation's details, ready to brief, draft a reply or suggest tags and an owner"
      />
      <Steps title="Catch up in one click." items={steps} />
      <Rows title="Ask it what you'd ask a colleague." rows={rows} />
      <FeatureGrid title="Not just a chat box." items={more} />
      <Questions body="What you'd ask before your team starts asking it." items={questions} />
      <FinalCta />
    </>
  );
}
