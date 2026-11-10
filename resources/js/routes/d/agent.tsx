import { createFileRoute } from "@tanstack/react-router";

import { AgentOverview, type AgentTab } from "@/features/ai/overview/agent-overview";

function AgentPage() {
  const { tab } = Route.useSearch();
  const navigate = Route.useNavigate();

  return (
    <AgentOverview
      tab={tab ?? "dimmi"}
      onTab={(next) => void navigate({ search: next === "dimmi" ? {} : { tab: next }, replace: true })}
    />
  );
}

export const Route = createFileRoute("/d/agent")({
  validateSearch: (search: Record<string, unknown>): { tab?: AgentTab } =>
    search.tab === "operator" ? { tab: "operator" } : {},
  component: AgentPage,
});
