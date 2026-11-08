/*
 * What can start a workflow. Triggers come in groups by who or what sets
 * them off, the customer arriving, the conversation going on, the team,
 * or dimmi itself, and each group has its own colour so a workflow's
 * trigger reads at a glance in the list, the picker and on the canvas.
 * A workflow keeps the trigger it was made with: its steps, conditions
 * and the data they can use depend on it.
 */
import {
  RiAlarmWarningLine,
  RiChat3Line,
  RiChatNewLine,
  RiChatSmile2Line,
  RiCheckboxCircleLine,
  RiErrorWarningLine,
  RiHourglassLine,
  RiLinksLine,
  RiMessage2Line,
  RiReplyLine,
  RiSparkling2Line,
  RiStarSmileLine,
  RiStickyNoteLine,
  RiTimerLine,
  RiUserShared2Line,
  RiUserVoiceLine,
} from "@remixicon/react";
import type { ComponentType } from "react";

import type { ChipColor } from "@/components/base/badges/chip";

import { customerKinds, type StepKind } from "./workflows";

export type TriggerGroup = "start" | "during" | "team" | "dimmi" | "sla" | "reuse";

export type TriggerKind =
  | "conversation.opener"
  | "conversation.first_message"
  | "conversation.customer_reopened"
  | "message.customer"
  | "conversation.customer_quiet"
  | "conversation.rated"
  | "message.teammate"
  | "note.created"
  | "conversation.assignment_changed"
  | "conversation.status_changed"
  | "conversation.teammate_quiet"
  | "dimmi.handed_over"
  | "dimmi.resolved"
  | "sla.warning"
  | "sla.breached"
  | "reusable";

/*
 * Trigger is how a trigger is offered: its group, what sets it off, an
 * example of what to do with it, and its icon. Runs says the server
 * starts workflows from it today; only those are offered for new
 * workflows. Talks lists the steps that speak to the customer it may
 * use, dimmi whether it may let dimmi handle the conversation, and
 * onceOnly says it runs at most once per conversation.
 */
export interface Trigger {
  group: TriggerGroup;
  name: string;
  detail: string;
  icon: ComponentType<{ className?: string }>;
  runs: boolean;
  talks: StepKind[];
  dimmi?: boolean;
  onceOnly?: boolean;
}

export const triggerGroups: { id: TriggerGroup; name: string; detail: string; tint: ChipColor }[] = [
  { id: "start", name: "Start of conversation", detail: "When someone reaches out.", tint: "blue" },
  { id: "during", name: "During conversation", detail: "While you're talking.", tint: "purple" },
  { id: "team", name: "Team", detail: "When your team does something.", tint: "orange" },
  { id: "dimmi", name: "dimmi", detail: "When dimmi is done.", tint: "lime" },
  { id: "sla", name: "Service levels", detail: "When a reply target is at risk.", tint: "yellow" },
  { id: "reuse", name: "Reusable", detail: "Steps you can use anywhere.", tint: "neutral" },
];

