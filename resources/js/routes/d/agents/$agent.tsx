import { RiRobot2Line } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";

import { Spinner } from "@/components/base/spinner/spinner";
import { StudioArrive } from "@/features/agents/agent-arrive";
import { AgentStudio } from "@/features/agents/agent-studio";
import { useAccount } from "@/features/settings/use-account";
import { EmptyState, Panel } from "@/features/shell/page";
import { agentQuery } from "@/lib/api/agents";

export const Route = createFileRoute("/d/agents/$agent")({
  component: AgentRoute,
});

/*
 * AgentRoute opens Agent Studio once the agent is freshly read, so the
 * first save names the lock version the server holds now, and says so
 * when the workspace has no such agent.
 */
function AgentRoute() {
  const { agent: id } = Route.useParams();
  const workspace = useAccount().workspace.id;
  const { data, isError, isFetchedAfterMount } = useQuery({
    ...agentQuery(workspace, id),
    refetchOnWindowFocus: false,
  });

  if (isError) {
    return (
      <Panel label="Agent" ground className="h-full flex-1">
        <EmptyState icon={RiRobot2Line} title="No such agent" description="It may have been deleted.">
          <Link to="/d/agents" className="text-body-medium text-text-primary underline">
            Back to agents
          </Link>
        </EmptyState>
      </Panel>
    );
  }

  if (!data || !isFetchedAfterMount) {
    return (
      <Panel label="Agent" ground className="h-full flex-1 items-center justify-center">
        <Spinner />
      </Panel>
    );
  }

  return (
    <StudioArrive key={data.agent.id}>
      <AgentStudio workspace={workspace} view={data} />
    </StudioArrive>
  );
}
