/*
 * The company's single sign-on: its connections to identity providers
 * and the domains it proves it owns. A connection's secret is only ever
 * sent, never read back; a domain carries the TXT record that proves it.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export type ConnectionStatus = "draft" | "tested" | "active" | "disabled";

export interface ConnectionConfig {
  idp_entity_id?: string;
  idp_sso_url?: string;
  idp_certificates?: string[];
  idp_metadata_url?: string;
  sp_certificate?: string;
  issuer?: string;
  client_id?: string;
}

export interface SSOConnection {
  id: string;
  protocol: "saml" | "oidc";
  name: string;
  status: ConnectionStatus;
  config: ConnectionConfig;
  has_secret: boolean;
  enforced: boolean;
  tested_at: string | null;
  last_login_at: string | null;
  sp: { entity_id?: string; acs_url?: string; metadata_url?: string; redirect_uri?: string };
}

export interface ConnectionDraft {
  protocol: "saml" | "oidc";
  name: string;
  config: ConnectionConfig;
  client_secret?: string;
}

export interface ConnectionChange {
  status?: ConnectionStatus;
  enforced?: boolean;
  confirm_owners_can_sign_in?: boolean;
}

export interface OrganizationDomain {
  id: string;
  domain: string;
  status: "pending" | "verified" | "failed";
  record_name: string;
  record_value: string;
  verified_via: "dns" | "cli" | null;
  verified_at: string | null;
  sso_connection_id: string | null;
}

export interface DomainCheck {
  status: OrganizationDomain["status"];
  found_records: string[];
  domain: OrganizationDomain;
}

export function connectionsQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "sso-connections"],
    queryFn: () => request<SSOConnection[]>("GET", "/sso-connections", undefined, { workspace }),
  });
}

export function domainsQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "organization-domains"],
    queryFn: () => request<OrganizationDomain[]>("GET", "/organization-domains", undefined, { workspace }),
  });
}

export function createConnection(workspace: string, draft: ConnectionDraft): Promise<SSOConnection> {
  return request<SSOConnection>("POST", "/sso-connections", draft, { workspace });
}

export function updateConnection(workspace: string, id: string, change: ConnectionChange): Promise<SSOConnection> {
  return request<SSOConnection>("PATCH", `/sso-connections/${encodeURIComponent(id)}`, change, { workspace });
}

export function deleteConnection(workspace: string, id: string): Promise<void> {
  return request<void>("DELETE", `/sso-connections/${encodeURIComponent(id)}`, undefined, { workspace });
}

export function addDomain(workspace: string, domain: string): Promise<OrganizationDomain> {
  return request<OrganizationDomain>("POST", "/organization-domains", { domain }, { workspace });
}

export function checkDomain(workspace: string, id: string): Promise<DomainCheck> {
  return request<DomainCheck>("POST", `/organization-domains/${encodeURIComponent(id)}/checks`, undefined, {
    workspace,
  });
}

export function routeDomain(workspace: string, id: string, connection: string | null): Promise<OrganizationDomain> {
  return request<OrganizationDomain>(
    "PATCH",
    `/organization-domains/${encodeURIComponent(id)}`,
    { sso_connection_id: connection },
    { workspace },
  );
}

export function removeDomain(workspace: string, id: string): Promise<void> {
  return request<void>("DELETE", `/organization-domains/${encodeURIComponent(id)}`, undefined, { workspace });
}
