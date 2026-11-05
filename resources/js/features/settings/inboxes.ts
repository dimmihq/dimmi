/*
 * What the inbox settings pages share: who may change inboxes, and how
 * a reopen window, an assignment mode and a reason a teammate is passed
 * over read in words.
 */
import { useQuery } from "@tanstack/react-query";

import type { AssignmentMode, QueueOrder, WhyNot } from "@/lib/api/inboxes";
import { callerQuery } from "@/lib/api/workspace";

/*
 * usePermission reports whether the server grants the person the
 * permission in the workspace, such as inboxes:manage to change inboxes
 * or settings:read to see how assignment stands.
 */
export function usePermission(workspace: string, permission: string): boolean {
  const { data: caller } = useQuery(callerQuery(workspace));

  return caller?.member?.permissions.includes(permission) ?? false;
}

/*
 * reopenWindows are the reopen windows offered, in days, 0 for never.
 */
export const reopenWindows = [0, 7, 30, 90];

export function reopenLabel(days: number): string {
  if (days === 0) {
    return "Never reopen";
  }

  return days === 1 ? "1 day" : `${days} days`;
}

export const assignmentModes: Record<AssignmentMode, string> = {
  manual: "Manual",
  round_robin: "Round robin",
  balanced: "Balanced",
};

export const queueOrders: Record<QueueOrder, string> = {
  oldest_waiting: "Oldest waiting first",
  priority_then_oldest: "Highest priority first",
};

const reasons: Record<WhyNot, string> = {
  away: "Away",
  at_capacity: "At capacity",
  at_channel_capacity: "At channel capacity",
};

/*
 * poolLine says how many open conversations a teammate holds and, when
 * they cannot take the next one, why.
 */
export function poolLine(open: number, why: WhyNot | null): string {
  const held = open === 1 ? "1 open" : `${open} open`;

  return why ? `${held} · ${reasons[why]}` : held;
}

/*
 * keyOf returns the key in the record whose label is the one given.
 */
export function keyOf<K extends string>(labels: Record<K, string>, label: string): K | undefined {
  return (Object.keys(labels) as K[]).find((key) => labels[key] === label);
}
