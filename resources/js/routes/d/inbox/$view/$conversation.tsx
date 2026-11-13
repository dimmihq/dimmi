import { createFileRoute } from "@tanstack/react-router";

import { OpenConversation } from "@/features/inbox/open-conversation";
import { isTicketId } from "@/features/tickets/ticket-ids";
import { TicketOpen } from "@/features/tickets/ticket-open";

export const Route = createFileRoute("/d/inbox/$view/$conversation")({
  component: ConversationRoute,
});

function ConversationRoute() {
  const { view, conversation: id } = Route.useParams();

  if (isTicketId(id)) {
    return <TicketOpen key={id} view={view} id={id} />;
  }

  return <OpenConversation view={view} id={id} />;
}
