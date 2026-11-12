import { createFileRoute } from "@tanstack/react-router";

import { WebhookEndpointSettings } from "@/features/settings/pages/webhook-endpoint";

export const Route = createFileRoute("/d/settings/webhooks/$endpoint")({
  component: WebhookEndpointRoute,
});

function WebhookEndpointRoute() {
  const { endpoint } = Route.useParams();

  return <WebhookEndpointSettings key={endpoint} endpoint={endpoint} />;
}
