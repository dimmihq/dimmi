/*
 * How each kind of workflow step and trigger looks: its name, its icon,
 * and the BoardUI chip tint its mark wears, so the builder reads at a
 * glance: messages and questions blue, logic purple, Dimmi lime, team
 * actions neutral. Every kind the server knows has a look, so a definition
 * made through the API reads on the canvas too.
 */
import {
  RiAlarmWarningLine,
  RiCheckboxCircleLine,
  RiContactsLine,
  RiFileList3Line,
  RiFlag2Line,
  RiGitBranchLine,
  RiGlobalLine,
  RiHashtag,
  RiLinksLine,
  RiListRadio,
  RiLoopRightLine,
  RiMessage2Line,
  RiPriceTag3Line,
  RiQuestionAnswerLine,
  RiShareForwardLine,
  RiSignpostLine,
  RiSparkling2Line,
  RiStackLine,
  RiStickyNoteLine,
  RiTicket2Line,
  RiTimeLine,
  RiTimerLine,
  RiUserShared2Line,
  RiUserUnfollowLine,
  RiZzzLine,
} from "@remixicon/react";
import type { ComponentType } from "react";

import type { ChipColor } from "@/components/base/badges/chip";

import { type TriggerKind, tintOf, triggers } from "./triggers";
import type { StepKind } from "./workflows";

export interface KindLook {
  name: string;
  icon: ComponentType<{ className?: string }>;
  tint: ChipColor;
}

export const stepLook: Record<StepKind, KindLook> = {
  message: { name: "Send a message", icon: RiMessage2Line, tint: "blue" },
  buttons: { name: "Reply buttons", icon: RiListRadio, tint: "blue" },
  collect: { name: "Collect data", icon: RiQuestionAnswerLine, tint: "blue" },
  reply_time: { name: "Show reply time", icon: RiTimeLine, tint: "blue" },
  branch: { name: "Branch", icon: RiGitBranchLine, tint: "purple" },
  classify: { name: "Sort by topic", icon: RiSignpostLine, tint: "purple" },
  wait: { name: "Wait", icon: RiTimerLine, tint: "purple" },
  run_workflow: { name: "Run a workflow", icon: RiLinksLine, tint: "purple" },
  dimmi: { name: "Let dimmi handle", icon: RiSparkling2Line, tint: "lime" },
  assign: { name: "Assign", icon: RiUserShared2Line, tint: "neutral" },
  unassign: { name: "Unassign", icon: RiUserUnfollowLine, tint: "neutral" },
  tag: { name: "Tag", icon: RiHashtag, tint: "neutral" },
  untag: { name: "Remove tags", icon: RiPriceTag3Line, tint: "neutral" },
  priority: { name: "Set priority", icon: RiFlag2Line, tint: "neutral" },
  note: { name: "Add a note", icon: RiStickyNoteLine, tint: "neutral" },
  update_contact: { name: "Update contact", icon: RiContactsLine, tint: "neutral" },
  snooze: { name: "Snooze", icon: RiZzzLine, tint: "neutral" },
  close: { name: "Close", icon: RiCheckboxCircleLine, tint: "neutral" },
  apply_sla: { name: "Apply an SLA", icon: RiAlarmWarningLine, tint: "neutral" },
  http_request: { name: "Call your system", icon: RiGlobalLine, tint: "neutral" },
  create_ticket: { name: "Create a ticket", icon: RiTicket2Line, tint: "cyan" },
  set_ticket_state: { name: "Set ticket state", icon: RiLoopRightLine, tint: "cyan" },
  set_ticket_fields: { name: "Set ticket fields", icon: RiFileList3Line, tint: "cyan" },
  escalate: { name: "Escalate to a team", icon: RiShareForwardLine, tint: "cyan" },
  add_to_tracker: { name: "Add to a tracker", icon: RiStackLine, tint: "cyan" },
};

/*
 * triggerLook is how a trigger's card looks on the canvas: named Trigger,
 * with the trigger's own icon in its group's colour.
 */
export function triggerLook(kind: TriggerKind): KindLook {
  return { name: "Trigger", icon: triggers[kind].icon, tint: tintOf(kind) };
}
