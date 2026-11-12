import { createFileRoute } from "@tanstack/react-router";

import { TicketTypePage } from "@/features/tickets/settings/type-editor";

export const Route = createFileRoute("/d/settings/tickets/$type")({
  component: TicketTypeRoute,
});

function TicketTypeRoute() {
  const { type } = Route.useParams();

  return <TicketTypePage key={type} type={type} />;
}
