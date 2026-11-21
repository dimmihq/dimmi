import {
  ArrowsMerge,
  Broadcast,
  CheckCircle,
  Code,
  Kanban,
  LockKey,
  Megaphone,
  Note,
  UserPlus,
} from "@phosphor-icons/react";

import { FeatureGrid, ProductHero, Questions, Rows, Steps } from "../components/product-page";
import {
  LinkFragment,
  RequestsFragment,
  RoadmapFragment,
  ShippedFragment,
  StatusesFragment,
  VoteFragment,
  VotersFragment,
} from "../fragments/feedback-fragments";
import { FinalCta } from "../sections/final-cta";

const steps = [
  {
    title: "Customers post and vote",
    body: "Open a board on your help center so customers request features and upvote ideas.",
    visual: <VoteFragment />,
  },
  {
    title: "Link from live chats",
    body: "When someone asks in chat or email, attach the request beside the thread so their vote counts.",
    visual: <LinkFragment />,
  },
  {
    title: "Notify voters at launch",
    body: "Move a request to shipped and email everyone behind it with the changelog update.",
    visual: <ShippedFragment />,
  },
];

const rows = [
  {
    label: "Triage",
    title: "Every request in one sortable list.",
    body: "Sort by trending, top votes, or newest. Filter by board or status so product meetings take five minutes instead of an hour.",
    visual: <RequestsFragment />,
  },
  {
    label: "Votes",
    title: "See who asked and how much it matters.",
    body: "Every vote shows the customer, their company, and their urgency, with the exact support chats sitting right underneath.",
    visual: <VotersFragment />,
  },
  {
    label: "Roadmap",
    title: "Your statuses power your roadmap.",
    body: "Name your own stages, set their order, and pick which ones appear as public roadmap columns.",
    visual: <StatusesFragment />,
  },
];

const more = [
  { icon: Kanban, title: "Multiple boards", body: "Split feature requests, bug ideas, or separate products." },
  { icon: LockKey, title: "Access controls", body: "Make each board public, customer-only, or private to your team." },
  { icon: CheckCircle, title: "Moderation queue", body: "Review new submissions before they show up on a public board." },
  { icon: ArrowsMerge, title: "Merge duplicates", body: "Combine duplicate posts and move every vote and comment over." },
  { icon: Note, title: "Internal notes", body: "Debate priorities under a request where customers never see." },
  {
    icon: UserPlus,
    title: "Vote for a customer",
    body: "Log a request you heard on a call along with how urgent it is.",
  },
  {
    icon: Megaphone,
    title: "Linked to changelog",
    body: "Every shipped item points straight to the changelog update that launched it.",
  },
  { icon: Broadcast, title: "Webhooks", body: "Listen for new posts, votes, merges, and status changes in your tools." },
  { icon: Code, title: "REST API", body: "Read and manage boards, statuses, and requests from your own code." },
];

const questions = [
  {
    question: "Does the board live on a separate site?",
    answer:
      "It lives right on your help center, on your own domain, with a sortable request list and a roadmap grouped by status.",
  },
  {
    question: "Can strangers stuff the ballot without an account?",
    answer:
      "Anyone can browse a public board, but customers sign in to post or vote. Every vote ties back to a real person you can notify.",
  },
  {
    question: "Can we keep a board private to customers or our team?",
    answer:
      "Yes. Set any board to signed-in customers only, or internal so only your workspace members see it.",
  },
  {
    question: "What happens when a teammate links a chat to a request?",
    answer:
      "Three things: the customer's vote counts, the chat shows on the request, and they get notified when it ships.",
  },
  {
    question: "Do feature requests sync straight into Linear or Jira?",
    answer:
      "Not out of the box yet. Tickets link straight to Linear, Jira, and GitHub, while feature requests connect through webhooks and the REST API if you want custom sync.",
  },
];

export function FeedbackPage() {
  return (
    <>
      <ProductHero
        eyebrow="Feedback"
        title="Stop losing feature requests in spreadsheets nobody checks."
        body="Collect votes on your help center, link live support chats, and email every voter the day you ship."
        visual={<RoadmapFragment />}
      />
      <Steps title="From customer request to shipped release." items={steps} />
      <Rows title="Know who asked and how much it matters." rows={rows} />
      <FeatureGrid title="Everything to run your public roadmap." items={more} />
      <Questions body="What teams ask before opening a feedback board." items={questions} />
      <FinalCta />
    </>
  );
}
