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
    body: "A board on your help center. Customers ask for what they need and vote for what others asked.",
    visual: <VoteFragment />,
  },
  {
    title: "Link it from the conversation",
    body: "When a customer asks in chat or email, link the request beside the thread. Their vote counts.",
    visual: <LinkFragment />,
  },
  {
    title: "Tell them when it ships",
    body: "Move a request and email everyone behind it once, with a note and the update that shipped it.",
    visual: <ShippedFragment />,
  },
];

const rows = [
  {
    label: "Triage",
    title: "Every request, in one list.",
    body: "Sort by trending, most votes, or newest. Narrow to a board or a status, and search by title.",
    visual: <RequestsFragment />,
  },
  {
    label: "Votes",
    title: "Who asked, and how much it matters.",
    body: "Each vote carries the customer, their company, and how much they need it. The conversations it came up in sit underneath.",
    visual: <VotersFragment />,
  },
  {
    label: "Roadmap",
    title: "Your statuses make your roadmap.",
    body: "Name the stages a request moves through, set their order, and choose which ones customers see as roadmap columns.",
    visual: <StatusesFragment />,
  },
];

const more = [
  { icon: Kanban, title: "Several boards", body: "Keep feature requests and bug reports apart, or split by product." },
  { icon: LockKey, title: "Who sees each board", body: "Public, signed-in customers only, or just your team." },
  { icon: CheckCircle, title: "Approve first", body: "New posts wait for a teammate before anyone else sees them." },
  { icon: ArrowsMerge, title: "Merge duplicates", body: "Votes and comments move to the request you keep." },
  { icon: Note, title: "Internal notes", body: "Argue it out under the request, where customers can't see." },
  {
    icon: UserPlus,
    title: "Vote for a customer",
    body: "Count a request you heard on a call, with how much it matters.",
  },
  {
    icon: Megaphone,
    title: "Linked to the changelog",
    body: "A shipped request points to the update that shipped it.",
  },
  { icon: Broadcast, title: "Webhooks", body: "New posts, votes, status changes, and merges, sent to your systems." },
  { icon: Code, title: "REST API", body: "Read and manage requests, boards, and statuses from your own code." },
];

const questions = [
  {
    question: "Is the board on yet another site?",
    answer: "No. It's on your help center, on your own domain, as a list customers can sort and a roadmap by status.",
  },
  {
    question: "Can someone stuff the ballot without an account?",
    answer:
      "No. Anyone can read a public board, but customers sign in to post and vote. Every vote belongs to a real person you can reach.",
  },
  {
    question: "Can we keep a board away from the public?",
    answer: "Yes. A board can be open to signed-in customers only, or internal, so only your team ever sees it.",
  },
  {
    question: "What does linking a conversation actually do?",
    answer:
      "Three things. The customer's vote counts, the conversation shows on the request, and they hear when it ships. Unlinking keeps the vote.",
  },
  {
    question: "Does it sync requests to Linear or Jira?",
    answer:
      "No, not yet. Tickets link to Linear, Jira, and GitHub. Feature requests don't. They reach your tracker through webhooks and the API, and you write that glue yourself.",
  },
];

export function FeedbackPage() {
  return (
    <>
      <ProductHero
        eyebrow="Feedback"
        title="Know who asked for what."
        body="Feature request boards and a public roadmap on your help center. Customers vote, your team links conversations, and voters hear when it ships."
        visual={<RoadmapFragment />}
      />
      <Steps title="From a request to a release." items={steps} />
      <Rows title="Built for the team that decides." rows={rows} />
      <FeatureGrid title="The rest of the board." items={more} />
      <Questions body="What people ask before they open a board." items={questions} />
      <FinalCta />
    </>
  );
}
