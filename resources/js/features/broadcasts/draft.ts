/*
 * The draft a broadcast composer holds, the steps it is written in, and
 * what must be true before it can go out. The draft is the part of a
 * stored broadcast a teammate changes; each change is saved on its own,
 * so the composer sends only the fields that changed.
 */
import type { Broadcast, BroadcastChanges, BroadcastEstimate, Channel } from "@/lib/api/broadcasts";

export type StepId = "setup" | "write" | "delivery" | "review";

export const stepNames: Record<StepId, string> = {
  setup: "Setup",
  write: "Write",
  delivery: "Delivery",
  review: "Review",
};

export const steps: StepId[] = ["setup", "write", "delivery", "review"];

/*
 * Draft is what the form holds: the fields of the broadcast a draft can
 * change. body is Markdown with Liquid details in it.
 */
export type Draft = Pick<
  Broadcast,
  | "name"
  | "audience_id"
  | "rules"
  | "room_ids"
  | "topic_id"
  | "service"
  | "subject"
  | "preview"
  | "body"
  | "sender_id"
  | "email_address_id"
  | "slack_as"
  | "reply_team_id"
  | "reply_member_id"
  | "ai_replies"
  | "track_clicks"
  | "skip_frequency_cap"
  | "schedule_kind"
  | "scheduled_at"
  | "goal"
>;

export type DraftKey = keyof Draft;

export function draftOf(broadcast: Broadcast): Draft {
  return {
    name: broadcast.name,
    audience_id: broadcast.audience_id,
    rules: broadcast.rules,
    room_ids: broadcast.room_ids,
    topic_id: broadcast.topic_id,
    service: broadcast.service,
    subject: broadcast.subject,
    preview: broadcast.preview,
    body: broadcast.body,
    sender_id: broadcast.sender_id,
    email_address_id: broadcast.email_address_id,
    slack_as: broadcast.slack_as,
    reply_team_id: broadcast.reply_team_id,
    reply_member_id: broadcast.reply_member_id,
    ai_replies: broadcast.ai_replies,
    track_clicks: broadcast.track_clicks,
    skip_frequency_cap: broadcast.skip_frequency_cap,
    schedule_kind: broadcast.schedule_kind,
    scheduled_at: broadcast.scheduled_at,
    goal: broadcast.goal,
  };
}

const references = [
  "audience_id",
  "topic_id",
  "sender_id",
  "email_address_id",
  "reply_team_id",
  "reply_member_id",
] as const;

const plain = [
  "name",
  "rules",
  "service",
  "subject",
  "preview",
  "body",
  "slack_as",
  "ai_replies",
  "track_clicks",
  "skip_frequency_cap",
  "goal",
] as const;

/*
 * changesOf writes the fields named in keys the way a save sends them:
 * a cleared reference as an empty id, the schedule as its kind and
 * time together.
 */
export function changesOf(draft: Draft, keys: Set<DraftKey>): BroadcastChanges {
  const changes: Record<string, unknown> = {};
  for (const key of references) {
    if (keys.has(key)) {
      changes[key] = draft[key] ?? "";
    }
  }
  for (const key of plain) {
    if (keys.has(key)) {
      changes[key] = draft[key];
    }
  }
  if (keys.has("room_ids")) {
    changes.room_ids = draft.room_ids ?? [];
  }
  if (keys.has("schedule_kind") || keys.has("scheduled_at")) {
    changes.schedule =
      draft.schedule_kind === "now" ? { kind: "now" } : { kind: draft.schedule_kind, at: draft.scheduled_at };
  }

  return changes as BroadcastChanges;
}

/*
 * Check is one thing that must be true before sending, and the step
 * where it is fixed. A check on the review step is advice, not a
 * requirement.
 */
export interface Check {
  label: string;
  done: boolean;
  step: StepId;
}

/*
 * testable says whether the channel takes a test send: an email to an
 * address, or a post into one Slack channel.
 */
export function testable(channel: Channel): boolean {
  return channel === "email" || channel === "slack";
}

function audienceCheck(channel: Channel, draft: Draft, estimate: BroadcastEstimate | undefined): Check {
  const chosen = channel === "slack" || draft.audience_id !== null || draft.rules !== null;
  const label = channel === "slack" ? "Customer channels to post in" : "An audience with people in it";

  return { label, done: chosen && (estimate?.reach ?? 0) > 0, step: "setup" };
}

/*
 * checksOf lists what must be true before the broadcast can go out, and
 * whether it is. The server checks again when it sends.
 */
export function checksOf(
  channel: Channel,
  draft: Draft,
  estimate: BroadcastEstimate | undefined,
  tested: boolean,
): Check[] {
  const checks: Check[] = [audienceCheck(channel, draft, estimate)];
  if (channel === "email") {
    checks.push({ label: "A subject", done: draft.subject.trim() !== "", step: "write" });
  }
  checks.push({ label: "A message", done: draft.body.trim() !== "", step: "write" });
  checks.push({ label: "A teammate it comes from", done: draft.sender_id !== null, step: "delivery" });
  if (channel === "email") {
    checks.push({ label: "An address to send from", done: draft.email_address_id !== null, step: "delivery" });
  }
  if (draft.schedule_kind !== "now") {
    const future = draft.scheduled_at !== null && Date.parse(draft.scheduled_at) > Date.now();
    checks.push({ label: "A send time in the future", done: future, step: "delivery" });
  }
  if (testable(channel)) {
    checks.push({ label: "A test sent to yourself", done: tested, step: "review" });
  }

  return checks;
}

/*
 * readyToSend says whether every required check passes.
 */
export function readyToSend(checks: Check[]): boolean {
  return checks.every((check) => check.done || check.step === "review");
}
