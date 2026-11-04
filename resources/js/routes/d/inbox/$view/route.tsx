import { createFileRoute, notFound } from "@tanstack/react-router";

import { InboxPage } from "@/features/inbox/inbox-page";
import { type StatusChoice, viewParams } from "@/features/inbox/views/view-route";

const statuses: StatusChoice[] = ["open", "snoozed", "closed", "any"];

export const Route = createFileRoute("/d/inbox/$view")({
  validateSearch: (search: Record<string, unknown>): { status?: StatusChoice; editor?: "new" } => {
    const status = statuses.find((s) => s === search.status);
    return { ...(status ? { status } : {}), ...(search.editor === "new" ? { editor: "new" as const } : {}) };
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
