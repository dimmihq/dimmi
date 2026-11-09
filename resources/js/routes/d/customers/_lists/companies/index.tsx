import { createFileRoute } from "@tanstack/react-router";

import { CompaniesPage } from "@/features/customers/companies-page";
import { companySearch } from "@/features/customers/people-params";

export const Route = createFileRoute("/d/customers/_lists/companies/")({
  validateSearch: companySearch,
  component: CompaniesPage,
});
