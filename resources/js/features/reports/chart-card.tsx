/*
 * One chart of a report on its card: its title, a legend when it has
 * more than one series, a CSV of exactly its data, and the chart its
 * kind calls for. A kind this page does not know yet draws as a table
 * when it has rows, else as bars.
 */
import { RiDownload2Line } from "@remixicon/react";

import { Button } from "@/components/base/buttons/button";
import { Skeleton } from "@/components/base/skeleton/skeleton";
import type { Report, ReportChart } from "@/lib/api/reports";
import { cx } from "@/utils/cx";

import { CartesianChart, Heatmap, type PointPick, ReportTable } from "./charts";
import { chartCsv, saveFile } from "./csv";
import { seriesColor } from "./format";

type Drawn = "cartesian" | "heatmap" | "table";

function drawnAs(chart: ReportChart): Drawn {
  switch (chart.kind) {
    case "line":
    case "bar":
    case "stacked_bar":
    case "histogram":
      return "cartesian";
    case "heatmap":
      return "heatmap";
    case "table":
      return "table";
    default:
      return chart.rows.length > 0 || chart.series.length === 0 ? "table" : "cartesian";
  }
}

function isEmpty(chart: ReportChart, drawn: Drawn): boolean {
  if (drawn === "table") {
    return chart.rows.length === 0;
  }

  return chart.series.every((s) => s.points.every((p) => p.y === null || p.y === 0));
}

/*
 * wide says whether the chart takes the grid's whole width.
 */
function isWide(chart: ReportChart): boolean {
  const drawn = drawnAs(chart);

  return drawn === "heatmap" || (drawn === "table" && chart.columns.length > 3);
}

export function ChartCard({
  chart,
  report,
  onPick,
  onRow,
}: {
  chart: ReportChart;
  report: Report;
  onPick: (pick: PointPick) => void;
  onRow: (row: ReportChart["rows"][number]) => void;
}) {
  const drawn = drawnAs(chart);
  const empty = isEmpty(chart, drawn);
  const legend = drawn === "cartesian" && chart.series.length > 1;

  return (
    <section
      aria-label={chart.title}
      className={cx(
        "flex min-w-0 flex-col gap-3 rounded-2xl bg-background-secondary-default p-4",
        isWide(chart) && "xl:col-span-2",
      )}
    >
      <header className="flex min-w-0 items-start justify-between gap-2">
        <div className="flex min-w-0 flex-col gap-1.5">
          <h2 className="truncate text-body-medium text-text-primary">{chart.title}</h2>
          {legend ? (
            <ul className="flex flex-wrap items-center gap-x-3 gap-y-1">
              {chart.series.map((series, i) => (
                <li key={series.key} className="flex items-center gap-1.5 text-caption-1-regular text-text-secondary">
                  <span aria-hidden className="size-2 rounded-full" style={{ background: seriesColor(i) }} />
                  {series.label}
                </li>
              ))}
            </ul>
          ) : null}
        </div>
        <Button
          size="xs"
          variant="secondary"
          leadingIcon={RiDownload2Line}
          aria-label={`Download ${chart.title} as CSV`}
          disabled={empty}
          onClick={() => saveFile(chartCsv(chart), `${report.key}-${chart.key}-${report.from}-${report.to}.csv`)}
        >
          CSV
        </Button>
      </header>
      {empty ? (
        <p className="flex h-40 items-center justify-center text-body-2-regular text-text-tertiary">
          Nothing in this range.
        </p>
      ) : drawn === "heatmap" ? (
        <Heatmap chart={chart} />
      ) : drawn === "table" ? (
        <ReportTable chart={chart} onRow={onRow} />
      ) : (
        <CartesianChart chart={chart} onPick={onPick} />
      )}
    </section>
  );
}

export function ChartsSkeleton() {
  return (
    <div className="grid grid-cols-1 gap-2 xl:grid-cols-2">
      {["a", "b"].map((id) => (
        <Skeleton key={id} className="h-[300px] rounded-2xl" />
      ))}
    </div>
  );
}
