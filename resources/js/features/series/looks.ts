/*
 * How each kind of series step looks: its name, icon and BoardUI chip
 * tint, in the same families the workflow builder uses, so messages
 * read blue, timing and logic purple, AI rose and housekeeping neutral.
 * The steps offered when adding one are grouped the same way. Calling
 * another system goes through a workspace action; the older webhook step
 * still runs and opens, but is no longer offered.
 */

import {
  RiChat3Line,
  RiDoorOpenLine,
  RiEqualizerLine,
  RiFileTextLine,
  RiFlashlightLine,
  RiGitBranchLine,
  RiHourglassLine,
  RiLoginCircleLine,
  RiMailLine,
  RiPriceTag3Line,
  RiScan2Line,
  RiSparkling2Line,
  RiSplitCellsHorizontal,
  RiTimerLine,
  RiUserUnfollowLine,
  RiWebhookLine,
} from "@remixicon/react";
import { SlackMark } from "@/components/application/brand/slack-mark";

import type { KindLook } from "@/features/automation/step-kinds";
import type { NodeKind, SeriesStatus } from "@/lib/api/series";

export const nodeLook: Record<NodeKind, KindLook> = {
  entry: { name: "Who enters", icon: RiLoginCircleLine, tint: "lime" },
  email: { name: "Email", icon: RiMailLine, tint: "blue" },
  chat: { name: "Chat", icon: RiChat3Line, tint: "blue" },
  post: { name: "Post", icon: RiFileTextLine, tint: "blue" },
  slack: { name: "Slack", icon: SlackMark, tint: "neutral" },
  wait: { name: "Wait", icon: RiTimerLine, tint: "purple" },
  wait_for: { name: "Wait for", icon: RiHourglassLine, tint: "purple" },
  condition: { name: "Check", icon: RiGitBranchLine, tint: "purple" },
  split: { name: "Split test", icon: RiSplitCellsHorizontal, tint: "purple" },
  ai_path: { name: "AI path", icon: RiSparkling2Line, tint: "rose" },
  tag: { name: "Tag", icon: RiPriceTag3Line, tint: "gray" },
  untag: { name: "Remove tag", icon: RiPriceTag3Line, tint: "gray" },
  set_attribute: { name: "Set attribute", icon: RiEqualizerLine, tint: "gray" },
  topic: { name: "Topic", icon: RiUserUnfollowLine, tint: "gray" },
  webhook: { name: "Webhook", icon: RiWebhookLine, tint: "gray" },
  exit: { name: "Leave", icon: RiDoorOpenLine, tint: "gray" },
  extract: { name: "Extract with AI", icon: RiScan2Line, tint: "rose" },
  run_action: { name: "Run an action", icon: RiFlashlightLine, tint: "gray" },
};

export const addGroups: [string, NodeKind[]][] = [
  ["Messages", ["email", "chat", "post", "slack"]],
  ["Timing", ["wait", "wait_for"]],
  ["Logic", ["condition", "split", "ai_path", "extract"]],
  ["Actions", ["tag", "untag", "set_attribute", "topic", "run_action", "exit"]],
];

export const statusLook: Record<SeriesStatus, { name: string; tint: "lime" | "gray" | "yellow" }> = {
  live: { name: "Live", tint: "lime" },
  draft: { name: "Draft", tint: "gray" },
  paused: { name: "Paused", tint: "yellow" },
  archived: { name: "Archived", tint: "gray" },
};

export const endReasonName: Record<string, string> = {
  finished: "Finished",
  goal: "Reached the goal",
  exit_rules: "Matched the exit rules",
  replied: "Replied",
  unsubscribed: "Unsubscribed",
  undeliverable: "Could not be reached",
  removed: "Removed by a teammate",
  series_stopped: "Series archived",
  node_removed: "Step removed",
  step_budget: "Too many steps",
  subject_gone: "Contact gone",
  replaced: "Replaced by a newer test",
};

export function percent(part: number, whole: number): string {
  if (whole === 0) {
    return "0%";
  }

  return `${Math.round((part / whole) * 1000) / 10}%`;
}
