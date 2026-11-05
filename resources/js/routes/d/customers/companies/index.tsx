import { createFileRoute } from "@tanstack/react-router";

import { CompaniesPage } from "@/features/customers/companies-page";

export const Route = createFileRoute("/d/customers/companies/")({
  component: CompaniesPage,
});
