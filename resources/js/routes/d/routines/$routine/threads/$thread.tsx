import { createFileRoute } from "@tanstack/react-router";

import { RoutineThread } from "@/features/routines/routine-thread";

function RoutineThreadRoute() {
  const { routine, thread } = Route.useParams();
  return <RoutineThread key={thread} routine={routine} thread={thread} />;
}

export const Route = createFileRoute("/d/routines/$routine/threads/$thread")({
  component: RoutineThreadRoute,
});
