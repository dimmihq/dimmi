import { createFileRoute } from "@tanstack/react-router";

import { ProcedureList } from "@/features/automation/procedure-list";

export const Route = createFileRoute("/d/procedures/")({
  component: ProcedureList,
});
