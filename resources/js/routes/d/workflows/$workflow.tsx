import { RiFlowChart } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";

import { Spinner } from "@/components/base/spinner/spinner";
import { WorkflowBuilder } from "@/features/automation/workflow-builder";
import { useAccount } from "@/features/settings/use-account";
import { EmptyState, Panel } from "@/features/shell/page";
import { workflowQuery } from "@/lib/api/workflows";

export const Route = createFileRoute("/d/workflows/$workflow")({
  component: WorkflowRoute,
});

/*
 * WorkflowRoute opens the builder once the workflow is freshly read, so
 * the builder's first save names the lock version the server holds now,
 * and says so when the workspace has no such workflow.
 */
function WorkflowRoute() {
  const { workflow: id } = Route.useParams();
  const workspace = useAccount().workspace.id;
  const { data, isError, isFetchedAfterMount } = useQuery({
    ...workflowQuery(workspace, id),
    refetchOnWindowFocus: false,
  });

  if (isError) {
    return (
      <Panel label="Workflow" ground className="h-full flex-1">
        <EmptyState icon={RiFlowChart} title="No such workflow" description="It may have been archived.">
          <Link to="/d/workflows" className="text-body-medium text-text-primary underline">
            Back to workflows
          </Link>
        </EmptyState>
      </Panel>
    );
  }

  if (!data?.spec || !isFetchedAfterMount) {
    return (
      <Panel label="Workflow" ground className="h-full flex-1 items-center justify-center">
        <Spinner />
      </Panel>
    );
  }

  return <WorkflowBuilder key={data.id} workspace={workspace} workflow={data} />;
}
