import { createFileRoute, notFound } from "@tanstack/react-router";

import { WorkflowBuilder } from "@/features/automation/workflow-builder";
import { findWorkflow } from "@/features/automation/workflows";

export const Route = createFileRoute("/d/workflows/$workflow")({
  loader: ({ params }) => {
    const workflow = findWorkflow(params.workflow);
    if (!workflow) {
      throw notFound();
    }

    return workflow;
  },
  component: WorkflowRoute,
});

function WorkflowRoute() {
  const workflow = Route.useLoaderData();

  return <WorkflowBuilder key={workflow.id} workflow={workflow} />;
}
