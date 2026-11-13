/*
 * What a tracker reaches and what it says: the conversations linked to
 * it with each customer's company, plan and revenue, and the message to
 * all of them with who it would hold back and how each delivery went.
 */
import { queryOptions } from "@tanstack/react-query";

import { request, requestBody } from "./client";

/*
 * TrackerConversation is one conversation a tracker links,
 * with who wrote it and the ticket of its own, if any.
 */
export interface TrackerConversation {
  object: "tracker_conversation";
  link_id: string;
  conversation_id: string;
  conversation_number: number;
  subject: string | null;
  status: string;
  contact_id: string;
  contact_name: string | null;
  contact_email: string | null;
  company_id: string | null;
  company_name: string | null;
  company_plan: string | null;
  company_revenue: number;
  ticket_id: string | null;
  ticket_reference: string | null;
  ticket_state_id: string | null;
  last_message_at: string | null;
  linked_at: string;
}

/*
 * Impact is who a tracker reaches, all of them, not only the page read.
 */
export interface Impact {
  conversations: number;
  customers: number;
  companies: number;
  revenue: number;
}

export interface TrackerConversations {
  rows: TrackerConversation[];
  next: string | null;
  impact: Impact;
}

/*
 * HeldReason is why a customer would not get the message: they wrote
 * since the tracker last changed, or a teammate already told them.
 */
export type HeldReason = "customer_wrote_since" | "teammate_already_replied" | (string & {});

export type UpdateStatus = "queued" | "sending" | "sent" | "partial" | "failed";
export type DeliveryStatus = "pending" | "sent" | "held" | "failed";

export interface UpdateDelivery {
  conversation_id: string;
  conversation_number: number;
  contact_name: string | null;
  contact_email: string | null;
  status: DeliveryStatus;
  reason: string | null;
  attempts: number;
  sent_at: string | null;
}

/*
 * TicketUpdate is one message to everyone linked and how it went.
 */
export interface TicketUpdate {
  object: "ticket_update";
  id: string;
  ticket_id: string;
  body: string;
  resolve: boolean;
  status: UpdateStatus;
  recipients: number;
  delivered: number;
  held: number;
  failed: number;
  author_id: string | null;
  deliveries: UpdateDelivery[];
  created_at: string;
  finished_at: string | null;
}

/*
 * UpdatePlan is what sending would do, or did: how many it reaches, who
 * is held back and why, and the update once it is really sent.
 */
export interface UpdatePlan {
  recipients: number;
  held: { conversation_id: string; reason: HeldReason }[];
  update: TicketUpdate | null;
}

export const trackerKeys = {
  conversations: (workspace: string, ticket: string) =>
    ["workspace", workspace, "tickets", "tracker", ticket, "conversations"] as const,
  updates: (workspace: string, ticket: string) =>
    ["workspace", workspace, "tickets", "tracker", ticket, "updates"] as const,
};

const noImpact: Impact = { conversations: 0, customers: 0, companies: 0, revenue: 0 };

/*
 * trackerConversationsQuery reads every conversation the ticket links,
 * page after page, with the totals.
 */
export function trackerConversationsQuery(workspace: string, ticket: string) {
  return queryOptions({
    queryKey: trackerKeys.conversations(workspace, ticket),
    queryFn: async (): Promise<TrackerConversations> => {
      const rows: TrackerConversation[] = [];
      let cursor: string | null = null;
      let impact = noImpact;
      for (let page = 0; page < 20; page++) {
        const path: string = `/tickets/${ticket}/conversations${cursor ? `?cursor=${encodeURIComponent(cursor)}` : ""}`;
        const body = await requestBody<{
          data: TrackerConversation[];
          meta?: { next_cursor?: string | null; impact?: Impact };
        }>("GET", path, undefined, { workspace });
        rows.push(...body.data);
        impact = body.meta?.impact ?? impact;
        cursor = body.meta?.next_cursor ?? null;
        if (!cursor) {
          break;
        }
      }

      return { rows, next: cursor, impact };
    },
    enabled: workspace !== "" && ticket !== "",
  });
}

/*
 * ticketUpdatesQuery reads the messages sent to everyone, newest first.
 * While one is still going out it asks again every two seconds.
 */
export function ticketUpdatesQuery(workspace: string, ticket: string) {
  return queryOptions({
    queryKey: trackerKeys.updates(workspace, ticket),
    queryFn: () => request<TicketUpdate[]>("GET", `/tickets/${ticket}/updates`, undefined, { workspace }),
    enabled: workspace !== "" && ticket !== "",
    refetchInterval: (query) =>
      (query.state.data ?? []).some((u) => u.status === "queued" || u.status === "sending") ? 2000 : false,
  });
}

export function sendTicketUpdate(
  workspace: string,
  ticket: string,
  message: { body: string; resolve?: boolean; dry_run?: boolean },
) {
  return request<UpdatePlan>("POST", `/tickets/${ticket}/updates`, message, { workspace });
}
