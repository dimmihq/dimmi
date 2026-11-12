/*
 * Spikes of new conversations the detector suggests declaring as an
 * incident, which a teammate accepts (the incident is declared with every
 * conversation linked) or dismisses for an hour.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

/*
 * IncidentSuggestion is one suggested incident.
 */
export interface IncidentSuggestion {
  object: "incident_suggestion";
  id: string;
  title: string;
  summary: string;
  area: string | null;
  conversation_ids: string[];
  conversation_count: number;
  window_minutes: number;
  status: "open" | "accepted" | "dismissed";
  muted_until: string | null;
  ticket_id: string | null;
  created_at: string;
  updated_at: string;
}

export const incidentSuggestionKeys = {
  all: (workspace: string) => ["workspace", workspace, "incident-suggestions"] as const,
};

/*
 * incidentSuggestionsQuery reads the open suggestions. Realtime refreshes
 * it the moment a spike is found; the minute's poll is the fallback.
 */
export function incidentSuggestionsQuery(workspace: string) {
  return queryOptions({
    queryKey: incidentSuggestionKeys.all(workspace),
    queryFn: () => request<IncidentSuggestion[]>("GET", "/incident-suggestions", undefined, { workspace }),
    enabled: workspace !== "",
    refetchInterval: 60_000,
  });
}

/*
 * decideIncidentSuggestion accepts or dismisses a suggestion.
 */
export function decideIncidentSuggestion(workspace: string, id: string, status: "accepted" | "dismissed") {
  return request<IncidentSuggestion>("PATCH", `/incident-suggestions/${id}`, { status }, { workspace });
}
