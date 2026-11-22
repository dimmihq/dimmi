import { createFileRoute } from "@tanstack/react-router";

import { RoutinePage } from "@/features/routines/routine-page";

function RoutineRoute() {
  const { routine } = Route.useParams();
  return <RoutinePage key={routine} id={routine} />;
}

export const Route = createFileRoute("/d/routines/$routine/")({
  component: RoutineRoute,
});
