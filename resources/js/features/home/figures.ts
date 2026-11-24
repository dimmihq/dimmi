/*
 * The words Home puts on its numbers: a person's initials, how long a
 * customer has waited, and how a week's figure moved against the week
 * before.
 */
import { formatDuration, formatValue } from "@/features/reports/format";
import type { Report, ReportMetric } from "@/lib/api/reports";
import type { Routine } from "@/lib/api/routines";

/*
 * initialsOf is the two letters an avatar shows for a name.
 */
export function initialsOf(name: string): string {
  return name
    .split(/\s+/)
    .map((part) => part[0] ?? "")
    .join("")
    .slice(0, 2)
    .toUpperCase();
}

/*
 * waited says how long ago a moment was, as "3h 5m" or "2d 4h".
 */
export function waited(since: string, now = Date.now()): string {
  return formatDuration(Math.max(0, Math.round((now - new Date(since).getTime()) / 1000)));
}

/*
 * metricOf finds one of a report's headline numbers by its key.
 */
export function metricOf(report: Report | undefined, key: string): ReportMetric | undefined {
  return report?.metrics.find((metric) => metric.key === key);
}

/*
 * changeOf says how a figure moved against the period before it: a
 * count by its share, a rate by where it came from, and a time by how
 * much faster or slower it got. It says nothing when there is nothing
 * to compare with.
 */
export function changeOf(metric: ReportMetric | undefined): string | undefined {
  if (!metric || metric.value === null || metric.previous === null) {
    return undefined;
  }
  const { value, previous, unit } = metric;
  if (value === previous) {
    return "no change";
  }
  if (unit === "seconds") {
    return `${formatDuration(Math.abs(value - previous))} ${value < previous ? "faster" : "slower"}`;
  }
  if (unit === "percent") {
    return `${value > previous ? "up" : "down"} from ${formatValue(unit, previous)}`;
  }
  if (previous === 0) {
    return undefined;
  }

  return `${value > previous ? "up" : "down"} ${Math.round((Math.abs(value - previous) / previous) * 100)}%`;
}

/*
 * told reports whether a routine tells the teammate about its runs, in
 * dimmi or by email. Home shows a routine's report to those people only.
 */
export function told(routine: Pick<Routine, "delivery">, member: string): boolean {
  return routine.delivery.in_app_member_ids.includes(member) || routine.delivery.email_member_ids.includes(member);
}
