/*
 * A ticket's type and the states it may move to, read from the
 * workspace's setup.
 */
import { useQuery } from "@tanstack/react-query";

import { type Ticket, ticketStatesQuery, ticketTypesQuery } from "@/lib/api/tickets";

/*
 * useTicketSetup finds a ticket's type and the states it may move to.
 */
export function useTicketSetup(workspace: string, ticket: Ticket) {
  const { data: types = [] } = useQuery(ticketTypesQuery(workspace));
  const { data: states = [] } = useQuery(ticketStatesQuery(workspace));
  const type = types.find((t) => t.id === ticket.type_id);
  const allowed = states.filter((s) => !s.archived_at && (type?.state_ids.includes(s.id) ?? true));

  return { types, type, states: allowed.length > 0 ? allowed : states };
}
