import { createFileRoute } from "@tanstack/react-router";

import { RoutineNew } from "@/features/routines/routine-new";

function NewRoutinePage() {
  const { template } = Route.useSearch();
  return <RoutineNew key={template} template={template ?? "tpl_custom"} />;
}

export const Route = createFileRoute("/d/routines/new")({
  validateSearch: (search: Record<string, unknown>): { template?: string } =>
    typeof search.template === "string" ? { template: search.template } : {},
  component: NewRoutinePage,
});
