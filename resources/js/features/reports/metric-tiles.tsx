/*
 * The row of numbers at the top of a report. A duration shows its
 * median, with the 90th percentile and the mean beside it. With compare
 * on, each number carries its change against the period before, green
 * when the change is good. A number with conversations behind it opens
 * them.
 */
import { Chip } from "@/components/base/badges/chip";
import { Skeleton } from "@/components/base/skeleton/skeleton";
import type { Report, ReportMetric } from "@/lib/api/reports";
import { cx } from "@/utils/cx";

import { deltaOf, formatRange, formatValue } from "./format";

const tile = "flex min-w-0 flex-col gap-1 rounded-2xl bg-background-secondary-default p-4 text-start";

function Tile({ metric, report, onDrill }: { metric: ReportMetric; report: Report; onDrill: (drill: string) => void }) {
  const delta = report.previous_from ? deltaOf(metric) : null;
  const since =
    report.previous_from && report.previous_to
      ? `Against ${formatRange(report.previous_from, report.previous_to)}`
      : "";
  const body = (
    <>
      <span className="flex min-w-0 items-center justify-between gap-2">
        <span className="truncate text-body-2-regular text-text-secondary">{metric.label}</span>
        {delta ? (
          <Chip variant="caption" color={delta.tone} title={since}>
            {delta.text}
          </Chip>
        ) : null}
      </span>
      <span className="text-title-2-medium whitespace-nowrap text-text-primary tabular-nums">
        {formatValue(metric.unit, metric.value)}
      </span>
      {metric.unit === "seconds" && (metric.p90 !== null || metric.mean !== null) ? (
        <span className="truncate text-caption-1-medium text-text-tertiary tabular-nums">
          {[
            metric.p90 !== null ? `p90 ${formatValue("seconds", metric.p90)}` : null,
            metric.mean !== null ? `avg ${formatValue("seconds", metric.mean)}` : null,
          ]
            .filter(Boolean)
            .join(" · ")}
        </span>
      ) : null}
    </>
  );

  if (!metric.drill) {
    return <section className={tile}>{body}</section>;
  }
  const drill = metric.drill;

  return (
    <button
      type="button"
      title="Show the conversations"
      onClick={() => onDrill(drill)}
      className={cx(
        tile,
        "cursor-pointer outline-none transition-colors hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring",
      )}
    >
      {body}
    </button>
  );
}

const grid = "grid grid-cols-2 gap-2 lg:grid-cols-4";

export function MetricTiles({ report, onDrill }: { report: Report; onDrill: (drill: string) => void }) {
  if (report.metrics.length === 0) {
    return null;
  }

  return (
    <div className={grid}>
      {report.metrics.map((metric) => (
        <Tile key={metric.key} metric={metric} report={report} onDrill={onDrill} />
      ))}
    </div>
  );
}

export function MetricTilesSkeleton() {
  return (
    <div className={grid}>
      {["a", "b", "c", "d"].map((id) => (
        <Skeleton key={id} className="h-[100px] rounded-2xl" />
      ))}
    </div>
  );
}
