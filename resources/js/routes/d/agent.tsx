import { createFileRoute } from "@tanstack/react-router";

import { AgentOverview } from "@/features/ai/overview/agent-overview";

export const Route = createFileRoute("/d/agent")({
  component: AgentOverview,
});
