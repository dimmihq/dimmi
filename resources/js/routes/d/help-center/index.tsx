import { createFileRoute } from "@tanstack/react-router";

import { HelpCenterPage } from "@/features/help-center/help-center-page";

interface DocsSearch {
  collection?: string;
}

export const Route = createFileRoute("/d/help-center/")({
  validateSearch: (search: Record<string, unknown>): DocsSearch =>
    typeof search.collection === "string" && search.collection !== "" ? { collection: search.collection } : {},
  component: () => <HelpCenterPage />,
});
