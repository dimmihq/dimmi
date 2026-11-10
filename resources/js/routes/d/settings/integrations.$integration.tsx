import { createFileRoute } from "@tanstack/react-router";

import { IntegrationSettingsPage } from "@/features/settings/pages/integration-detail";

export const Route = createFileRoute("/d/settings/integrations/$integration")({
  component: IntegrationRoute,
});

function IntegrationRoute() {
  const { integration } = Route.useParams();

  return <IntegrationSettingsPage key={integration} integration={integration} />;
}
