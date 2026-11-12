/*
 * The ticket setup the builder's ticket steps and conditions pick from,
 * read from the workspace: types with their fields, states, open
 * trackers and SLA policies. Archived ones are left out of the pickers
 * but still named on cards that hold them.
 */
import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";

import { slaPoliciesQuery } from "@/lib/api/sla";
import { ticketStatesQuery, ticketsQuery, ticketTypesQuery } from "@/lib/api/tickets";

import type { TicketLists } from "./ticket-fields";

/*
 * The step kinds whose settings TicketStepFields shows.
 */
export const ticketStepKinds = [
  "create_ticket",
  "set_ticket_state",
  "set_ticket_fields",
  "escalate",
  "add_to_tracker",
  "apply_sla",
] as const;

export function useTicketLists(workspace: string): TicketLists {
  const types = useQuery({ ...ticketTypesQuery(workspace), retry: false }).data;
  const states = useQuery({ ...ticketStatesQuery(workspace), retry: false }).data;
  const trackers = useQuery({ ...ticketsQuery(workspace, { kind: "tracker", active: true }), retry: false }).data;
  const policies = useQuery({ ...slaPoliciesQuery(workspace), retry: false }).data;

  return useMemo(
    () => ({
      types: (types ?? [])
        .filter((type) => !type.archived_at)
        .map((type) => ({
          id: type.id,
          name: type.name,
          kind: type.kind,
          fields: type.fields.filter((field) => !field.archived_at).map((field) => [field.key, field.label] as const),
        })),
      states: (states ?? []).filter((state) => !state.archived_at).map((state) => [state.id, state.name] as const),
      trackers: (trackers?.items ?? []).map((ticket) => [ticket.id, `${ticket.reference} ${ticket.title}`] as const),
      policies: (policies ?? [])
        .filter((policy) => !policy.archived_at)
        .map((policy) => [policy.id, policy.name] as const),
    }),
    [types, states, trackers, policies],
  );
}

/*
 * ticketNames names every id the ticket lists hold, for the cards.
 */
export function ticketNames(lists: TicketLists): ReadonlyMap<string, string> {
  return new Map([
    ...lists.types.map((type) => [type.id, type.name] as const),
    ...lists.states,
    ...lists.trackers,
    ...lists.policies,
  ]);
}
