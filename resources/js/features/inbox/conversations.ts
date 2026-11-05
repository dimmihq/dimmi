/*
 * How the inbox names and dates what the API returns: a customer by name,
 * then address, then as a customer; times as how long ago they were; and
 * a timeline entry as the kind of bubble the thread draws for it.
 */
import { useSuspenseQuery } from "@tanstack/react-query";
import { useCurrentWorkspace } from "@/features/shell/workspace";
import { meQuery } from "@/lib/api/auth";
import type { ContactSummary, Conversation, Message, TimelineEvent } from "@/lib/api/inbox";
import { formatRelative } from "@/lib/i18n/format";

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

/*
 * ago says how long ago a moment was, in the reader's language: "Just
 * now" under a minute, then minutes, hours and so on.
 */
export function ago(iso: string | null, now = Date.now()): string {
  if (!iso) {
    return "";
  }
  if (now - new Date(iso).getTime() < 60_000) {
    return "Just now";
  }

  return formatRelative(iso, { now, style: "short" });
}

/*
 * waitedFor says how long since the moment, in the shortest unit that
 * reads well: minutes, hours, then days.
 */
export function waitedFor(iso: string, now = Date.now()): string {
  const minutes = Math.max(0, Math.floor((now - new Date(iso).getTime()) / 60_000));
  if (minutes < 60) {
    return `${Math.max(minutes, 1)} m`;
  }
  if (minutes < 1440) {
    return `${Math.floor(minutes / 60)} h`;
  }

  return `${Math.floor(minutes / 1440)} d`;
}

/*
 * A thread entry is a timeline entry as the thread draws it: from the
 * customer, from a teammate, from dimmi (the agent that answers
 * customers), an internal note, or an event such as a handover.
 */
export type ThreadEntry =
  | {
      kind: "customer" | "teammate" | "dimmi" | "note";
      id: string;
      author: string;
      at: string;
      text: string;
      held?: boolean;
      authorId?: string | null;
      edited?: boolean;
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
  if (event.name === "conversation.assigned" && typeof event.to !== "string") {
    const to = event.to?.member?.name ?? event.to?.team?.name;
    return to ? `${actor} assigned the conversation to ${to}` : `${actor} unassigned the conversation`;
  }
  if (event.name === "conversation.priority_changed") {
    return event.to === "none" ? `${actor} removed the priority` : `${actor} set the priority to ${event.to}`;
  }

  return `${actor} ${statusWords[event.name] ?? "changed the conversation"}`;
}

export function toEntry(message: Message): ThreadEntry {
  const at = ago(message.created_at);
  if (message.type === "event" && message.event) {
    return { kind: "event", id: message.id, at, text: eventText(message.event) };
  }
  if (message.deleted_at) {
    const by = message.deleted_by?.name;
    return { kind: "event", id: message.id, at, text: by ? `Message deleted by ${by}` : "Message deleted" };
  }
  const text = message.body ?? "";
  const author = message.author.name;
  if (message.type === "note") {
    return {
      kind: "note",
      id: message.id,
      author,
      at,
      text,
      authorId: message.author.id,
      edited: Boolean(message.edited_at),
    };
  }
  if (message.author.type === "contact") {
    return { kind: "customer", id: message.id, author, at, text };
  }
  if (message.author.type === "agent") {
    return { kind: "dimmi", id: message.id, author, at, text };
  }

  return {
    kind: "teammate",
    id: message.id,
    author,
    at,
    text,
    held: Boolean(message.scheduled_send_at),
    authorId: message.author.id,
  };
}
