/*
 * How the inbox names and dates what the API returns: a customer by name,
 * then address, then as a customer; times as how long ago they were; and
 * a timeline entry as the kind of bubble the thread draws for it.
 */
import { useSuspenseQuery } from "@tanstack/react-query";
import type { ActionCall } from "@/features/ai/actions/api";
import { actionCallOf, callEventText, callOfEvent, isOpen } from "@/features/ai/actions/format";
import { useCurrentWorkspace } from "@/features/shell/workspace";
import { meQuery } from "@/lib/api/auth";
import type {
  Attachment,
  ContactSummary,
  Conversation,
  MentionRef,
  Message,
  Reaction,
  TimelineEvent,
} from "@/lib/api/inbox";
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
/*
 * ThreadEntry is one entry as the thread shows it. A said entry carries
 * how its text is read, Markdown or plain words, and for received email
 * whether more of it can be opened and whether its body was cut short.
 * Failed holds why a reply never reached the customer, null otherwise.
 * Files are what the message carries. Call is an action the agent asked
 * to run, drawn as a card under its reply while it waits on someone; a
 * call entry is the timeline's line for its decision or outcome.
 */
export type ThreadEntry =
  | {
      kind: "customer" | "teammate" | "dimmi" | "note";
      id: string;
      author: string;
      at: string;
      iso?: string;
      seenAt?: string | null;
      text: string;
      format: Message["body_format"];
      original: boolean;
      truncated: boolean;
      failed: string | null;
      redacted?: number;
      held?: boolean;
      authorId?: string | null;
      edited?: boolean;
      mentions?: MentionRef[];
      reactions?: Reaction[];
      files: Attachment[];
      call?: ActionCall;
    }
  | { kind: "event"; id: string; at: string; iso?: string; text: string }
  | { kind: "call"; id: string; at: string; iso?: string; call: ActionCall };

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
 * renamedTo returns the new subject an update event gave the
 * conversation, if it changed the subject.
 */
function renamedTo(event: TimelineEvent): string | null {
  const subject = event.changes?.subject?.[1];
  if (event.name !== "conversation.updated" || typeof subject !== "string" || subject === "") {
    return null;
  }
  return subject;
}

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

  const renamed = renamedTo(event);
  if (renamed) {
    return `${actor} renamed the conversation to “${renamed}”`;
  }

  if (event.name === "conversation.tagged") {
    return `${actor} tagged the conversation ${(event.tags ?? []).join(", ")}`;
  }
  if (event.name === "conversation.untagged") {
    return `${actor} removed the tag ${(event.tags ?? []).join(", ")}`;
  }

  return `${actor} ${statusWords[event.name] ?? "changed the conversation"}`;
}

/*
 * filesOf returns the files the message carries.
 */
function filesOf(message: Message): Attachment[] {
  return message.attachments ?? [];
}

/*
 * eventEntry is an event as the thread draws it: a call's decision or
 * outcome as its line, anything else as words.
 */
function eventEntry(message: Message, event: TimelineEvent): ThreadEntry {
  const at = ago(message.created_at);
  const decided = callOfEvent(event);
  if (decided) {
    return { kind: "call", id: message.id, at, iso: message.created_at, call: decided };
  }

  return { kind: "event", id: message.id, at, iso: message.created_at, text: callEventText(event) ?? eventText(event) };
}

/*
 * waitingCall is the call a reply reports while it still waits on
 * someone; once decided, the timeline's own entry shows it.
 */
function waitingCall(message: Message): ActionCall | null {
  const reported = actionCallOf(message.ai);

  return reported && isOpen(reported.state) ? reported : null;
}

export function toEntry(message: Message): ThreadEntry {
  const at = ago(message.created_at);
  const iso = message.created_at;
  if (message.type === "event" && message.event) {
    return eventEntry(message, message.event);
  }
  const call = waitingCall(message);
  if (message.deleted_at) {
    const by = message.deleted_by?.name;
    return { kind: "event", id: message.id, at, iso, text: by ? `Message deleted by ${by}` : "Message deleted" };
  }
  const said = {
    id: message.id,
    author: message.author.name,
    at,
    iso,
    text: message.body ?? "",
    format: message.body_format ?? "markdown",
    original: message.has_original ?? false,
    truncated: message.truncated ?? false,
    failed: message.delivery_status === "failed" ? (message.delivery_error ?? "") : null,
    authorId: message.author.id,
    files: filesOf(message),
    ...(call ? { call } : {}),
  } as const;
  const redacted = message.redactions?.reduce((total, hit) => total + hit.count, 0);

  return byAuthor(message, redacted ? { ...said, redacted } : said);
}

type SaidFields = Omit<Extract<ThreadEntry, { author: string }>, "kind">;

/*
 * byAuthor finishes a said entry by who wrote it: a note keeps its
 * mentions and reactions, and a reply to the customer when they saw it.
 */
function byAuthor(message: Message, said: SaidFields): ThreadEntry {
  if (message.type === "note") {
    return {
      kind: "note",
      ...said,
      edited: Boolean(message.edited_at),
      mentions: message.mentions ?? [],
      reactions: message.reactions ?? [],
    };
  }
  if (message.author.type === "contact") {
    return { kind: "customer", ...said };
  }
  if (message.author.type === "agent") {
    return { kind: "dimmi", ...said, seenAt: message.read_at ?? null };
  }

  return { kind: "teammate", ...said, seenAt: message.read_at ?? null, held: Boolean(message.scheduled_send_at) };
}
