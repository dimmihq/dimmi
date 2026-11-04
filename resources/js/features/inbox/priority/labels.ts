/*
 * How the screen names each priority level.
 */
import type { ConversationPriority } from "@/lib/api/inbox";

export const priorityLabel: Record<ConversationPriority, string> = {
  none: "No priority",
  low: "Low",
  medium: "Medium",
  high: "High",
  urgent: "Urgent",
};
