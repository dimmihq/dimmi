/*
 * What can start a workflow. Triggers come in groups by who or what sets
 * them off, the customer arriving, the conversation going on, the team,
 * or dimmi itself, and each group has its own colour so a workflow's
 * trigger reads at a glance in the list, the picker and on the canvas.
 * A workflow keeps the trigger it was made with: its steps, conditions
 * and the data they can use depend on it.
 */
import {
  RiChat3Line,
  RiChatSmile2Line,
  RiCheckboxCircleLine,
  RiHourglassLine,
  RiLinksLine,
  RiMessage2Line,
  RiPagesLine,
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

export type TriggerGroup = "start" | "during" | "team" | "dimmi" | "reuse";

export type TriggerKind =
  | "page"
  | "opened"
  | "first-message"
  | "customer-message"
  | "customer-quiet"
  | "rated"
  | "teammate-message"
  | "note-added"
  | "assigned"
  | "state"
  | "teammate-quiet"
  | "dimmi-handover"
  | "dimmi-closed"
  | "reusable";

/*
 * Trigger is how a trigger is offered: its group, what sets it off, an
 * example of what to do with it, and its icon.
 */
export interface Trigger {
  group: TriggerGroup;
  name: string;
  detail: string;
  icon: ComponentType<{ className?: string }>;
}

export const triggerGroups: { id: TriggerGroup; name: string; detail: string; tint: ChipColor }[] = [
  { id: "start", name: "Start of conversation", detail: "When someone reaches out.", tint: "blue" },
  { id: "during", name: "During conversation", detail: "While you're talking.", tint: "purple" },
  { id: "team", name: "Team", detail: "When your team does something.", tint: "orange" },
  { id: "dimmi", name: "dimmi", detail: "When dimmi is done.", tint: "lime" },
  { id: "reuse", name: "Reusable", detail: "Steps you can use anywhere.", tint: "neutral" },
];

export const triggers: Record<TriggerKind, Trigger> = {
  page: {
    group: "start",
    name: "Customer visits a page",
    detail: "Greet people on key pages.",
    icon: RiPagesLine,
  },
  opened: {
    group: "start",
    name: "Customer opens the messenger",
    detail: "Say hi before they type.",
    icon: RiChatSmile2Line,
  },
  "first-message": {
    group: "start",
    name: "Customer sends their first message",
    detail: "Route it, or let dimmi answer.",
    icon: RiMessage2Line,
  },
  "customer-message": {
    group: "during",
    name: "Customer sends any message",
    detail: "Route every reply they send.",
    icon: RiChat3Line,
  },
  "customer-quiet": {
    group: "during",
    name: "Customer stops replying",
    detail: "Nudge them, then close.",
    icon: RiTimerLine,
  },
  rated: {
    group: "during",
    name: "Customer rates the conversation",
    detail: "Follow up on a bad rating.",
    icon: RiStarSmileLine,
  },
  "teammate-message": {
    group: "team",
    name: "Teammate sends a message",
    detail: "Tag or snooze after a reply.",
    icon: RiReplyLine,
  },
  "note-added": {
    group: "team",
    name: "Teammate adds a note",
    detail: "Act on notes, like escalations.",
    icon: RiStickyNoteLine,
  },
  assigned: {
    group: "team",
    name: "Assignment changes",
    detail: "Brief whoever picks it up.",
    icon: RiUserShared2Line,
  },
  state: {
    group: "team",
    name: "Conversation state changes",
    detail: "When it's closed, snoozed or reopened.",
    icon: RiCheckboxCircleLine,
  },
  "teammate-quiet": {
    group: "team",
    name: "Teammate stops replying",
    detail: "Reassign before the customer waits.",
    icon: RiHourglassLine,
  },
  "dimmi-handover": {
    group: "dimmi",
    name: "dimmi hands over",
    detail: "Brief the team when dimmi can't help.",
    icon: RiUserVoiceLine,
  },
  "dimmi-closed": {
    group: "dimmi",
    name: "dimmi closes a conversation",
    detail: "Ask for a rating, or tag it.",
    icon: RiSparkling2Line,
  },
  reusable: {
    group: "reuse",
    name: "Run from other workflows",
    detail: "Share steps between workflows.",
    icon: RiLinksLine,
  },
};

/*
 * tintOf is the colour of a trigger's group, worn by its mark wherever
 * the trigger shows.
 */
export function tintOf(kind: TriggerKind): ChipColor {
  return triggerGroups.find((group) => group.id === triggers[kind].group)?.tint ?? "neutral";
}

/*
 * triggersIn lists a group's triggers in the order they are offered,
 * keeping only those whose name, example or group name holds every word
 * of the query.
 */
export function triggersIn(group: TriggerGroup, query = ""): TriggerKind[] {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean);
  const groupName = triggerGroups.find((entry) => entry.id === group)?.name ?? "";

  return (Object.keys(triggers) as TriggerKind[]).filter((kind) => {
    const trigger = triggers[kind];
    const text = `${trigger.name} ${trigger.detail} ${groupName}`.toLowerCase();

    return trigger.group === group && words.every((word) => text.includes(word));
  });
}

export function isTrigger(value: unknown): value is TriggerKind {
  return typeof value === "string" && Object.hasOwn(triggers, value);
}
