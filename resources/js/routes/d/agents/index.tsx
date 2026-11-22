import { createFileRoute } from "@tanstack/react-router";

import { AgentList } from "@/features/agents/agent-list";
import { useAccount } from "@/features/settings/use-account";

export const Route = createFileRoute("/d/agents/")({
  component: AgentsRoute,
});

function AgentsRoute() {
  const workspace = useAccount().workspace.id;

  return <AgentList workspace={workspace} />;
}
