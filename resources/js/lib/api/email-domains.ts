/*
 * The workspace's sending domains: the queries that read them and the
 * calls that add one, ask for a check, change its return path or DNS
 * host, rotate a relay's signing key and remove it. Every call names
 * the workspace in the Dimmi-Workspace header.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export type DomainStatus = "pending" | "verifying" | "verified" | "degraded" | "failed";

export type RecordStatus = "verified" | "mismatch" | "missing" | "propagating" | "error" | "info";

export interface DomainRecord {
  purpose: string;
  type: string;
  name: string;
  host: string;
  value: string;
  priority?: number;
  required: boolean;
  status: RecordStatus;
  observed: string[];
  selector?: string;
}

export interface DomainWarning {
  code: string;
  message: string;
  host?: string;
  value?: string;
}

export interface HostGuide {
  key: string;
  name: string;
  deep_link: string;
  host_form: string;
  steps: string[];
  quirks: string[];
  detected: boolean;
}

export interface EmailDomain {
  id: string;
  domain: string;
  ascii_domain: string;
  status: DomainStatus;
  driver: string;
  records: DomainRecord[];
  warnings: DomainWarning[];
  provider_hint: string;
  host_guide: HostGuide;
  dns_setup: { host_name: string } | null;
  mail_from_domain: string | null;
  provider_trusted: boolean;
  failure_reason: string | null;
  failure_message: string | null;
  last_checked_at: string | null;
  next_check_at: string | null;
  verified_at: string | null;
  links: { self: string; checks: string; zone_file: string };
}

export interface EmailDomainChanges {
  mail_from_subdomain?: string;
  dns_host?: string;
}

/*
 * domainsKey is the cache key every domain query starts with, so one
 * refresh covers the list and each domain.
 */
export function domainsKey(workspace: string) {
  return ["workspace", workspace, "email-domains"];
}

export function emailDomainsQuery(workspace: string) {
  return queryOptions({
    queryKey: domainsKey(workspace),
    queryFn: () => request<EmailDomain[]>("GET", "/email-domains", undefined, { workspace }),
  });
}

export function emailDomainQuery(workspace: string, domain: string) {
  return queryOptions({
    queryKey: [...domainsKey(workspace), domain],
    queryFn: () =>
      request<EmailDomain>("GET", `/email-domains/${encodeURIComponent(domain)}`, undefined, { workspace }),
  });
}

export function createEmailDomain(workspace: string, domain: string): Promise<EmailDomain> {
  return request<EmailDomain>("POST", "/email-domains", { domain }, { workspace });
}

export function checkEmailDomain(workspace: string, domain: string): Promise<EmailDomain> {
  return request<EmailDomain>("POST", `/email-domains/${encodeURIComponent(domain)}/checks`, undefined, {
    workspace,
  });
}

export function updateEmailDomain(workspace: string, domain: string, changes: EmailDomainChanges) {
  return request<EmailDomain>("PATCH", `/email-domains/${encodeURIComponent(domain)}`, changes, { workspace });
}

export function startDNSSetup(workspace: string, domain: string): Promise<{ apply_url: string }> {
  return request<{ apply_url: string }>("POST", `/email-domains/${encodeURIComponent(domain)}/dns-setups`, undefined, {
    workspace,
  });
}

export function rotateEmailDomainKey(workspace: string, domain: string): Promise<EmailDomain> {
  return request<EmailDomain>("POST", `/email-domains/${encodeURIComponent(domain)}/dkim-rotations`, undefined, {
    workspace,
  });
}

export function deleteEmailDomain(workspace: string, domain: string, force: boolean): Promise<void> {
  const query = force ? "?force=true" : "";

  return request<void>("DELETE", `/email-domains/${encodeURIComponent(domain)}${query}`, undefined, { workspace });
}
