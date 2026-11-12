import { createFileRoute, notFound } from "@tanstack/react-router";

import { InboxPage } from "@/features/inbox/inbox-page";
import { isAiFilter } from "@/features/inbox/views/ai-filters";
import { type StatusChoice, viewParams } from "@/features/inbox/views/view-route";
import type { AiFilter } from "@/lib/api/inbox";

const statuses: StatusChoice[] = ["open", "snoozed", "closed", "any"];

export const Route = createFileRoute("/d/inbox/$view")({
  validateSearch: (
    search: Record<string, unknown>,
  ): { status?: StatusChoice; editor?: "new"; ai?: AiFilter; where?: string } => {
    const status = statuses.find((s) => s === search.status);
    return {
      ...(status ? { status } : {}),
      ...(typeof search.where === "string" && search.where !== "" ? { where: search.where } : {}),
      ...(search.editor === "new" ? { editor: "new" as const } : {}),
      ...(isAiFilter(search.ai) ? { ai: search.ai } : {}),
    };
  },
  beforeLoad: ({ params }) => {
    if (!viewParams(params.view)) {
      throw notFound();
    }
  },
  component: InboxRoute,
});

function InboxRoute() {
  const { view } = Route.useParams();

  return <InboxPage view={view} />;
}
