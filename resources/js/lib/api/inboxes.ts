/*
 * The workspace's inboxes on the server: listing them, one inbox with
 * its settings and how its automatic assignment stands, and the calls
 * that add, change and delete one. Settings changes are merge patches:
 * what a change leaves out stays as it is.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";
import type { Named } from "./inbox";

export type AssignmentMode = "manual" | "round_robin" | "balanced";

export type QueueOrder = "oldest_waiting" | "priority_then_oldest";

export interface InboxAssignment {
  mode: AssignmentMode;
  include_away: boolean;
  overflow_team_id: string | null;
  queue_order: QueueOrder;
  default_capacity: number | null;
  capacity_by_channel: Record<string, number>;
}

export interface InboxSettings {
  reopen_window_days: number;
  assign_on_reply: boolean;
  color: string;
  assignment: InboxAssignment;
}

export interface Inbox {
  id: string;
  name: string;
  default: boolean;
  default_team_id: string | null;
  access: InboxAccess;
  settings: InboxSettings;
  /*
   * ticket_type_id is the customer ticket type every new conversation of
   * the inbox becomes, null when they stay plain conversations.
   */
  ticket_type_id?: string | null;
  created_at: string;
  updated_at: string;
}

/*
 * InboxAccess is who may open an inbox: everyone, or only the teams and
 * teammates on its list. Owners and admins open every inbox.
 */
export interface InboxAccess {
  mode: "everyone" | "restricted";
  team_ids: string[];
  member_ids: string[];
}

/*
 * InboxChange is what an update sends: any of the name, the default team
 * (null for none), the ticket type new conversations become (null for
 * none) and a patch on the settings.
 */
export interface InboxChange {
  name?: string;
  default_team_id?: string | null;
  ticket_type_id?: string | null;
  settings?: Partial<Omit<InboxSettings, "assignment">> & { assignment?: Partial<InboxAssignment> };
}

export type WhyNot = "away" | "at_capacity" | "at_channel_capacity";

export interface PoolMember {
  member: Named;
  availability: string;
  open_count: number;
  capacity: number | null;
  channel_capacity: number | null;
  eligible: boolean;
  why_not: WhyNot | null;
}

export interface AssignmentStatus {
  mode: AssignmentMode;
  channel_type: string;
  members: PoolMember[];
  waiting: { count: number; oldest_at: string | null; reasons: Record<string, number> | null };
}

export function inboxesQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "inboxes"],
    queryFn: () => request<Inbox[]>("GET", "/inboxes", undefined, { workspace }),
  });
}

export function inboxQuery(workspace: string, inbox: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "inboxes", inbox],
    queryFn: () => request<Inbox>("GET", `/inboxes/${encodeURIComponent(inbox)}`, undefined, { workspace }),
    retry: false,
  });
}

export function assignmentStatusQuery(workspace: string, inbox: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "inboxes", inbox, "assignment-status"],
    queryFn: () =>
      request<AssignmentStatus>("GET", `/inboxes/${encodeURIComponent(inbox)}/assignment-status`, undefined, {
        workspace,
      }),
  });
}

export function createInbox(workspace: string, name: string): Promise<Inbox> {
  return request<Inbox>("POST", "/inboxes", { name }, { workspace });
}

export function updateInbox(workspace: string, inbox: string, change: InboxChange): Promise<Inbox> {
  return request<Inbox>("PATCH", `/inboxes/${encodeURIComponent(inbox)}`, change, { workspace });
}

/*
 * sameAccess reports whether two accesses hold the same people, in any
 * order.
 */
export function sameAccess(a: InboxAccess, b: InboxAccess): boolean {
  const same = (x: string[], y: string[]) => x.length === y.length && x.every((id) => y.includes(id));

  return a.mode === b.mode && same(a.team_ids, b.team_ids) && same(a.member_ids, b.member_ids);
}

/*
 * updateInboxAccess replaces who may open the inbox.
 */
export function updateInboxAccess(workspace: string, inbox: string, access: InboxAccess): Promise<Inbox> {
  return request<Inbox>("PUT", `/inboxes/${encodeURIComponent(inbox)}/access`, access, { workspace });
}

/*
 * deleteInbox deletes the inbox, moving its conversations and channels
 * to moveTo when it holds any.
 */
export function deleteInbox(workspace: string, inbox: string, moveTo?: string): Promise<void> {
  const query = moveTo ? `?move_to=${encodeURIComponent(moveTo)}` : "";

  return request<void>("DELETE", `/inboxes/${encodeURIComponent(inbox)}${query}`, undefined, { workspace });
}
