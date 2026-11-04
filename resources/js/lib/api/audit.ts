/*
 * The two logs people answer for: the workspace's audit log, read by its
 * owners and admins a page at a time, and the person's own security log
 * of sign-ins and account changes. Pages are cut by an opaque cursor the
 * server hands back with each page.
 */
import { queryOptions } from "@tanstack/react-query";

import { ApiError, requestPage } from "./client";

export interface AuditEntry {
  id: string;
  event: string;
  actor: { type: string; id: string | null; label: string; via: { kind: string; id: string | null } | null };
  subject: { type: string; id: string | null; label: string | null } | null;
  changes: Record<string, unknown>;
  ip: string | null;
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

export interface AuditFilters {
  event?: string;
  actor?: string;
}

/*
 * auditPath builds the address of one page of the audit log.
 */
export function auditPath(filters: AuditFilters, cursor: string | null, limit: number): string {
  const params = new URLSearchParams({ limit: String(limit) });
  if (filters.event) {
    params.set("filter[event]", filters.event);
  }
  if (filters.actor) {
    params.set("filter[actor]", filters.actor);
  }
  if (cursor) {
    params.set("cursor", cursor);
  }

  return `/audit-logs?${params.toString()}`;
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
