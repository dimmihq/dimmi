import { createFileRoute } from "@tanstack/react-router";

import { FeedbackPage } from "@/features/feedback/feedback-page";

interface FeedbackSearch {
  view?: string;
  sort?: string;
  page?: number;
}

export const Route = createFileRoute("/d/feedback/")({
  validateSearch: (search: Record<string, unknown>): FeedbackSearch => ({
    ...(typeof search.view === "string" && search.view !== "" ? { view: search.view } : {}),
    ...(typeof search.sort === "string" && search.sort !== "" ? { sort: search.sort } : {}),
    ...(typeof search.page === "number" && search.page > 1 ? { page: search.page } : {}),
  }),
  component: () => <FeedbackPage />,
});
