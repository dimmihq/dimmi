import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";

import { useAccount } from "@/features/settings/use-account";
import { IncidentPage } from "@/features/tickets/incident-page";
import { TicketMissing, TicketPage } from "@/features/tickets/ticket-page";
import { TrackerPage } from "@/features/tickets/tracker-page";
import { ticketQuery } from "@/lib/api/tickets";

export const Route = createFileRoute("/d/tickets/$ticket")({
  component: TicketRoute,
});

/*
 * TicketRoute shows a ticket by its kind: trackers and incidents on
 * their own pages, everything else on the ticket page.
 */
function TicketRoute() {
  const { ticket } = Route.useParams();
  const workspace = useAccount().workspace.id;
  const { data, isError } = useQuery(ticketQuery(workspace, ticket));

  if (isError) {
    return <TicketMissing />;
  }
  if (data?.kind === "incident") {
    return <IncidentPage key={ticket} ticketId={ticket} />;
  }
  if (data?.kind === "tracker") {
    return <TrackerPage key={ticket} ticketId={ticket} />;
  }
  if (data) {
    return <TicketPage key={data.id} ticket={data} />;
  }

  return null;
}
