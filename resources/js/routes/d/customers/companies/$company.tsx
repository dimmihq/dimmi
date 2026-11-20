import { createFileRoute } from "@tanstack/react-router";

import { CompanyPage } from "@/features/customers/company-page";

export const Route = createFileRoute("/d/customers/companies/$company")({
  component: CompanyRoute,
});

function CompanyRoute() {
  const { company } = Route.useParams();

  return <CompanyPage key={company} id={company} />;
}
