/*
 * A workspace's help centers as sites: adding and deleting them, the
 * custom domain one answers on with the DNS records the team creates,
 * and what readers read and searched for there.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";
import type { HelpCenter } from "./help-centers";

export interface DNSRecord {
  type: "CNAME" | "TXT";
  name: string;
  value: string;
}

export type DomainStatus = "pending" | "verified" | "failed";

export interface HelpCenterDomain {
  object: "help_center_domain";
  help_center_id: string;
  hostname: string;
  status: DomainStatus;
  failure_reason: string | null;
  records: DNSRecord[];
  last_checked_at: string | null;
  next_check_at: string | null;
  verified_at: string | null;
  created_at: string;
}

export interface HelpCenterAnalytics {
  object: "help_center_analytics";
  days: number;
  since: string;
  views: number;
  daily: { date: string; views: number; messenger: number }[];
  top_articles: { id: string; title: string; slug: string; views: number; helpful: number; unhelpful: number }[];
  helpful: number;
  unhelpful: number;
  helpful_percent: number | null;
  searches: number;
  empty_searches: number;
  content_gaps: { query: string; searches: number; last_date: string }[];
}

export function createHelpCenter(
  workspace: string,
  input: { name: string; subdomain?: string; locale?: string },
): Promise<HelpCenter> {
  return request<HelpCenter>("POST", "/help-centers", input, { workspace });
}

export function deleteHelpCenter(workspace: string, id: string, confirm: string): Promise<void> {
  const path = `/help-centers/${encodeURIComponent(id)}?confirm=${encodeURIComponent(confirm)}`;

  return request<void>("DELETE", path, undefined, { workspace });
}

export function helpCenterDomainQuery(workspace: string, helpCenter: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "help-centers", helpCenter, "domain"],
    queryFn: () =>
      request<HelpCenterDomain | null>("GET", `/help-centers/${encodeURIComponent(helpCenter)}/domain`, undefined, {
        workspace,
      }),
    refetchInterval: (query) => (query.state.data?.status === "pending" ? 15_000 : false),
  });
}

export function setHelpCenterDomain(workspace: string, helpCenter: string, hostname: string) {
  return request<HelpCenterDomain>(
    "PATCH",
    `/help-centers/${encodeURIComponent(helpCenter)}/domain`,
    { hostname },
    { workspace },
  );
}

export function checkHelpCenterDomain(workspace: string, helpCenter: string) {
  return request<HelpCenterDomain>("POST", `/help-centers/${encodeURIComponent(helpCenter)}/domain/checks`, undefined, {
    workspace,
  });
}

export function removeHelpCenterDomain(workspace: string, helpCenter: string): Promise<void> {
  return request<void>("DELETE", `/help-centers/${encodeURIComponent(helpCenter)}/domain`, undefined, { workspace });
}

export function helpCenterAnalyticsQuery(workspace: string, helpCenter: string, days = 30) {
  return queryOptions({
    queryKey: ["workspace", workspace, "help-centers", helpCenter, "analytics", days],
    queryFn: () =>
      request<HelpCenterAnalytics>(
        "GET",
        `/help-centers/${encodeURIComponent(helpCenter)}/analytics?days=${days}`,
        undefined,
        { workspace },
      ),
  });
}
