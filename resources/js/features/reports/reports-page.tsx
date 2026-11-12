/*
 * The reports: a list of the reports the member may read, the filter
 * bar, the report's numbers, then its charts. Everything a view shows,
 * the conversations open behind a number included, lives in the
 * address, so a view can be shared and reloaded. A live report has no
 * range and refreshes itself every thirty seconds.
 */
import { RiBarChart2Line, RiErrorWarningLine } from "@remixicon/react";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { Link, useNavigate, useParams, useSearch } from "@tanstack/react-router";
import type { ReactNode } from "react";

import { Select, SelectItem } from "@/components/base/select/select";
import { Skeleton } from "@/components/base/skeleton/skeleton";
import { useInbox } from "@/features/inbox/conversations";
import { EmptyState, PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { type Report, type ReportChart, type ReportType, reportQuery, reportTypesQuery } from "@/lib/api/reports";
import { callerQuery } from "@/lib/api/workspace";
import { cx } from "@/utils/cx";

import { ChartCard, ChartsSkeleton } from "./chart-card";
import type { PointPick } from "./charts";
import { DrillDrawer } from "./drill-drawer";
import { FilterBar } from "./filter-bar";
import { formatRange } from "./format";
import { DigestButton, FactsExportButton } from "./header-actions";
import { MetricTiles, MetricTilesSkeleton } from "./metric-tiles";
import { type ReportSearch, reportFilters, withoutDrill, writeDrillBy } from "./search";

const route = "/d/reports/$report";

function ReportList({ types, current }: { types: ReportType[] | undefined; current: string }) {
  const search = withoutDrill(useSearch({ from: route }));

  return (
    <nav aria-label="Reports" className="hidden w-52 shrink-0 flex-col gap-0.5 ps-2 pb-4 lg:flex">
      {types
        ? types.map((type) => (
            <Link
              key={type.key}
              to={route}
              params={{ report: type.key }}
              search={search}
              title={type.summary}
              aria-current={type.key === current ? "page" : undefined}
              className={cx(
                "flex h-8 items-center gap-2 rounded-lg px-2 text-body-medium outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring",
                type.key === current
                  ? "bg-background-secondary-default text-text-primary"
                  : "text-text-secondary hover:bg-background-secondary-hover",
              )}
            >
              <span className="min-w-0 flex-1 truncate">{type.title}</span>
              {type.live ? (
                <span title="Live" className="size-1.5 shrink-0 rounded-full bg-chart-2">
                  <span className="sr-only">Live</span>
                </span>
              ) : null}
            </Link>
          ))
        : ["a", "b", "c", "d", "e"].map((id) => <Skeleton key={id} shape="text" className="mx-2 my-2 h-4 w-32" />)}
    </nav>
  );
}

function ReportPicker({ types, current }: { types: ReportType[]; current: string }) {
  const navigate = useNavigate({ from: route });

  return (
    <Select
      aria-label="Report"
      size="sm"
      className="lg:hidden"
      selectedKey={current}
      onSelectionChange={(key) =>
        void navigate({ to: route, params: { report: String(key) }, search: (prev) => withoutDrill(prev) })
      }
    >
      {types.map((type) => (
        <SelectItem key={type.key} id={type.key} textValue={type.title}>
          {type.title}
        </SelectItem>
      ))}
    </Select>
  );
}

/*
 * Caption says what the numbers cover: the range, the period compared
 * and whether only business hours count.
 */
function Caption({ report, summary }: { report: Report; summary: string | undefined }) {
  const parts = [
    report.live ? "Live, refreshed every 30 seconds" : formatRange(report.from, report.to),
    report.previous_from && report.previous_to
      ? `against ${formatRange(report.previous_from, report.previous_to)}`
      : "",
    report.business_hours ? "business hours" : "",
    summary ?? "",
  ];

  return <p className="text-body-2-regular text-text-tertiary">{parts.filter(Boolean).join(" · ")}</p>;
}

type Drill = Pick<ReportSearch, "drill" | "drill_from" | "drill_to" | "drill_by">;

/*
 * ReportBody draws a report: its numbers, then its charts. A point
 * clicked on a chart over days opens that day's conversations; a table
 * row opens its conversations, or narrows the report when the table has
 * none behind it.
 */
function ReportBody({
  report,
  dimmed,
  onDrill,
  onNarrow,
}: {
  report: Report;
  dimmed: boolean;
  onDrill: (drill: Drill) => void;
  onNarrow: (filter: Record<string, string>) => void;
}) {
  const pick = (chart: ReportChart, p: PointPick) =>
    onDrill(chart.axis === "day" ? { drill: p.drill, drill_from: p.x, drill_to: p.x } : { drill: p.drill });
  const row = (chart: ReportChart, r: ReportChart["rows"][number]) =>
    chart.drill ? onDrill({ drill: chart.drill, drill_by: writeDrillBy(r.filter) }) : onNarrow(r.filter);

  if (report.metrics.length === 0 && report.charts.length === 0) {
    return <EmptyState icon={RiBarChart2Line} title="Nothing to show yet" />;
  }

  return (
    <div className={cx("flex flex-col gap-4 transition-opacity", dimmed && "opacity-60")}>
      <MetricTiles report={report} onDrill={(d) => onDrill({ drill: d })} />
      {report.charts.length > 0 ? (
        <div className="grid grid-cols-1 gap-2 xl:grid-cols-2">
          {report.charts.map((chart) => (
            <ChartCard
              key={chart.key}
              chart={chart}
              report={report}
              onPick={(p) => pick(chart, p)}
              onRow={(r) => row(chart, r)}
            />
          ))}
        </div>
      ) : null}
    </div>
  );
}

function Loading() {
  return (
    <>
      <MetricTilesSkeleton />
      <ChartsSkeleton />
    </>
  );
}

function NoAccess() {
  return (
    <Panel className="h-full flex-1">
      <PageHeader title="Reports" />
      <EmptyState
        icon={RiBarChart2Line}
        title="No access to reports"
        description="Reports need permission to read conversations. Ask an admin."
      />
    </Panel>
  );
}

/*
 * useReport reads the report the address names, for its range and
 * filters. A live report refreshes every thirty seconds.
 */
function useReport(workspace: string, key: string, search: ReportSearch) {
  const enabled = workspace !== "";
  const caller = useQuery({ ...callerQuery(workspace), enabled });
  const allowed = caller.data?.member?.permissions.includes("conversations:read") ?? false;
  const types = useQuery({ ...reportTypesQuery(workspace), enabled: enabled && allowed });
  const type = types.data?.find((t) => t.key === key);
  const live = type?.live ?? false;
  const filters = reportFilters(search, live);
  const report = useQuery({
    ...reportQuery(workspace, key, filters),
    enabled: enabled && allowed && types.isSuccess && type !== undefined,
    placeholderData: keepPreviousData,
    refetchInterval: live ? 30_000 : false,
  });

  return {
    denied: caller.isSuccess && !allowed,
    types: types.data,
    type,
    unknown: types.isSuccess && !type,
    live,
    filters,
    data: report.data?.key === key ? report.data : undefined,
    dimmed: report.isPlaceholderData,
    error: types.error ?? (report.isError && !report.isFetching ? report.error : null),
  };
}

export function ReportsPage() {
  const { report: key } = useParams({ from: route });
  const search = useSearch({ from: route });
  const navigate = useNavigate({ from: route });
  const { workspace } = useInbox();
  const view = useReport(workspace, key, search);

  const change = (changes: Partial<ReportSearch>) =>
    void navigate({ search: (prev) => withoutDrill({ ...prev, ...changes }), replace: true });
  const drill = (next: Drill) => void navigate({ search: (prev) => ({ ...withoutDrill(prev), ...next }) });

  if (view.denied) {
    return <NoAccess />;
  }

  let body: ReactNode;
  if (view.error) {
    body = <EmptyState icon={RiErrorWarningLine} title="This report could not load" description={view.error.message} />;
  } else if (view.data) {
    body = <ReportBody report={view.data} dimmed={view.dimmed} onDrill={drill} onNarrow={change} />;
  } else {
    body = <Loading />;
  }

  return (
    <Panel ground label="Reports" className="h-full w-full flex-1">
      <PageHeader title={view.type?.title ?? "Reports"}>
        {view.type ? <FactsExportButton workspace={workspace} filters={view.filters} /> : null}
        <DigestButton workspace={workspace} />
      </PageHeader>
      <div className="flex min-h-0 flex-1">
        <ReportList types={view.types} current={key} />
        <ScrollArea ground className="min-w-0">
          <div className="flex min-w-0 flex-col gap-4 px-4 pt-1 pb-8">
            {view.types ? <ReportPicker types={view.types} current={key} /> : null}
            {view.unknown ? (
              <EmptyState icon={RiBarChart2Line} title="Report not found" description="Pick a report from the list." />
            ) : (
              <>
                <FilterBar workspace={workspace} search={search} live={view.live} onChange={change} />
                {view.data ? <Caption report={view.data} summary={view.type?.summary} /> : null}
                {body}
              </>
            )}
          </div>
        </ScrollArea>
      </div>
      <DrillDrawer
        workspace={workspace}
        reportKey={key}
        report={view.data}
        search={search}
        onClose={() => void navigate({ search: (prev) => withoutDrill(prev), replace: true })}
      />
    </Panel>
  );
}
