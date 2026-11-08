/*
 * What customers did in the team's product, as the team's systems report
 * it: the names events go by and each person's summary of each event.
 * Reading takes contacts:read; changing a name takes attributes:manage.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export interface EventDefinition {
  id: string;
  name: string;
  label: string;
  description: string | null;
  origin: "created" | "observed";
  status: "active" | "archived";
  total_count: number;
  last_seen_at: string | null;
}

export interface EventSummary {
  name: string;
  label: string;
  count: number;
  last_7_days: number;
  last_30_days: number;
  first_at: string;
  last_at: string;
  last_metadata: Record<string, unknown>;
}

export function eventsQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "event-definitions"],
    queryFn: () => request<EventDefinition[]>("GET", "/event-definitions", undefined, { workspace }),
  });
}

export function contactEventsQuery(workspace: string, contact: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "contacts", contact, "event-summaries"],
    queryFn: () =>
      request<EventSummary[]>("GET", `/contacts/${encodeURIComponent(contact)}/event-summaries`, undefined, {
        workspace,
      }),
  });
}

export function updateEvent(
  workspace: string,
  id: string,
  changes: { label?: string; status?: "active" | "archived" },
): Promise<EventDefinition> {
  return request<EventDefinition>("PATCH", `/event-definitions/${encodeURIComponent(id)}`, changes, { workspace });
}
