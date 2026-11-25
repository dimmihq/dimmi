/*
 * An agent's Analytics: the AI report narrowed to this agent, for the
 * last thirty days. The same numbers and charts as Reports, without the
 * table that compares agents; that table, and every conversation behind
 * a number, open in Reports.
 */
import { RiArrowRightUpLine, RiBarChart2Line } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";

import { LinkButton } from "@/components/base/buttons/link-button";
import { ChartCard, ChartsSkeleton } from "@/features/reports/chart-card";
import { MetricTiles, MetricTilesSkeleton } from "@/features/reports/metric-tiles";
import { EmptyState } from "@/features/shell/page";
import { reportQuery } from "@/lib/api/reports";

/* Charts the agent's own view leaves to Reports. */
const fleetOnly = new Set(["ai_agents"]);

export function AgentAnalytics({ workspace, agent }: { workspace: string; agent: string }) {
  const navigate = useNavigate();
  const { data: report, isPending } = useQuery(reportQuery(workspace, "ai", { agent }));
  const open = (drill?: string) =>
    void navigate({ to: "/d/reports/$report", params: { report: "ai" }, search: { agent, drill } });

  return (
    <div className="mx-auto flex w-full max-w-[960px] flex-col gap-4 px-6 pt-6 pb-16">
      <div className="flex items-center justify-between gap-3">
        <div className="flex flex-col">
          <h2 className="text-body-medium text-text-primary">Last 30 days</h2>
          <p className="text-body-2-regular text-text-secondary">
            Conversations this agent held last, so a transfer counts for the agent that finished it.
          </p>
        </div>
        <LinkButton size="small" trailingIcon={RiArrowRightUpLine} onClick={() => open()}>
          Compare agents
        </LinkButton>
      </div>
      {isPending ? (
        <>
          <MetricTilesSkeleton />
          <ChartsSkeleton />
        </>
      ) : !report || report.metrics.every((m) => !m.sample) ? (
        <EmptyState
          icon={RiBarChart2Line}
          title="No conversations yet"
          description="Numbers show up once this agent answers customers."
        />
      ) : (
        <>
          <MetricTiles report={report} onDrill={open} />
          <div className="grid grid-cols-1 gap-2 xl:grid-cols-2">
            {report.charts
              .filter((chart) => !fleetOnly.has(chart.key))
              .map((chart) => (
                <ChartCard
                  key={chart.key}
                  chart={chart}
                  report={report}
                  onPick={(p) => open(p.drill)}
                  onRow={() => open(chart.drill ?? undefined)}
                />
              ))}
          </div>
        </>
      )}
    </div>
  );
}
