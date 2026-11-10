import { createFileRoute } from "@tanstack/react-router";

import { BroadcastPage } from "@/features/broadcasts/broadcast-page";

export const Route = createFileRoute("/d/broadcasts/$broadcast")({
  component: BroadcastRoute,
});

function BroadcastRoute() {
  const { broadcast } = Route.useParams();

  return <BroadcastPage key={broadcast} id={broadcast} />;
}
