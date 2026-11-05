import { createFileRoute } from "@tanstack/react-router";

import { GatePage } from "@/features/gate/gate-page";

interface GateSearch {
  next: string;
  failed?: true;
}

export const Route = createFileRoute("/__gate")({
  validateSearch: (search: Record<string, unknown>): GateSearch => ({
    next: typeof search.next === "string" && search.next.startsWith("/") ? search.next : "/",
    failed: search.failed ? true : undefined,
  }),
  component: GateRoute,
});

function GateRoute() {
  const { next, failed } = Route.useSearch();

  return <GatePage next={next} failed={failed === true} />;
}
