/*
 * What a tracker reaches: the conversations linked to it with each
 * customer's company, plan and revenue.
 */
import { queryOptions } from "@tanstack/react-query";

import { requestBody } from "./client";

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

export const trackerKeys = {
  conversations: (workspace: string, ticket: string) =>
    ["workspace", workspace, "tickets", "tracker", ticket, "conversations"] as const,
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
