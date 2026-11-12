/*
 * How a report's numbers read: durations in the largest units that
 * matter, costs in dollars, rates in percent, scores to one decimal,
 * and the change against the period before, coloured by whether the
 * change is good.
 */

import type { ReportMetric, ReportUnit } from "@/lib/api/reports";
import { formatMonthDay, formatNumber } from "@/lib/i18n/format";

/*
 * formatDuration writes seconds as "45s", "12m", "3h 5m" or "2d 4h".
 */
export function formatDuration(seconds: number): string {
  const s = Math.max(0, Math.round(seconds));
  if (s < 60) {
    return `${s}s`;
  }
  const minutes = Math.round(s / 60);
  if (minutes < 60) {
    return `${minutes}m`;
  }
  const hours = Math.floor(s / 3600);
  if (hours < 24) {
    const rest = Math.round((s - hours * 3600) / 60);
    return rest === 0 ? `${hours}h` : `${hours}h ${rest}m`;
  }
  const days = Math.floor(s / 86_400);
  const rest = Math.round((s - days * 86_400) / 3600);

  return rest === 0 ? `${days}d` : `${days}d ${rest}h`;
}

/*
 * formatTick writes an axis tick short enough to sit on one line: a
 * duration in its largest unit, anything else as formatValue does.
 */
export function formatTick(unit: ReportUnit | string, value: number): string {
  if (unit !== "seconds") {
    return formatValue(unit, value);
  }
  const steps: [number, string][] = [
    [86_400, "d"],
    [3600, "h"],
    [60, "m"],
  ];
  for (const [size, suffix] of steps) {
    if (value >= size) {
      return `${Math.round((value / size) * 10) / 10}${suffix}`;
    }
  }

  return `${Math.round(value)}s`;
}

/*
 * formatValue writes one value in its unit, or a dash for none.
 */
export function formatValue(unit: ReportUnit | string, value: number | null | undefined): string {
  if (value === null || value === undefined || Number.isNaN(value)) {
    return "—";
  }
  switch (unit) {
    case "seconds":
      return formatDuration(value);
    case "micros": {
      const dollars = value / 1e6;
      const digits = dollars !== 0 && Math.abs(dollars) < 1 ? 3 : 2;
      return formatNumber(dollars, {
        style: "currency",
        currency: "USD",
        minimumFractionDigits: 2,
        maximumFractionDigits: digits,
      });
    }
    case "percent":
      return `${formatNumber(value, { maximumFractionDigits: 1 })}%`;
    case "score":
      return formatNumber(value, { minimumFractionDigits: 1, maximumFractionDigits: 1 });
    default:
      return formatNumber(value, { maximumFractionDigits: 2 });
  }
}

/*
 * csvValue writes a value for a spreadsheet: plain numbers, durations in
 * seconds and costs in dollars, so they can be summed.
 */
export function csvValue(unit: ReportUnit | string, value: number | null | undefined): string {
  if (value === null || value === undefined || Number.isNaN(value)) {
    return "";
  }

  return String(unit === "micros" ? value / 1e6 : value);
}

const lowerIsBetterKeys = new Set([
  "low_ratings",
  "reopen_rate",
  "ai_handoff",
  "unassigned",
  "awaiting_first_response",
  "oldest_open",
  "median_open_age",
  "empty_searches",
  "ai_cost_per_resolution",
]);

export function lowerIsBetter(metric: Pick<ReportMetric, "key" | "unit">): boolean {
  return metric.unit === "seconds" || lowerIsBetterKeys.has(metric.key);
}

export interface Delta {
  text: string;
  tone: "lime" | "rose" | "neutral";
}

/*
 * deltaOf is the change against the period before: points for rates, a
 * percentage for everything else. Nothing when either side is missing,
 * or when there was nothing before to compare a percentage with.
 */
export function deltaOf(metric: ReportMetric): Delta | null {
  const { value, previous } = metric;
  if (value === null || previous === null) {
    return null;
  }
  const diff = value - previous;
  let text: string;
  if (metric.unit === "percent") {
    const points = Math.round(diff * 10) / 10;
    text = `${points > 0 ? "+" : ""}${formatNumber(points, { maximumFractionDigits: 1 })} pts`;
  } else {
    if (previous === 0) {
      return value === 0 ? { text: "0%", tone: "neutral" } : null;
    }
    const pct = Math.round((diff / Math.abs(previous)) * 1000) / 10;
    text = `${pct > 0 ? "+" : ""}${formatNumber(pct, { maximumFractionDigits: 1 })}%`;
  }
  if (diff === 0) {
    return { text, tone: "neutral" };
  }
  const good = diff > 0 !== lowerIsBetter(metric);

  return { text, tone: good ? "lime" : "rose" };
}

/*
 * formatDay writes a day of the range, such as "Nov 3". Days are read
 * as UTC so no time zone moves them.
 */
export function formatDay(day: string): string {
  return formatMonthDay(`${day}T00:00:00Z`, { timeZone: "UTC" });
}

export function formatRange(from: string, to: string): string {
  return from === to ? formatDay(from) : `${formatDay(from)} – ${formatDay(to)}`;
}

/*
 * formatAxis writes an x value of a chart for its axis.
 */
export function formatAxis(axis: string, x: string): string {
  if (axis === "day" && /^\d{4}-\d{2}-\d{2}$/.test(x)) {
    return formatDay(x);
  }
  if (axis === "hour" && /^\d{2}$/.test(x)) {
    return `${x}:00`;
  }

  return x;
}

/*
 * seriesColor is the chart colour of the series at the index.
 */
export function seriesColor(index: number, active = false): string {
  return `var(--color-chart-${(index % 9) + 1}${active ? "-active" : ""})`;
}
