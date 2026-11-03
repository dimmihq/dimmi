/*
 * Conversations as the inbox shows them, and the example conversation a
 * new workspace sees until real ones arrive, so the inbox's panels can be
 * tried before anything is connected. The example lives in the browser
 * only and says so wherever it shows.
 */
import type { InboxView } from "@/features/shell/navigation";

export interface Customer {
  name: string;
  email: string;
  company: string;
  initials: string;
  location: string;
}

/*
 * A message in the thread: from the customer, from a teammate, from Dimmi
 * (the agent that answers customers), an internal note only teammates see,
 * or an event such as a handover.
 */
export type ThreadEntry =
  | { kind: "customer" | "teammate" | "dimmi"; id: string; author: string; at: string; text: string }
  | { kind: "note"; id: string; author: string; at: string; text: string }
  | { kind: "event"; id: string; at: string; text: string };

export interface Conversation {
  id: string;
  example: boolean;
  subject: string;
  preview: string;
  updated: string;
  status: "open" | "snoozed" | "done";
  priority: "low" | "normal" | "high" | "urgent";
  channel: string;
  assignee: string | null;
  customer: Customer;
  thread: ThreadEntry[];
  /* What Operator, the teammates' own agent, has gathered about the case. */
  briefing: string;
  /* Teammates the example can be assigned to, besides the person reading. */
  teammates: string[];
}

const example: Conversation = {
  id: "example",
  example: true,
  subject: "Can't log in",
  preview: "I've been having trouble logging into my account for the past few days…",
  updated: "14m ago",
  status: "open",
  priority: "normal",
  channel: "Messenger",
  assignee: null,
  customer: {
    name: "Emily Johnson",
    email: "emily@northwind.example",
    company: "Northwind",
    initials: "EJ",
    location: "Lisbon, Portugal",
  },
  thread: [
    {
      kind: "customer",
      id: "m1",
      author: "Emily Johnson",
      at: "32m ago",
      text: "I've been having trouble logging into my account for the past few days. It keeps saying my password is incorrect, but I'm sure I'm typing it right since I use a password manager.\n\nCan you help me out?",
    },
    {
      kind: "dimmi",
      id: "m2",
      author: "Dimmi",
      at: "31m ago",
      text: "Sorry about that, Emily. Password managers sometimes keep an old password after a reset. Could you try “Forgot password?” on the sign-in page? The link works for 60 minutes and signs you in right after.",
    },
    {
      kind: "customer",
      id: "m3",
      author: "Emily Johnson",
      at: "18m ago",
      text: "I tried that twice and the reset link says it has expired as soon as I open it.",
    },
    {
      kind: "event",
      id: "e1",
      at: "18m ago",
      text: "Dimmi handed the conversation to your team: the reset link fails for this customer.",
    },
    {
      kind: "note",
      id: "n1",
      author: "Operator",
      at: "17m ago",
      text: "Two other customers reported expired reset links this week. Both use Outlook, which opens links in a preview first. Worth checking whether the preview spends the link.",
    },
  ],
  teammates: ["Sam Okafor", "Lea Martin"],
  briefing:
    "Emily can't sign in: her saved password is refused, and both reset links she opened said they had expired. Dimmi handed over after the second failure.\n\nTwo similar reports this week came from Outlook users, whose mail preview may open the link before they do. Suggested next step: ask Emily which mail app she uses, and send a sign-in code instead of a link.",
};

/*
 * conversationsIn returns the conversations of an inbox view. Until the
 * inbox is connected, only the example shows, in the views a new
 * conversation lands in.
 */
export function conversationsIn(view: InboxView): Conversation[] {
  return view === "unassigned" || view === "all" ? [example] : [];
}

export function findConversation(id: string): Conversation | undefined {
  return id === example.id ? example : undefined;
}
