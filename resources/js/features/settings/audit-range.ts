/*
 * The time range the audit log is narrowed to. A preset such as the
 * last 7 days runs up to the moment it is applied, to the minute; days
 * picked on the calendar run from the start of the first day to the end
 * of the last, in the reader's time zone. The calendar never offers days
 * older than the server keeps.
 */
import { type CalendarDate, today } from "@internationalized/date";

import type { DateRangePreset, DateRangeValue } from "@/components/base/date-picker/date-range-picker";
import type { AuditFilters } from "@/lib/api/audit";

const hour = 3_600_000;

export const rangePresets = [
  { id: "24h", label: "Last 24 hours", hours: 24 },
  { id: "7d", label: "Last 7 days", hours: 7 * 24 },
  { id: "30d", label: "Last 30 days", hours: 30 * 24 },
  { id: "90d", label: "Last 90 days", hours: 90 * 24 },
];

/*
 * presetRanges are the presets as the calendar shows them: the days
 * each one touches, today included.
 */
export function presetRanges(zone: string): DateRangePreset[] {
  const now = today(zone);

  return rangePresets.map((preset) => ({
    id: preset.id,
    label: preset.label,
    range: { start: now.subtract({ days: preset.hours / 24 }), end: now },
  }));
}

/*
 * rangeFilters turns the range picked into the time filters of the log:
 * nothing for no range, a start for a preset, and both ends for days.
 */
export function rangeFilters(
  value: DateRangeValue | null,
  preset: string | null,
  zone: string,
  now: Date,
): Pick<AuditFilters, "from" | "to"> {
  if (!value) {
    return {};
  }

  const known = rangePresets.find((p) => p.id === preset);
  if (known) {
    return { from: new Date(now.getTime() - known.hours * hour).toISOString() };
  }

  return {
    from: value.start.toDate(zone).toISOString(),
    to: value.end.add({ days: 1 }).toDate(zone).toISOString(),
  };
}

/*
 * oldestDay is the first day the log still holds, or undefined when the
 * server keeps entries for ever.
 */
export function oldestDay(retentionDays: number, zone: string): CalendarDate | undefined {
  if (retentionDays <= 0) {
    return undefined;
  }

  return today(zone).subtract({ days: retentionDays });
}
