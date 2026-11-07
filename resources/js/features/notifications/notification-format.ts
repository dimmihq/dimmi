/*
 * How a notification reads in the notification center: the icon and tone
 * of its trigger, how long ago it changed, how often it happened, and,
 * for an action that waits for approval, the buttons while it waits and
 * how it ended once someone decided. Kept apart from the page so it can
 * be tested without rendering.
 */
import {
  RiAtLine,
  RiChat3Line,
  RiReplyLine,
  RiRobot2Line,
  RiShieldCheckLine,
  RiTeamLine,
  RiTimeLine,
  RiUserReceived2Line,
} from "@remixicon/react";

import type {
  NotificationCenterItem,
  NotificationCenterStatus,
} from "@/components/application/notification-center/notification-center";
import type { MemberNotification } from "@/lib/api/notifications";
import { formatRelative } from "@/lib/i18n/format";

type Visual = { icon: NotificationCenterItem["icon"]; status: NotificationCenterStatus };

const visuals: Record<string, Visual> = {
  new_conversation: { icon: RiChat3Line, status: "information" },
  customer_replied: { icon: RiReplyLine, status: "information" },
  conversation_assigned: { icon: RiUserReceived2Line, status: "success" },
  mentioned: { icon: RiAtLine, status: "neutral" },
  ai_handoff_requested: { icon: RiRobot2Line, status: "error" },
  ai_action_approval_requested: { icon: RiShieldCheckLine, status: "error" },
  snooze_expired: { icon: RiTimeLine, status: "neutral" },
  team_joined: { icon: RiTeamLine, status: "success" },
};

const endings: Record<string, string> = {
  approved: "Approved",
  running: "Approved",
  succeeded: "Approved",
  failed: "Approved, but it failed",
  declined: "Declined",
  expired: "Expired before anyone decided",
  canceled: "Canceled",
};

/*
 * isApproval reports whether the notification asks for a decision that
 * nobody made yet.
 */
export function isPendingApproval(n: MemberNotification): boolean {
  return n.trigger === "ai_action_approval_requested" && n.action_call_id !== null && !n.resolved;
}

/*
 * centerItem shapes a notification for the notification center. busy
 * names the action in progress on it, if any.
 */
export function centerItem(n: MemberNotification, now: Date, busy?: string): NotificationCenterItem {
  const visual = visuals[n.trigger] ?? { icon: RiChat3Line, status: "neutral" as const };
  const repeated = n.count > 1 ? ` · ${n.count} times` : "";
  const item: NotificationCenterItem = {
    id: n.id,
    category: n.trigger,
    group: "",
    title: n.title,
    description: n.body,
    timestamp: formatRelative(n.updated_at, { now: now.getTime(), style: "short" }) + repeated,
    unread: !n.read,
    status: visual.status,
    icon: visual.icon,
  };
  if (isPendingApproval(n)) {
    item.actions = [
      { id: "approve", label: "Approve", variant: "primary", loading: busy === "approve" },
      { id: "decline", label: "Decline", variant: "secondary", loading: busy === "decline" },
    ];
  } else if (n.resolved) {
    item.note = endings[n.resolved] ?? n.resolved;
  }
  return item;
}

/*
 * pathOf turns a notification's link into an address inside the app.
 */
export function pathOf(link: string): string {
  try {
    const url = new URL(link);
    return url.pathname + url.search + url.hash;
  } catch {
    return link;
  }
}