export const triggers: Record<TriggerKind, Trigger> = {
  "conversation.opener": {
    group: "start",
    name: "Customer opens the messenger",
    detail: "Say hi before they type.",
    icon: RiChatSmile2Line,
    runs: false,
    talks: customerKinds,
    dimmi: true,
    onceOnly: true,
  },
  "conversation.first_message": {
    group: "start",
    name: "Customer sends their first message",
    detail: "Route it, or ask what it's about.",
    icon: RiMessage2Line,
    runs: true,
    talks: customerKinds,
    dimmi: true,
    onceOnly: true,
  },
  "conversation.customer_reopened": {
    group: "start",
    name: "Customer writes on a closed conversation",
    detail: "Route a returning customer again.",
    icon: RiChatNewLine,
    runs: true,
    talks: customerKinds,
    dimmi: true,
  },
  "message.customer": {
    group: "during",
    name: "Customer sends a message",
    detail: "Tag or route every message they send.",
    icon: RiChat3Line,
    runs: true,
    talks: [],
  },
  "conversation.customer_quiet": {
    group: "during",
    name: "Customer stops replying",
    detail: "Nudge them, then close.",
    icon: RiTimerLine,
    runs: false,
    talks: customerKinds,
    dimmi: true,
  },
  "conversation.rated": {
    group: "during",
    name: "Customer rates the conversation",
    detail: "Follow up on a bad rating.",
    icon: RiStarSmileLine,
    runs: false,
    talks: ["message"],
  },
  "message.teammate": {
    group: "team",
    name: "Teammate sends a reply",
    detail: "Tag or snooze after a reply.",
    icon: RiReplyLine,
    runs: true,
    talks: [],
  },
  "note.created": {
    group: "team",
    name: "Teammate adds a note",
    detail: "Act on notes, like escalations.",
    icon: RiStickyNoteLine,
    runs: true,
    talks: [],
  },
  "conversation.assignment_changed": {
    group: "team",
    name: "Assignment changes",
    detail: "Brief whoever picks it up.",
    icon: RiUserShared2Line,
    runs: true,
    talks: [],
  },
  "conversation.status_changed": {
    group: "team",
    name: "Conversation status changes",
    detail: "When it's closed, snoozed or reopened.",
    icon: RiCheckboxCircleLine,
    runs: true,
    talks: [],
  },
  "conversation.teammate_quiet": {
    group: "team",
    name: "Customer waits for the team",
    detail: "Reassign before the customer waits.",
    icon: RiHourglassLine,
    runs: false,
    talks: ["message", "reply_time"],
    onceOnly: true,
  },
  "dimmi.handed_over": {
    group: "dimmi",
    name: "dimmi hands over",
    detail: "Brief the team when dimmi can't help.",
    icon: RiUserVoiceLine,
    runs: false,
    talks: [],
  },
  "dimmi.resolved": {
    group: "dimmi",
    name: "dimmi resolves a conversation",
    detail: "Ask for a rating, or tag it.",
    icon: RiSparkling2Line,
    runs: false,
    talks: ["message"],
  },
  "sla.warning": {
    group: "sla",
    name: "SLA about to breach",
    detail: "Raise the priority in time.",
    icon: RiAlarmWarningLine,
    runs: false,
    talks: [],
  },
  "sla.breached": {
    group: "sla",
    name: "SLA breached",
    detail: "Escalate a missed target.",
    icon: RiErrorWarningLine,
    runs: false,
    talks: [],
  },
  reusable: {
    group: "reuse",
    name: "Run from other workflows",
    detail: "Share steps between workflows.",
    icon: RiLinksLine,
    runs: false,
    talks: customerKinds,
    dimmi: true,
  },
};

/*
 * allowsStep reports whether a step kind may appear under the trigger:
 * actions and logic everywhere, steps that speak to the customer only
 * where the trigger may speak, and dimmi only where it may take over.
 */
export function allowsStep(kind: TriggerKind, step: StepKind): boolean {
  if (step === "dimmi") {
    return triggers[kind].dimmi === true;
  }

  return !customerKinds.includes(step) || triggers[kind].talks.includes(step);
}

/*
 * tintOf is the colour of a trigger's group, worn by its mark wherever
 * the trigger shows.
 */
export function tintOf(kind: TriggerKind): ChipColor {
  return triggerGroups.find((group) => group.id === triggers[kind].group)?.tint ?? "neutral";
}

/*
 * triggersIn lists a group's triggers that run today, in the order they
 * are offered, keeping only those whose name, example or group name holds every word
 * of the query.
 */
export function triggersIn(group: TriggerGroup, query = ""): TriggerKind[] {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean);
  const groupName = triggerGroups.find((entry) => entry.id === group)?.name ?? "";

  return (Object.keys(triggers) as TriggerKind[]).filter((kind) => {
    const trigger = triggers[kind];
    const text = `${trigger.name} ${trigger.detail} ${groupName}`.toLowerCase();

    return trigger.runs && trigger.group === group && words.every((word) => text.includes(word));
  });
}

export function isTrigger(value: unknown): value is TriggerKind {
  return typeof value === "string" && Object.hasOwn(triggers, value);
}
