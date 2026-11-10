/*
 * How each kind of series step looks: its name, icon and BoardUI chip
 * tint, in the same families the workflow builder uses, so messages
 * read blue, timing and logic purple, and housekeeping neutral. The
 * steps offered when adding one are grouped the same way.
 */
import {
  RiChat3Line,
  RiDoorOpenLine,
  RiFileTextLine,
  RiGitBranchLine,
  RiLayoutTopLine,
  RiLoginCircleLine,
  RiMailLine,
  RiPriceTag3Line,
  RiTimerLine,
  RiWebhookLine,
} from "@remixicon/react";

import type { KindLook } from "@/features/automation/step-kinds";

import type { NodeKind, SeriesStatus } from "./sample";

export const nodeLook: Record<NodeKind, KindLook> = {
  entry: { name: "Who enters", icon: RiLoginCircleLine, tint: "lime" },
  email: { name: "Email", icon: RiMailLine, tint: "blue" },
  chat: { name: "Chat", icon: RiChat3Line, tint: "blue" },
  post: { name: "Post", icon: RiFileTextLine, tint: "blue" },
  banner: { name: "Banner", icon: RiLayoutTopLine, tint: "blue" },
  wait: { name: "Wait", icon: RiTimerLine, tint: "purple" },
  branch: { name: "Check", icon: RiGitBranchLine, tint: "purple" },
  tag: { name: "Tag", icon: RiPriceTag3Line, tint: "neutral" },
  webhook: { name: "Send a webhook", icon: RiWebhookLine, tint: "neutral" },
  exit: { name: "Leave", icon: RiDoorOpenLine, tint: "neutral" },
};

export const addGroups: [string, NodeKind[]][] = [
  ["Messages", ["email", "chat", "post", "banner"]],
  ["Timing and logic", ["wait", "branch"]],
  ["Actions", ["tag", "webhook", "exit"]],
];

export const statusLook: Record<SeriesStatus, { name: string; tint: "lime" | "gray" | "yellow" }> = {
  live: { name: "Live", tint: "lime" },
  draft: { name: "Draft", tint: "gray" },
  paused: { name: "Paused", tint: "yellow" },
};

export const messageKinds: NodeKind[] = ["email", "chat", "post", "banner"];
