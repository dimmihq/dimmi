import { createFileRoute } from "@tanstack/react-router";

import { WorkflowList } from "@/features/automation/workflow-list";

export const Route = createFileRoute("/d/workflows/")({
  component: WorkflowList,
});
