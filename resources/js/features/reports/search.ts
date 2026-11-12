/*
 * A report view lives in its address, so it can be shared and reloaded:
 * the range (a rolling preset or two days), compare and business hours,
 * the filters, and the conversations open behind a number, with the day
 * or row they were narrowed to.
 */
import { getLocalTimeZone, parseDate, today } from "@internationalized/date";

import { filterKeys, type ReportFilters } from "@/lib/api/reports";

export const rangePresets = [
  { id: "today", label: "Today", days: 1 },
  { id: "7d", label: "Last 7 days", days: 7 },
  { id: "30d", label: "Last 30 days", days: 30 },
  { id: "90d", label: "Last 90 days", days: 90 },
] as const;

export type RangePreset = (typeof rangePresets)[number]["id"];

export interface ReportSearch {
  range?: RangePreset;
  from?: string;
  to?: string;
  compare?: boolean;
  business_hours?: boolean;
  inbox?: string;
  team?: string;
  member?: string;
  channel?: string;
  tag?: string;
  priority?: string;
  ai?: "involved" | "not_involved";
  /** The metric whose conversations are open. */
  drill?: string;
  /** The first and last day the open list is narrowed to. */
  drill_from?: string;
  drill_to?: string;
  /** A table row's filter the open list is narrowed to, as key=value;key=value. */
  drill_by?: string;
}

const day = /^\d{4}-\d{2}-\d{2}$/;

function text(value: unknown, max = 2000): string | undefined {
  if (typeof value === "number") {
    return String(value);
  }

  return typeof value === "string" && value !== "" && value.length <= max ? value : undefined;
}

function date(value: unknown): string | undefined {
  const v = text(value);

  return v && day.test(v) ? v : undefined;
}

function flag(value: unknown): true | undefined {
  return value === true || value === "true" ? true : undefined;
}

/*
 * validateReportSearch keeps only what a report reads, dropping anything
 * malformed instead of failing the page.
 */
export function validateReportSearch(search: Record<string, unknown>): ReportSearch {
  const range = rangePresets.find((p) => p.id === search.range)?.id;
  const out: ReportSearch = {
    range,
    from: range ? undefined : date(search.from),
    to: range ? undefined : date(search.to),
    compare: flag(search.compare),
    business_hours: flag(search.business_hours),
    inbox: text(search.inbox),
    team: text(search.team),
    member: text(search.member),
    channel: text(search.channel),
    tag: text(search.tag),
    priority: text(search.priority),
    ai: search.ai === "involved" || search.ai === "not_involved" ? search.ai : undefined,
    ...drillSearch(search),
  };

  return Object.fromEntries(Object.entries(out).filter(([, v]) => v !== undefined)) as ReportSearch;
}

function drillSearch(search: Record<string, unknown>): Partial<ReportSearch> {
  const drill = text(search.drill, 64);
  if (!drill) {
    return {};
  }

  return {
    drill,
    drill_from: date(search.drill_from),
    drill_to: date(search.drill_to),
    drill_by: text(search.drill_by, 500),
  };
}

/*
 * presetDays turns a rolling preset into its first and last day, today
 * included, in the reader's time zone.
 */
export function presetDays(preset: RangePreset): { from: string; to: string } {
  const now = today(getLocalTimeZone());
  const days = rangePresets.find((p) => p.id === preset)?.days ?? 30;

  return { from: now.subtract({ days: days - 1 }).toString(), to: now.toString() };
}

/*
 * reportFilters is what the report is asked for. With no range at all
 * the server answers the last 30 days. A live report has no range.
 */
export function reportFilters(search: ReportSearch, live = false): ReportFilters {
  const filters: ReportFilters = {};
  if (!live) {
    if (search.range) {
      Object.assign(filters, presetDays(search.range));
    } else {
      if (search.from) {
        filters.from = search.from;
      }
      if (search.to) {
        filters.to = search.to;
      }
    }
    if (search.compare) {
      filters.compare = true;
    }
  }
  if (search.business_hours) {
    filters.business_hours = true;
  }
  for (const key of filterKeys) {
    const value = search[key];
    if (value) {
      Object.assign(filters, { [key]: value });
    }
  }

  return filters;
}

/*
 * drillFilters is what the open list is asked for: the report's filters
 * narrowed to the day or row picked. A row's filter replaces the
 * report's filter of the same name.
 */
export function drillFilters(search: ReportSearch, live = false): ReportFilters {
  const filters = reportFilters(search, live);
  delete filters.compare;
  if (search.drill_from) {
    filters.from = search.drill_from;
  }
  if (search.drill_to) {
    filters.to = search.drill_to;
  }
  for (const [key, value] of Object.entries(parseDrillBy(search.drill_by))) {
    if ((filterKeys as readonly string[]).includes(key)) {
      Object.assign(filters, { [key]: value });
    }
  }

  return filters;
}

export function parseDrillBy(value: string | undefined): Record<string, string> {
  const out: Record<string, string> = {};
  for (const pair of (value ?? "").split(";")) {
    const at = pair.indexOf("=");
    if (at > 0) {
      out[pair.slice(0, at)] = pair.slice(at + 1);
    }
  }

  return out;
}

export function writeDrillBy(filter: Record<string, string>): string | undefined {
  const pairs = Object.entries(filter).map(([k, v]) => `${k}=${v}`);

  return pairs.length > 0 ? pairs.join(";") : undefined;
}

/*
 * withoutDrill is the search with the open list closed.
 */
export function withoutDrill(search: ReportSearch): ReportSearch {
  const { drill: _d, drill_from: _f, drill_to: _t, drill_by: _b, ...rest } = search;

  return rest;
}

/*
 * calendarRange is the range as the date picker holds it, or null when
 * it cannot be read.
 */
export function calendarRange(from: string, to: string) {
  try {
    return { start: parseDate(from), end: parseDate(to) };
  } catch {
    return null;
  }
}
