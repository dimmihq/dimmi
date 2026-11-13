/*
 * A ticket opened in the inbox, in the place a conversation opens. A
 * customer's request opens its conversation, where its replies and its
 * ticket panel live. Back-office work opens the conversation of the
 * customer request it serves, where it shows in that ticket panel.
 * Trackers, which many conversations share, open their own view. A back-office ticket made without a customer request opens
 * as a thread of its notes beside its details.
 */
import { RiCoupon3Line } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Navigate } from "@tanstack/react-router";
import { ConversationSkeleton } from "@/features/inbox/skeletons";
import { useAccount } from "@/features/settings/use-account";
import { EmptyState, Panel } from "@/features/shell/page";
import { type Ticket, ticketQuery } from "@/lib/api/tickets";

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

  return <BackOffice view={view} ticket={ticket} />;
}

/*
 * servedRequest is the customer request a back-office ticket was made
 * for, when it has one.
 */
function servedRequest(ticket: Ticket): string | null {
  const link = (ticket.links ?? []).find((l) => l.kind === "back_office" && l.ticket && l.ticket.id !== ticket.id);

  return link?.ticket?.id ?? null;
}

/*
 * BackOffice opens the conversation of the request the work serves.
 */
function BackOffice({ view, ticket }: { view: string; ticket: Ticket }) {
  const workspace = useAccount().workspace.id;
  const parent = servedRequest(ticket);
  const { data: request, isPending } = useQuery({ ...ticketQuery(workspace, parent ?? ""), enabled: parent !== null });

  if (parent && isPending) {
    return (
      <Panel label="Ticket" className="flex-1">
        <ConversationSkeleton />
      </Panel>
    );
  }
  if (request?.conversation_id) {
    return (
      <Navigate to="/d/inbox/$view/$conversation" params={{ view, conversation: request.conversation_id }} replace />
    );
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
