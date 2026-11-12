import { createFileRoute } from "@tanstack/react-router";

import { ReportsPage } from "@/features/reports/reports-page";
import { validateReportSearch } from "@/features/reports/search";

export const Route = createFileRoute("/d/reports/$report")({
  validateSearch: validateReportSearch,
  component: ReportsPage,
});
