/*
 * The two logs people answer for: the workspace's audit log, read by its
 * owners and admins a page at a time, and the person's own security log
 * of sign-ins and account changes. Pages are cut by an opaque cursor the
 * server hands back with each page.
 */
import { queryOptions } from "@tanstack/react-query";

import { ApiError, requestFile, requestPage } from "./client";

export interface AuditEntry {
  id: string;
  event: string;
  actor: { type: string; id: string | null; label: string; via: { kind: string; id: string | null } | null };
  subject: { type: string; id: string | null; label: string | null } | null;
  changes: Record<string, unknown>;
  ip: string | null;
  user_agent?: string | null;
  request_id?: string | null;
  created_at: string;
}

export interface SecurityEvent {
  id: string;
  type: string;
  ip: string | null;
  device_label: string | null;
  metadata: Record<string, unknown>;
  created_at: string;
}

/*
 * AuditFilters narrows the log: event is one event name or a comma
 * list of them, actor a member's id, and from and to the start and the
 * end of a time range as RFC 3339 instants.
 */
export interface AuditFilters {
  event?: string;
  actor?: string;
  from?: string;
  to?: string;
}

/*
 * filterParams writes the filters as query parameters.
 */
function filterParams(params: URLSearchParams, filters: AuditFilters) {
  const names: [keyof AuditFilters, string][] = [
    ["event", "filter[event]"],
    ["actor", "filter[actor]"],
    ["from", "filter[created_after]"],
    ["to", "filter[created_before]"],
  ];
  for (const [key, name] of names) {
    const value = filters[key];
    if (value) {
      params.set(name, value);
    }
  }
}

/*
 * auditPath builds the address of one page of the audit log.
 */
export function auditPath(filters: AuditFilters, cursor: string | null, limit: number): string {
  const params = new URLSearchParams({ limit: String(limit) });
  filterParams(params, filters);
  if (cursor) {
    params.set("cursor", cursor);
  }

  return `/audit-logs?${params.toString()}`;
}

/*
 * exportPath builds the address of the audit log as a CSV file,
 * narrowed by the same filters as the page on screen.
 */
export function exportPath(filters: AuditFilters): string {
  const params = new URLSearchParams({ format: "csv" });
  filterParams(params, filters);

  return `/audit-logs?${params.toString()}`;
}

/*
 * exportAuditLog fetches the audit log as a CSV file.
 */
export function exportAuditLog(workspace: string, filters: AuditFilters) {
  return requestFile(exportPath(filters), "text/csv", { workspace });
}

export function auditLogsQuery(workspace: string, filters: AuditFilters, cursor: string | null) {
  return queryOptions({
    queryKey: ["workspace", workspace, "audit-logs", filters, cursor],
    queryFn: () => requestPage<AuditEntry>(auditPath(filters, cursor, 25), { workspace }),
    retry: (count, error) => !(error instanceof ApiError && error.status === 403) && count < 2,
  });
}

export function securityEventsQuery() {
  return queryOptions({
    queryKey: ["user", "security-events"],
    queryFn: () => requestPage<SecurityEvent>("/user/security-events?limit=10"),
  });
}
