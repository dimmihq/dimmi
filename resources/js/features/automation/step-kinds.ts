/*
 * How each kind of workflow step and trigger looks: its name, its icon,
 * and the BoardUI chip tint its mark wears, so the builder reads at a
 * glance: messages and questions blue, logic purple, Dimmi lime, team
 * actions neutral.
 */
import {
  RiChatSmile2Line,
  RiCheckboxCircleLine,
  RiFlag2Line,
  RiGitBranchLine,
  RiHashtag,
  RiListRadio,
  RiMessage2Line,
  RiPlayCircleLine,
  RiQuestionAnswerLine,
  RiSparkling2Line,
  RiStickyNoteLine,
  RiTimeLine,
  RiTimerLine,
  RiUserShared2Line,
} from "@remixicon/react";
import type { ComponentType } from "react";

import type { ChipColor } from "@/components/base/badges/chip";

import type { StepKind, TriggerKind } from "./workflows";

export interface KindLook {
  name: string;
  icon: ComponentType<{ className?: string }>;
  tint: ChipColor;
}

export const stepLook: Record<StepKind, KindLook> = {
  message: { name: "Send a message", icon: RiMessage2Line, tint: "blue" },
  buttons: { name: "Reply buttons", icon: RiListRadio, tint: "blue" },
  collect: { name: "Collect data", icon: RiQuestionAnswerLine, tint: "blue" },
  branch: { name: "Branch", icon: RiGitBranchLine, tint: "purple" },
  dimmi: { name: "Let Dimmi handle", icon: RiSparkling2Line, tint: "lime" },
  assign: { name: "Assign", icon: RiUserShared2Line, tint: "neutral" },
  tag: { name: "Tag", icon: RiHashtag, tint: "neutral" },
  priority: { name: "Mark as priority", icon: RiFlag2Line, tint: "neutral" },
  wait: { name: "Wait", icon: RiTimerLine, tint: "purple" },
  "reply-time": { name: "Show reply time", icon: RiTimeLine, tint: "blue" },
  note: { name: "Add a note", icon: RiStickyNoteLine, tint: "neutral" },
  close: { name: "Close", icon: RiCheckboxCircleLine, tint: "neutral" },
};

export const triggerLook: Record<TriggerKind, KindLook> = {
  opened: { name: "Trigger", icon: RiChatSmile2Line, tint: "orange" },
  "first-message": { name: "Trigger", icon: RiMessage2Line, tint: "orange" },
  "customer-quiet": { name: "Trigger", icon: RiTimerLine, tint: "orange" },
  "teammate-quiet": { name: "Trigger", icon: RiTimerLine, tint: "orange" },
  state: { name: "Trigger", icon: RiCheckboxCircleLine, tint: "orange" },
  reusable: { name: "Trigger", icon: RiPlayCircleLine, tint: "orange" },
};
