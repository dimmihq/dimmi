/*
 * The drawn charts of a report, driven only by the chart's kind: lines
 * and bars over days, hours or categories, a week of hours as a heat
 * map, and tables. Any report, one added later included, draws without
 * code of its own. Clicking a point opens the conversations behind it,
 * narrowed to that day when the axis is days.
 */
import {
  Bar,
  BarChart,
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  type TooltipContentProps,
  type TooltipValueType,
  XAxis,
  YAxis,
} from "recharts";

import { Table, TableBody, TableCell, TableColumn, TableHeader, TableRow } from "@/components/base/table/table";
import type { ReportChart } from "@/lib/api/reports";
import { cx } from "@/utils/cx";

import { formatAxis, formatTick, formatValue, seriesColor } from "./format";

/*
 * PointPick is what a click on a chart picked: the x value and the drill
 * that lists its conversations.
 */
export interface PointPick {
  x: string;
  drill: string;
}

function rowsOf(chart: ReportChart): Record<string, string | number | null>[] {
  const xs: string[] = [];
  const seen = new Set<string>();
  const values = chart.series.map((series) => new Map(series.points.map((p) => [p.x, p.y])));
  for (const series of chart.series) {
    for (const point of series.points) {
      if (!seen.has(point.x)) {
        seen.add(point.x);
        xs.push(point.x);
      }
    }
  }

  return xs.map((x) => {
    const row: Record<string, string | number | null> = { x };
    values.forEach((map, i) => {
      row[`s${i}`] = map.get(x) ?? null;
    });
    return row;
  });
}

function ChartTooltip({
  active,
  payload,
  label,
  chart,
}: Pick<Partial<TooltipContentProps<TooltipValueType, string | number>>, "active" | "payload" | "label"> & {
  chart: ReportChart;
}) {
  if (!active || !payload || payload.length === 0) {
    return null;
  }

  return (
    <div className="flex min-w-36 flex-col gap-1.5 rounded-xl bg-background-primary-default p-2.5 shadow-card">
      <p className="text-caption-1-medium text-text-tertiary">{formatAxis(chart.axis, String(label ?? ""))}</p>
      {payload.map((entry) => {
        const index = Number(String(entry.dataKey ?? "").slice(1));
        const series = chart.series[index];
        return (
          <p key={String(entry.dataKey)} className="flex items-center gap-2 text-body-2-regular text-text-secondary">
            <span aria-hidden className="size-2 shrink-0 rounded-full" style={{ background: seriesColor(index) }} />
            <span className="min-w-0 flex-1 truncate">{series?.label}</span>
            <span className="text-body-2-medium text-text-primary tabular-nums">
              {formatValue(chart.unit, typeof entry.value === "number" ? entry.value : null)}
            </span>
          </p>
        );
      })}
    </div>
  );
}

const tick = { fontSize: 12, fill: "var(--color-text-tertiary)" };

/*
 * drillOf is the drill a click on a cartesian chart opens: the first
 * series' own, else the chart's.
 */
function drillOf(chart: ReportChart): string | null {
  return chart.series.find((s) => s.drill)?.drill ?? chart.drill;
}

export function CartesianChart({ chart, onPick }: { chart: ReportChart; onPick: (pick: PointPick) => void }) {
  const data = rowsOf(chart);
  const drill = drillOf(chart);
  const kind = chart.kind;
  const stacked = kind === "stacked_bar";
  const common = {
    data,
    margin: { top: 8, right: 4, bottom: 0, left: 0 },
    style: drill ? { cursor: "pointer" } : undefined,
    onClick: (state: { activeTooltipIndex?: unknown }) => {
      const row = data[Number(state.activeTooltipIndex)];
      if (drill && row) {
        onPick({ x: String(row.x), drill });
      }
    },
  };
  const axes = [
    <CartesianGrid key="grid" vertical={false} stroke="var(--color-chart-track)" strokeDasharray="3 3" />,
    <XAxis
      key="x"
      dataKey="x"
      tick={tick}
      tickLine={false}
      axisLine={false}
      minTickGap={16}
      tickFormatter={(x: string) => formatAxis(chart.axis, String(x))}
    />,
    <YAxis
      key="y"
      width={52}
      tick={tick}
      tickLine={false}
      axisLine={false}
      allowDecimals={chart.unit !== "count"}
      tickFormatter={(v: number) => formatTick(chart.unit, v)}
    />,
  ];

  return (
    <div className="h-56 w-full min-w-0">
      <ResponsiveContainer width="100%" height="100%">
        {kind === "line" ? (
          <LineChart {...common}>
            {axes}
            <Tooltip
              cursor={{ stroke: "var(--color-chart-cursor)" }}
              content={(props) => <ChartTooltip {...props} chart={chart} />}
            />
            {chart.series.map((series, i) => (
              <Line
                key={series.key}
                type="monotone"
                dataKey={`s${i}`}
                name={series.label}
                stroke={seriesColor(i)}
                strokeWidth={2}
                dot={false}
                activeDot={{ r: 4, fill: seriesColor(i, true), strokeWidth: 0 }}
                connectNulls
                isAnimationActive={false}
              />
            ))}
          </LineChart>
        ) : (
          <BarChart {...common}>
            {axes}
            <Tooltip
              cursor={{ fill: "var(--color-chart-cursor)", opacity: 0.35 }}
              content={(props) => <ChartTooltip {...props} chart={chart} />}
            />
            {chart.series.map((series, i) => (
              <Bar
                key={series.key}
                dataKey={`s${i}`}
                name={series.label}
                fill={seriesColor(i)}
                activeBar={{ fill: seriesColor(i, true) }}
                stackId={stacked ? "stack" : undefined}
                radius={!stacked || i === chart.series.length - 1 ? [4, 4, 0, 0] : [0, 0, 0, 0]}
                maxBarSize={32}
                isAnimationActive={false}
              />
            ))}
          </BarChart>
        )}
      </ResponsiveContainer>
    </div>
  );
}

