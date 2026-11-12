import { createFileRoute } from "@tanstack/react-router";

import { SLAPolicyPage } from "@/features/tickets/sla/policy-editor";

export const Route = createFileRoute("/d/settings/sla/$policy")({
  component: SLAPolicyRoute,
});

function SLAPolicyRoute() {
  const { policy } = Route.useParams();

  return <SLAPolicyPage key={policy} policy={policy} />;
}
