/*
 * A ticket opened in the inbox, in the place a conversation opens. A
 * customer's request opens its conversation, where its replies and its
 * ticket panel live. Trackers and incidents open their own views, and
 * any other ticket, such as back-office work, opens as a thread of its
 * notes and changes beside its details.
 */
import { RiCoupon3Line } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Navigate } from "@tanstack/react-router";
import { ConversationSkeleton } from "@/features/inbox/skeletons";
import { useAccount } from "@/features/settings/use-account";
import { EmptyState, Panel } from "@/features/shell/page";
import { ticketQuery } from "@/lib/api/tickets";

import { IncidentPage } from "./incident-page";
import { TicketThread } from "./ticket-thread";
import { TrackerPage } from "./tracker-page";

/*
 * isTicketId reports whether an opened item's id names a ticket rather
 * than a conversation.
 */
export function isTicketId(id: string): boolean {
  return id.startsWith("tkt_");
}

export function TicketOpen({ view, id }: { view: string; id: string }) {
  const workspace = useAccount().workspace.id;
  const { data: ticket, isError } = useQuery(ticketQuery(workspace, id));

  if (isError) {
    return <TicketMissing />;
  }
  if (!ticket) {
    return (
      <Panel label="Ticket" className="flex-1">
        <ConversationSkeleton />
      </Panel>
    );
  }
  if (ticket.kind === "customer" && ticket.conversation_id) {
    return (
      <Navigate to="/d/inbox/$view/$conversation" params={{ view, conversation: ticket.conversation_id }} replace />
    );
  }
  if (ticket.kind === "tracker") {
    return <TrackerPage key={ticket.id} ticketId={ticket.id} />;
  }
  if (ticket.kind === "incident") {
    return <IncidentPage key={ticket.id} ticketId={ticket.id} />;
  }

  return <TicketThread key={ticket.id} view={view} ticket={ticket} />;
}

function TicketMissing() {
  return (
    <Panel label="Ticket" className="flex-1">
      <EmptyState
        icon={RiCoupon3Line}
        title="This ticket is not here"
        description="It may have been deleted, or it is not yours to see."
      />
    </Panel>
  );
}