/*
 * Heatmap draws a week of hours, one row a weekday, darker the busier.
 */
export function Heatmap({ chart }: { chart: ReportChart }) {
  const most = Math.max(0, ...chart.series.flatMap((s) => s.points.map((p) => p.y ?? 0)));
  const hours = chart.series[0]?.points.map((p) => p.x) ?? [];
  const fill = (y: number | null) =>
    !y || most === 0
      ? "var(--color-chart-track)"
      : `color-mix(in oklab, var(--color-chart-6-active) ${Math.round(18 + (82 * y) / most)}%, var(--color-chart-track))`;

  return (
    <div className="flex flex-col gap-2">
      <div
        role="img"
        aria-label={chart.title}
        className="grid gap-[2px]"
        style={{ gridTemplateColumns: `2.25rem repeat(${Math.max(1, hours.length)}, minmax(0, 1fr))` }}
      >
        {chart.series.map((series) => (
          <div key={series.key} className="contents">
            <span className="truncate pe-1 text-caption-1-regular leading-5 text-text-tertiary sm:leading-7">
              {series.label.slice(0, 3)}
            </span>
            {series.points.map((point) => {
              const text = `${series.label} ${formatAxis("hour", point.x)} · ${formatValue(chart.unit, point.y)}`;
              return (
                <span
                  key={point.x}
                  title={text}
                  className="h-5 rounded-[3px] sm:h-7"
                  style={{ background: fill(point.y) }}
                />
              );
            })}
          </div>
        ))}
        <span />
        {hours.map((h, i) => (
          <span key={h} className="text-caption-1-regular text-text-tertiary">
            {i % 6 === 0 ? h : ""}
          </span>
        ))}
      </div>
      <div className="flex items-center justify-end gap-1 text-caption-1-regular text-text-tertiary">
        <span>Less</span>
        {[0, 0.25, 0.5, 0.75, 1].map((step) => (
          <span key={step} aria-hidden className="size-3 rounded-[3px]" style={{ background: fill(step * most) }} />
        ))}
        <span>More</span>
      </div>
    </div>
  );
}

/*
 * columnClass hides the less important columns on narrow screens, so
 * the table always fits: the first two columns of numbers always show.
 */
const labelColumn = "label";

function columnClass(index: number): string | undefined {
  if (index >= 4) {
    return "hidden xl:table-cell";
  }
  if (index >= 2) {
    return "hidden sm:table-cell";
  }

  return undefined;
}

/*
 * ReportTable draws a table chart. A row that narrows the report or has
 * conversations behind it can be clicked.
 */
export function ReportTable({
  chart,
  onRow,
}: {
  chart: ReportChart;
  onRow?: (row: ReportChart["rows"][number]) => void;
}) {
  const clickable = (row: ReportChart["rows"][number]) =>
    onRow !== undefined && (Object.keys(row.filter).length > 0 || chart.drill !== null);

  return (
    <div className="-mx-4 -mb-2 min-w-0">
      <Table
        aria-label={chart.title}
        size="sm"
        className="w-full [&_th]:border-t-0"
        containerClassName="overflow-x-hidden"
        onRowAction={(key) => {
          const row = chart.rows.find((r) => r.key === key);
          if (row && clickable(row)) {
            onRow?.(row);
          }
        }}
      >
        <TableHeader>
          <TableColumn id={labelColumn} isRowHeader className="w-full ps-4">
            <span className="sr-only">Name</span>
          </TableColumn>
          {chart.columns.map((column, i) => (
            <TableColumn key={column.key} id={column.key} className={cx("text-end", columnClass(i))}>
              {column.label}
            </TableColumn>
          ))}
        </TableHeader>
        <TableBody
          renderEmptyState={() => (
            <p className="px-4 py-6 text-body-2-regular text-text-tertiary">Nothing in this range.</p>
          )}
        >
          {chart.rows.map((row) => (
            <TableRow
              key={row.key}
              id={row.key}
              className={cx(clickable(row) && "cursor-pointer hover:bg-background-secondary-hover")}
            >
              <TableCell className="max-w-0 ps-4">
                <span className="block truncate text-body-2-medium text-text-primary">{row.label}</span>
              </TableCell>
              {chart.columns.map((column, i) => (
                <TableCell key={column.key} className={cx("text-end whitespace-nowrap", columnClass(i))}>
                  <span className="text-body-2-regular text-text-secondary tabular-nums">
                    {formatValue(column.unit, row.values[i])}
                  </span>
                </TableCell>
              ))}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
