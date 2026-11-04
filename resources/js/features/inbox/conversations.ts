/*
 * How the inbox names and dates what the API returns: a customer by name,
 * then address, then as a customer; times as how long ago they were; and
 * a timeline entry as the kind of bubble the thread draws for it.
 */
import { useSuspenseQuery } from "@tanstack/react-query";
import { useCurrentWorkspace } from "@/features/shell/workspace";
import { meQuery } from "@/lib/api/auth";
import type { ContactSummary, Conversation, Message, TimelineEvent } from "@/lib/api/inbox";

/*
 * useInbox returns the workspace the inbox works in and the person
 * reading it.
 */
export function useInbox() {
  const { data: me } = useSuspenseQuery(meQuery);
  const { current } = useCurrentWorkspace(me?.workspaces ?? []);

  return { workspace: current?.id ?? "", me: me?.user.name ?? "You", userId: me?.user.id ?? "" };
}

export function customerName(contact: ContactSummary): string {
  return contact.name ?? contact.email ?? "Customer";
}

export function subjectOf(conversation: Conversation): string {
  return conversation.subject ?? conversation.last_message?.excerpt ?? `Conversation #${conversation.number}`;
}

const units: [Intl.RelativeTimeFormatUnit, number][] = [
  ["year", 31_536_000],
  ["month", 2_592_000],
  ["week", 604_800],
  ["day", 86_400],
  ["hour", 3600],
  ["minute", 60],
];

/*
 * ago says how long ago a moment was, in the reader's language: "Just
 * now" under a minute, then minutes, hours and so on.
 */
export function ago(iso: string | null, now = Date.now()): string {
  if (!iso) {
    return "";
  }
  const seconds = Math.round((now - new Date(iso).getTime()) / 1000);
  const format = new Intl.RelativeTimeFormat(undefined, { numeric: "auto", style: "short" });
  for (const [unit, size] of units) {
    if (seconds >= size) {
      return format.format(-Math.floor(seconds / size), unit);
    }
  }

  return "Just now";
}

/*
 * A thread entry is a timeline entry as the thread draws it: from the
 * customer, from a teammate, from dimmi (the agent that answers
 * customers), an internal note, or an event such as a handover.
 */
/*
 * ThreadEntry is one entry as the thread shows it. A said entry carries
 * how its text is read, Markdown or plain words, and for received email
 * whether more of it can be opened and whether its body was cut short.
 */
export type ThreadEntry =
  | {
      kind: "customer" | "teammate" | "dimmi" | "note";
      id: string;
      author: string;
      at: string;
      text: string;
      format: Message["body_format"];
      original: boolean;
      truncated: boolean;
    }
  | { kind: "event"; id: string; at: string; text: string };

const statusWords: Record<string, string> = {
  "conversation.closed": "closed the conversation",
  "conversation.reopened": "reopened the conversation",
  "conversation.snoozed": "snoozed the conversation",
  "conversation.unsnoozed": "woke the conversation up",
  "conversation.marked_spam": "marked the conversation as spam",
  "conversation.unmarked_spam": "took the conversation out of spam",
  "conversation.updated": "updated the conversation",
};

/*
 * eventText says in words what an event entry records.
 */
export function eventText(event: TimelineEvent): string {
  const actor = event.actor.name || "dimmi";
  if (event.name === "conversation.assigned") {
    const to = event.to?.member?.name ?? event.to?.team?.name;
    return to ? `${actor} assigned the conversation to ${to}` : `${actor} unassigned the conversation`;
  }
  if (event.name === "conversation.priority_changed") {
    return event.priority ? `${actor} marked the conversation as a priority` : `${actor} removed the priority`;
  }

  return `${actor} ${statusWords[event.name] ?? "changed the conversation"}`;
}

export function toEntry(message: Message): ThreadEntry {
  const at = ago(message.created_at);
  if (message.type === "event" && message.event) {
    return { kind: "event", id: message.id, at, text: eventText(message.event) };
  }
  const said = {
    id: message.id,
    author: message.author.name,
    at,
    text: message.body ?? "This message was deleted.",
    format: message.body === null ? "plain" : (message.body_format ?? "markdown"),
    original: message.has_original ?? false,
    truncated: message.truncated ?? false,
  } as const;
  if (message.type === "note") {
    return { kind: "note", ...said };
  }
  if (message.author.type === "contact") {
    return { kind: "customer", ...said };
  }
  if (message.author.type === "agent") {
    return { kind: "dimmi", ...said };
  }

  return { kind: "teammate", ...said };
}
