import { createFileRoute } from "@tanstack/react-router";

import { AgentList } from "@/features/agents/agent-list";

export const Route = createFileRoute("/d/agents/")({
  component: AgentList,
});
