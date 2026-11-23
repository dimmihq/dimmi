import { createFileRoute } from "@tanstack/react-router";

import { RunPage } from "@/features/routines/run-page";

function RoutineRunRoute() {
  const { routine, run } = Route.useParams();
  return <RunPage key={run} routine={routine} run={run} />;
}

export const Route = createFileRoute("/d/routines/$routine/runs/$run")({
  component: RoutineRunRoute,
});
