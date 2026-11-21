/*
 * The procedures dimmi followed in the range, side by side: the
 * conversations each ran in, how many dimmi resolved (counted as this
 * report counts them), how many went to the team, how customers rated
 * them, and how often each one's guardrails stepped in. A row opens the
 * procedure's own scorecard.
 */
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";

import { Row, Section } from "@/features/settings/layout";
import { proceduresReportQuery } from "@/lib/api/procedures";

function percent(rate: number | null): string {
  return rate === null ? "–" : `${Math.round(rate * 100)}%`;
}

export function ProceduresTable({ workspace, from, to }: { workspace: string; from?: string; to?: string }) {
  const rows = useQuery({ ...proceduresReportQuery(workspace, from ?? "", to ?? ""), retry: false });
  const list = rows.data ?? [];

  return (
    <Section
      title="Procedures"
      description="Each procedure dimmi followed in the range. Resolutions count as above."
      loading={rows.isPending}
    >
      {list.length === 0 ? (
        <p className="px-3 py-2.5 text-body-2-regular text-text-secondary">dimmi followed no procedure in the range.</p>
      ) : null}
      {list.map((r) => (
        <Row
          key={r.procedure_id}
          label={
            <Link to="/d/procedures/$procedure" params={{ procedure: r.procedure_id }} className="underline">
              {r.name}
            </Link>
          }
          description={`${r.matched_conversations} conversations · ${percent(r.handover_rate)} handed over · ${
            r.mean_rating === null ? "no ratings" : `rated ${r.mean_rating.toFixed(1)}`
          } · ${r.guard_events} guardrail events`}
        >
          <span className="text-body-medium tabular-nums">{percent(r.ai_resolution_rate)} resolved</span>
        </Row>
      ))}
    </Section>
  );
}
