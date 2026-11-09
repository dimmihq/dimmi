import { createFileRoute } from "@tanstack/react-router";

import { ChangelogPage } from "@/features/changelog/changelog-page";

interface ChangelogSearch {
  view?: string;
  page?: number;
}

export const Route = createFileRoute("/d/changelog/")({
  validateSearch: (search: Record<string, unknown>): ChangelogSearch => ({
    ...(typeof search.view === "string" && search.view !== "" ? { view: search.view } : {}),
    ...(typeof search.page === "number" && search.page > 1 ? { page: search.page } : {}),
  }),
  component: () => <ChangelogPage />,
});
