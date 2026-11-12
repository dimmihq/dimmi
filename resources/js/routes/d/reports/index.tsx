import { createFileRoute, redirect } from "@tanstack/react-router";

import { validateReportSearch } from "@/features/reports/search";

/*
 * /d/reports opens the overview, keeping any range and filters in the
 * address.
 */
export const Route = createFileRoute("/d/reports/")({
  validateSearch: validateReportSearch,
  beforeLoad: ({ search }) => {
    throw redirect({ to: "/d/reports/$report", params: { report: "overview" }, search, replace: true });
  },
});
