import { createFileRoute } from "@tanstack/react-router";

import { MessengerPage } from "@/features/settings/pages/messengers";

export const Route = createFileRoute("/d/settings/messenger/$widget")({
  component: MessengerRoute,
});

function MessengerRoute() {
  const { widget } = Route.useParams();

  return <MessengerPage key={widget} widget={widget} />;
}
