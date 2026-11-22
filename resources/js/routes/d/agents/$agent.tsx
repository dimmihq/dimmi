import { createFileRoute } from "@tanstack/react-router";

import { AgentStudio } from "@/features/agents/agent-studio";
import { agentById } from "@/features/agents/mock";
import { useAccount } from "@/features/settings/use-account";

export const Route = createFileRoute("/d/agents/$agent")({
  component: AgentRoute,
});

function AgentRoute() {
  const { agent: id } = Route.useParams();
  const workspace = useAccount().workspace.id;
  const agent = agentById(id);

  return <AgentStudio key={agent.id} workspace={workspace} agent={agent} />;
}
