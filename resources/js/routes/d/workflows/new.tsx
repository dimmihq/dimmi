import { createFileRoute, redirect } from "@tanstack/react-router";

import { isTrigger, type TriggerKind } from "@/features/automation/triggers";
import { WorkflowBuilder } from "@/features/automation/workflow-builder";
import { blankWorkflow } from "@/features/automation/workflows";

export const Route = createFileRoute("/d/workflows/new")({
  validateSearch: (search: Record<string, unknown>): { trigger?: TriggerKind } => ({
    trigger: isTrigger(search.trigger) ? search.trigger : undefined,
  }),
  beforeLoad: ({ search }) => {
    if (!search.trigger) {
      throw redirect({ to: "/d/workflows" });
    }
  },
  component: NewWorkflowRoute,
});

function NewWorkflowRoute() {
  const { trigger } = Route.useSearch();
  if (!trigger) {
    return null;
  }

  return <WorkflowBuilder key={trigger} workflow={blankWorkflow(trigger)} />;
}
