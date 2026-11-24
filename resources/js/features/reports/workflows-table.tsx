/*
 * The workflows that ran in the range, side by side with the agents and
 * procedures: runs started, how many completed, went to a person or
 * failed, and how many reached the workflow's goal. A row opens the
 * workflow's own analytics.
 */
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";

import { type TriggerKind, triggers } from "@/features/automation/triggers";
import { Row, Section } from "@/features/settings/layout";
import { workflowsReportQuery } from "@/lib/api/workflows";

function share(part: number, whole: number): string {
  return whole === 0 ? "–" : `${Math.round((part / whole) * 100)}%`;
}

export function WorkflowsTable({ workspace, from, to }: { workspace: string; from?: string; to?: string }) {
  const rows = useQuery({ ...workflowsReportQuery(workspace, from ?? "", to ?? ""), retry: false });
  const list = rows.data ?? [];

  return (
    <Section
      title="Workflows"
      description="Each workflow that ran in the range, busiest first. Test conversations never count."
      loading={rows.isPending}
    >
      {list.length === 0 ? (
        <p className="px-3 py-2.5 text-body-2-regular text-text-secondary">No workflow ran in the range.</p>
      ) : null}
      {list.map((r) => (
        <Row
          key={r.workflow_id}
          label={
            <Link to="/d/workflows/$workflow" params={{ workflow: r.workflow_id }} className="underline">
              {r.name}
            </Link>
          }
          description={`${triggers[r.trigger as TriggerKind]?.name ?? r.trigger} · ${share(r.handed_over, r.triggered)} handed over · ${r.failed} failed${
            r.goal_met === null ? "" : ` · ${share(r.goal_met, r.triggered)} reached the goal`
          }`}
        >
          <span className="text-body-medium tabular-nums">
            {r.triggered} runs · {share(r.completed, r.triggered)} completed
          </span>
        </Row>
      ))}
    </Section>
  );
}
