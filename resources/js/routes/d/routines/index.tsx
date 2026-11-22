import { createFileRoute } from "@tanstack/react-router";

import { RoutineList } from "@/features/routines/routine-list";

export const Route = createFileRoute("/d/routines/")({
  component: RoutineList,
});
