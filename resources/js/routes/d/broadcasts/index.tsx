import { createFileRoute } from "@tanstack/react-router";

import { BroadcastList } from "@/features/broadcasts/broadcast-list";

export const Route = createFileRoute("/d/broadcasts/")({
  validateSearch: (search: Record<string, unknown>): { view?: string } =>
    typeof search.view === "string" ? { view: search.view } : {},
  component: BroadcastsRoute,
});

function BroadcastsRoute() {
  const { view } = Route.useSearch();

  return <BroadcastList view={view} />;
}
