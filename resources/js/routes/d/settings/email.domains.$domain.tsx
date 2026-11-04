import { createFileRoute } from "@tanstack/react-router";

import { DomainPage } from "@/features/email/domains/domain-page";

export const Route = createFileRoute("/d/settings/email/domains/$domain")({
  component: DomainRoute,
});

function DomainRoute() {
  const { domain } = Route.useParams();

  return <DomainPage key={domain} domain={domain} />;
}
