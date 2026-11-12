/*
 * The reports: the list of reports a member may read, one report for a
 * range and filters, the conversations behind one of its numbers, the
 * member's digest by mail, and the facts export. A report is data the
 * page draws without knowing which report it is: numbers with their
 * unit, and charts with their kind.
 */
import { queryOptions } from "@tanstack/react-query";
import { request, requestBody } from "./client";
import type { DataExport } from "./data-exports";

export interface ReportType {
  object: "report_type";
  key: string;
  title: string;
  summary: string;
  live: boolean;
}

export type ReportUnit = "count" | "percent" | "seconds" | "micros" | "score";

export interface ReportMetric {
  key: string;
  label: string;
  unit: ReportUnit;
  value: number | null;
  previous: number | null;
  p90: number | null;
  mean: number | null;
  sample: number;
  drill: string | null;
}

export interface ReportPoint {
  x: string;
  y: number | null;
}

export interface ReportSeries {
  key: string;
  label: string;
  drill: string | null;
  points: ReportPoint[];
}

export interface ReportColumn {
  key: string;
  label: string;
  unit: ReportUnit;
}

export interface ReportRow {
  key: string;
  label: string;
  values: (number | null)[];
  filter: Record<string, string>;
}

export type ReportChartKind = "line" | "bar" | "stacked_bar" | "heatmap" | "histogram" | "table";

export interface ReportChart {
  key: string;
  title: string;
  kind: ReportChartKind | (string & {});
  unit: ReportUnit;
  axis: "day" | "hour" | "category";
  drill: string | null;
  series: ReportSeries[];
  columns: ReportColumn[];
  rows: ReportRow[];
}

export interface Report {
  object: "report";
  key: string;
  title: string;
  from: string;
  to: string;
  previous_from: string | null;
  previous_to: string | null;
  timezone: string;
  business_hours: boolean;
  live: boolean;
  metrics: ReportMetric[];
  charts: ReportChart[];
}

/*
 * ReportFilters are the range and filters every report reads. Lists are
 * comma separated, as the query string carries them.
 */
export interface ReportFilters {
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
}

export const filterKeys = ["inbox", "team", "member", "channel", "tag", "priority", "ai"] as const;
export type FilterKey = (typeof filterKeys)[number];

/*
 * reportParams writes the range and filters as the query string.
 */
export function reportParams(filters: ReportFilters): URLSearchParams {
  const params = new URLSearchParams();
  if (filters.from) {
    params.set("from", filters.from);
  }
  if (filters.to) {
    params.set("to", filters.to);
  }
  if (filters.compare) {
    params.set("compare", "true");
  }
  if (filters.business_hours) {
    params.set("business_hours", "true");
  }
  for (const key of filterKeys) {
    const value = filters[key];
    if (value) {
      params.set(`filter[${key}]`, value);
    }
  }

  return params;
}

export const reportsKey = (workspace: string) => ["workspace", workspace, "reports"] as const;

export function reportTypesQuery(workspace: string) {
  return queryOptions({
    queryKey: [...reportsKey(workspace), "types"],
    queryFn: () => request<ReportType[]>("GET", "/reports", undefined, { workspace }),
    staleTime: 5 * 60_000,
  });
}

export function reportQuery(workspace: string, key: string, filters: ReportFilters) {
  return queryOptions({
    queryKey: [...reportsKey(workspace), "report", key, filters],
    queryFn: () =>
      request<Report>("GET", `/reports/${encodeURIComponent(key)}?${reportParams(filters).toString()}`, undefined, {
        workspace,
      }),
  });
}

export interface ReportConversation {
  object: "report_conversation";
  id: string;
  number: number;
  subject: string | null;
  status: string;
  channel: string;
  started_at: string;
  closed_at: string | null;
  assignee_id: string | null;
  value: number | null;
}

export interface ReportConversationsPage {
  data: ReportConversation[];
  total: number;
  capped: boolean;
  limit: number;
  offset: number;
}

export const drillPageSize = 25;

export function reportConversationsQuery(
  workspace: string,
  key: string,
  metric: string,
  filters: ReportFilters,
  offset: number,
) {
  const params = reportParams(filters);
  params.set("metric", metric);
  params.set("limit", String(drillPageSize));
  params.set("offset", String(offset));

  return queryOptions({
    queryKey: [...reportsKey(workspace), "conversations", key, metric, filters, offset],
    queryFn: () =>
      requestBody<ReportConversationsPage>(
        "GET",
        `/reports/${encodeURIComponent(key)}/conversations?${params.toString()}`,
        undefined,
        { workspace },
      ),
  });
}

export type DigestFrequency = "daily" | "weekly";

export interface ReportDigest {
  object: "report_digest";
  frequency: DigestFrequency | null;
  inbox_ids: string[];
  team_ids: string[];
  channels: string[];
  business_hours: boolean;
  next_send_at: string | null;
  last_sent_at: string | null;
}

export const reportDigestKey = (workspace: string) => [...reportsKey(workspace), "digest"] as const;

export function reportDigestQuery(workspace: string) {
  return queryOptions({
    queryKey: reportDigestKey(workspace),
    queryFn: () => request<ReportDigest>("GET", "/report-digest", undefined, { workspace }),
  });
}

export interface ReportDigestChanges {
  frequency: DigestFrequency;
  inbox_ids?: string[];
  team_ids?: string[];
  channels?: string[];
  business_hours?: boolean;
}

export function updateReportDigest(workspace: string, changes: ReportDigestChanges): Promise<ReportDigest> {
  return request<ReportDigest>("PATCH", "/report-digest", changes, { workspace });
}

export function deleteReportDigest(workspace: string): Promise<void> {
  return request<void>("DELETE", "/report-digest", undefined, { workspace });
}

/*
 * exportReportFacts starts a CSV of one row per conversation the range
 * and filters cover, built in the background like every export.
 */
export function exportReportFacts(workspace: string, filters: ReportFilters): Promise<DataExport> {
  const filter: Record<string, string> = {};
  if (filters.from) {
    filter.from = filters.from;
  }
  if (filters.to) {
    filter.to = filters.to;
  }
  if (filters.business_hours) {
    filter.business_hours = "true";
  }
  for (const key of filterKeys) {
    const value = filters[key];
    if (value) {
      filter[key] = value;
    }
  }

  return request<DataExport>(
    "POST",
    "/data-exports",
    { kind: "conversation_facts", format: "csv", selection: { filter } },
    { workspace },
  );
}
