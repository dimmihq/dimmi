/*
 * A workspace's rules for who may work in it: how people sign in, how
 * long a session lasts and which addresses may connect. A change sends
 * only the keys it touches and a list replaces the whole stored list.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export interface AllowedAddress {
  cidr: string;
  label: string;
}

export interface AccessPolicy {
  allowed_sign_in_methods: string[];
  google_allowed_domains: string[];
  session_max_age_minutes: number | null;
  session_idle_timeout_minutes: number | null;
  dashboard_ip_allowlist: AllowedAddress[];
  api_ip_allowlist: AllowedAddress[];
  require_two_factor: boolean;
  sso_enforced: boolean;
  available_sign_in_methods: string[];
  your_ip: string;
  tokens_outside: number;
}

export type AccessPolicyChange = Partial<
  Pick<
    AccessPolicy,
    | "allowed_sign_in_methods"
    | "google_allowed_domains"
    | "session_max_age_minutes"
    | "session_idle_timeout_minutes"
    | "dashboard_ip_allowlist"
    | "api_ip_allowlist"
  >
>;

export function accessPolicyQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "access-policy"],
    queryFn: () => request<AccessPolicy>("GET", "/workspace/access-policy", undefined, { workspace }),
  });
}

export function updateAccessPolicy(workspace: string, change: AccessPolicyChange): Promise<AccessPolicy> {
  return request<AccessPolicy>("PATCH", "/workspace/access-policy", change, { workspace });
}
