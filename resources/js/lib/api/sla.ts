/*
 * The workspace's service level policies: targets in seconds counted in
 * a schedule's open hours, the warning before each, when clocks pause,
 * and what a breach changes. Workflows apply them to conversations;
 * new tickets of the types or priorities a policy names follow it.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export interface SLAPolicy {
  object: "sla_policy";
  id: string;
  name: string;
  description: string;
  schedule_id: string | null;
  first_reply_seconds: number | null;
  next_reply_seconds: number | null;
  follow_up_seconds: number | null;
  resolution_seconds: number | null;
  warn_before_seconds: number;
  pause_on_waiting_customer: boolean;
  pause_on_waiting_team: boolean;
  breach_priority: string | null;
  breach_team_id: string | null;
  ticket_type_ids: string[];
  priorities: string[];
  position: number;
  archived_at: string | null;
}

export function slaPoliciesQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "sla-policies"],
    queryFn: () => request<SLAPolicy[]>("GET", "/sla-policies", undefined, { workspace }),
    enabled: workspace !== "",
  });
}

/*
 * SLAPolicyChange is what a create or an edit sends. A target of 0
 * removes it, an empty schedule or breach team removes that, and a
 * breach priority of "none" removes the escalation; empty lists make the
 * policy match no ticket type or priority.
 */
export interface SLAPolicyChange {
  name?: string;
  description?: string;
  schedule_id?: string;
  first_reply_seconds?: number;
  next_reply_seconds?: number;
  follow_up_seconds?: number;
  resolution_seconds?: number;
  warn_before_seconds?: number;
  pause_on_waiting_customer?: boolean;
  pause_on_waiting_team?: boolean;
  breach_priority?: "none" | "low" | "medium" | "high" | "urgent";
  breach_team_id?: string;
  ticket_type_ids?: string[];
  priorities?: string[];
}

export function slaPolicyQuery(workspace: string, policy: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "sla-policies", policy],
    queryFn: () => request<SLAPolicy>("GET", `/sla-policies/${encodeURIComponent(policy)}`, undefined, { workspace }),
    enabled: workspace !== "" && policy !== "",
  });
}

export function createSLAPolicy(workspace: string, change: SLAPolicyChange): Promise<SLAPolicy> {
  return request<SLAPolicy>("POST", "/sla-policies", change, { workspace });
}

export function updateSLAPolicy(workspace: string, policy: string, change: SLAPolicyChange): Promise<SLAPolicy> {
  return request<SLAPolicy>("PATCH", `/sla-policies/${encodeURIComponent(policy)}`, change, { workspace });
}

/*
 * archiveSLAPolicy archives a policy: the clocks it still runs are
 * canceled and nothing new follows it.
 */
export function archiveSLAPolicy(workspace: string, policy: string): Promise<void> {
  return request<void>("DELETE", `/sla-policies/${encodeURIComponent(policy)}`, undefined, { workspace });
}
