/*
 * What a tracker or an incident reaches and what it says: the
 * conversations linked to it with each customer's company, plan and
 * revenue, the message to all of them with who it would hold back and
 * how each delivery went, an incident's public log, and the incidents
 * active right now. A section whose endpoint the server does not have
 * yet answers null, so its screen can leave it out.
 */
import { queryOptions } from "@tanstack/react-query";

import { ApiError, request, requestBody } from "./client";
import type { Ticket } from "./tickets";

/*
 * TrackerConversation is one conversation a tracker or incident links,
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
 * Impact is who a tracker or incident reaches, all of them, not only
 * the page read.
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

export interface PublicUpdate {
  object: "ticket_public_update";
  id: string;
  ticket_id: string;
  body: string;
  state_id: string | null;
  update_id: string | null;
  author_id: string | null;
  source: string;
  created_at: string;
}

/*
 * TrackerSummary is the AI's read of a tracker: what customers report,
 * how many, which plans. The server may not send it yet.
 */
export interface TrackerSummary {
  text: string;
  generated_at?: string;
}

export const trackerKeys = {
  conversations: (workspace: string, ticket: string) =>
    ["workspace", workspace, "tickets", "tracker", ticket, "conversations"] as const,
  updates: (workspace: string, ticket: string) =>
    ["workspace", workspace, "tickets", "tracker", ticket, "updates"] as const,
  log: (workspace: string, ticket: string) => ["workspace", workspace, "tickets", "tracker", ticket, "log"] as const,
  active: (workspace: string) => ["workspace", workspace, "tickets", "incidents", "active"] as const,
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

export function publicUpdatesQuery(workspace: string, ticket: string) {
  return queryOptions({
    queryKey: trackerKeys.log(workspace, ticket),
    queryFn: () => request<PublicUpdate[]>("GET", `/tickets/${ticket}/public-updates`, undefined, { workspace }),
    enabled: workspace !== "" && ticket !== "",
  });
}

export function postPublicUpdate(
  workspace: string,
  ticket: string,
  post: { body: string; state_id?: string; send_to_linked?: boolean },
) {
  return request<PublicUpdate>("POST", `/tickets/${ticket}/public-updates`, post, { workspace });
}

/*
 * activeIncidentsQuery reads the incidents not resolved yet, for the
 * pins beside the inbox. The counts stay live: the list is read again
 * every thirty seconds.
 */
export function activeIncidentsQuery(workspace: string) {
  return queryOptions({
    queryKey: trackerKeys.active(workspace),
    queryFn: () => request<Ticket[]>("GET", "/tickets?kind=incident&active=true&limit=10", undefined, { workspace }),
    enabled: workspace !== "",
    refetchInterval: 30_000,
  });
}

/*
 * optional reads an endpoint that may not exist yet: a 404 or 405
 * answers null instead of failing.
 */
export async function optional<T>(read: () => Promise<T>): Promise<T | null> {
  try {
    return await read();
  } catch (error) {
    if (error instanceof ApiError && (error.status === 404 || error.status === 405)) {
      return null;
    }
    throw error;
  }
}
