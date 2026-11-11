/*
 * What a step did since the series went live: how many reached it and
 * wait on it now, how many left by each path, what its message did, and
 * for a split each variant's goal with a button that sends everyone
 * down the winner. Below, the people standing on the step right now,
 * each of whom a teammate can take off the series. On the entry step,
 * the series' own totals and why people left.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/base/buttons/button";
import { toast } from "@/components/base/toast/toast";
import { Row, Section } from "@/features/settings/layout";
import {
  pickSeriesWinner,
  removeFromSeries,
  type SeriesNode,
  type SeriesReport,
  seriesJourneysQuery,
  seriesKey,
} from "@/lib/api/series";
import { formatNumber, formatRelative } from "@/lib/i18n/format";

import { endReasonName, percent } from "./looks";
import { isMessage, pathsOf } from "./model";

function Value({ children }: { children: string }) {
  return <span className="text-body-2-medium text-text-primary tabular-nums">{children}</span>;
}

function Totals({ report }: { report: SeriesReport }) {
  return (
    <>
      <Section title="Results" description="Since it went live. Test journeys never count.">
        <Row label="Entered">
          <Value>{formatNumber(report.entered)}</Value>
        </Row>
        <Row label="On it now">
          <Value>{formatNumber(report.active)}</Value>
        </Row>
        <Row label="Finished">
          <Value>{formatNumber(report.finished)}</Value>
        </Row>
        <Row label="Reached the goal">
          <Value>{`${formatNumber(report.goal)} · ${percent(report.goal, report.entered)}`}</Value>
        </Row>
      </Section>
      {report.reasons.length > 0 ? (
        <Section title="Why people left">
          {report.reasons.map((r) => (
            <Row key={r.reason} label={endReasonName[r.reason] ?? r.reason}>
              <Value>{formatNumber(r.count)}</Value>
            </Row>
          ))}
        </Section>
      ) : null}
    </>
  );
}

function Variants({
  workspace,
  series,
  node,
  report,
}: {
  workspace: string;
  series: string;
  node: SeriesNode;
  report: SeriesReport;
}) {
  const client = useQueryClient();
  const variants = report.steps[node.id]?.variants ?? [];
  const pick = useMutation({
    mutationFn: (path: string) => pickSeriesWinner(workspace, series, node.id, path),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: seriesKey(workspace) });
      toast.success("Everyone who reaches the split now takes the winner.");
    },
    onError: (error) => void toast.error(error instanceof Error ? error.message : "The winner was not saved."),
  });
  if (variants.length === 0) {
    return null;
  }
  const names = new Map((node.settings?.variants ?? []).map((v) => [v.id, v.label]));

  return (
    <Section title="Variants" description="Goal reached by people given each variant.">
      {variants.map((v) => (
        <Row
          key={v.path}
          label={names.get(v.path) ?? v.path}
          description={`${formatNumber(v.entered)} people · ${percent(v.goal, v.entered)} reached the goal`}
        >
          <Button size="xs" variant="secondary" loading={pick.isPending} onClick={() => pick.mutate(v.path)}>
            Pick as winner
          </Button>
        </Row>
      ))}
    </Section>
  );
}

function PeopleHere({ workspace, series, node }: { workspace: string; series: string; node: SeriesNode }) {
  const client = useQueryClient();
  const journeys = useQuery(seriesJourneysQuery(workspace, series, node.id)).data ?? [];
  const remove = useMutation({
    mutationFn: (journey: string) => removeFromSeries(workspace, series, journey),
    onSuccess: () => void client.invalidateQueries({ queryKey: seriesKey(workspace) }),
  });
  if (journeys.length === 0) {
    return null;
  }

  return (
    <Section title="Here now">
      {journeys.map((j) => (
        <Row
          key={j.id}
          label={j.contact_name || j.company_name || j.contact_email || "Someone"}
          description={j.wake_at ? `Moves on ${formatRelative(j.wake_at)}` : `Entered ${formatRelative(j.entered_at)}`}
        >
          <Button size="xs" variant="secondary" loading={remove.isPending} onClick={() => remove.mutate(j.id)}>
            Take off
          </Button>
        </Row>
      ))}
    </Section>
  );
}

export function StepResults({
  workspace,
  series,
  node,
  report,
}: {
  workspace: string;
  series: string;
  node: SeriesNode;
  report?: SeriesReport;
}) {
  if (!report) {
    return null;
  }
  if (node.kind === "entry") {
    return <Totals report={report} />;
  }
  const step = report.steps[node.id];
  const labels = new Map(pathsOf(node).map((p) => [p.id, p.label || "Went on"]));
  labels.set("failed", "Could not send");
  const sent = step?.message?.sent ?? 0;

  return (
    <>
      <Section title="Results" description="Since it went live.">
        <Row label="Reached this step">
          <Value>{formatNumber(step?.reached ?? 0)}</Value>
        </Row>
        <Row label="Here now">
          <Value>{formatNumber(step?.waiting ?? 0)}</Value>
        </Row>
        {isMessage(node.kind) && step?.message ? (
          <>
            <Row label="Sent">
              <Value>{formatNumber(sent)}</Value>
            </Row>
            <Row label="Opened" description={node.kind === "email" ? "Email opens are estimated." : undefined}>
              <Value>{percent(step.message.opened ?? 0, sent)}</Value>
            </Row>
            <Row label="Clicked">
              <Value>{percent(step.message.clicked ?? 0, sent)}</Value>
            </Row>
            <Row label="Replied">
              <Value>{formatNumber(step.message.replied ?? 0)}</Value>
            </Row>
          </>
        ) : null}
        {Object.entries(step?.paths ?? {})
          .filter(([path]) => path !== "next")
          .map(([path, n]) => (
            <Row key={path} label={labels.get(path) ?? path}>
              <Value>{formatNumber(n)}</Value>
            </Row>
          ))}
      </Section>
      {node.kind === "split" ? <Variants workspace={workspace} series={series} node={node} report={report} /> : null}
      <PeopleHere workspace={workspace} series={series} node={node} />
    </>
  );
}
