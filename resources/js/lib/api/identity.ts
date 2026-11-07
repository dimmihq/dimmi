/*
 * The secrets a workspace's server signs messenger visitors' identity
 * tokens with: the query that lists them and the calls that create,
 * rotate and revoke one. A shared secret comes back once, in the answer
 * that made it, and is never listed again. A secret can instead point at
 * an auth provider's key set. Beside the secrets: how strict sign-in is,
 * how sign-ins went lately, and a checker that explains a pasted token.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export type IdentityAlgorithm = "HS256" | "RS256" | "ES256" | "EdDSA";

export type IdentityMode = "off" | "optional" | "required";

export interface IdentitySecret {
  id: string;
  object: "identity_secret";
  name: string;
  algorithm: IdentityAlgorithm;
  source: "shared_secret" | "public_key" | "jwks";
  kid: string;
  jwks_url: string | null;
  issuer: string | null;
  audience: string | null;
  jwks_fetched_at: string | null;
  jwks_error: string | null;
  fingerprint: string;
  status: "active" | "retiring" | "revoked";
  retires_at: string | null;
  revoked_at: string | null;
  last_used_at: string | null;
  created_at: string;
}

export interface CreatedIdentitySecret extends IdentitySecret {
  secret?: string;
}

export interface NewIdentitySecret {
  name: string;
  algorithm: IdentityAlgorithm;
  public_key?: string;
  jwks_url?: string;
  issuer?: string;
}

export interface IdentitySettings {
  object: "identity_settings";
  mode: IdentityMode;
  client_writable_attributes: string[];
  updated_at: string | null;
}

export interface IdentityStats {
  object: "identity_stats";
  days: number;
  totals: Record<string, number>;
  by_day: { day: string; outcomes: Record<string, number> }[];
  verified: number;
  invalid: number;
  anonymous: number;
  verified_share: number;
}

export interface IdentityMistake {
  code: string;
  hint: string;
}

export interface IdentityValidation {
  object: "identity_validation";
  valid: boolean;
  code: string | null;
  message: string | null;
  hint: string | null;
  mistake: IdentityMistake | null;
  header: Record<string, unknown> | null;
  claims: Record<string, unknown> | null;
  contact_preview: { action: "create" | "reuse"; contact_id: string | null } | null;
  warnings: IdentityMistake[];
}

export function identitySettingsQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "identity-settings"],
    queryFn: () => request<IdentitySettings>("GET", "/identity-verification/settings", undefined, { workspace }),
  });
}

export function identityStatsQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "identity-stats"],
    queryFn: () => request<IdentityStats>("GET", "/identity-verification/stats?days=7", undefined, { workspace }),
  });
}

export function updateIdentitySettings(
  workspace: string,
  change: { mode?: IdentityMode; force?: boolean; client_writable_attributes?: string[] },
): Promise<IdentitySettings> {
  return request<IdentitySettings>("PATCH", "/identity-verification/settings", change, { workspace });
}

export function checkIdentityToken(workspace: string, token: string): Promise<IdentityValidation> {
  return request<IdentityValidation>("POST", "/identity-verification/validations", { token }, { workspace });
}

export function identitySecretsQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "identity-secrets"],
    queryFn: () => request<IdentitySecret[]>("GET", "/identity-secrets", undefined, { workspace }),
  });
}

export function createIdentitySecret(workspace: string, secret: NewIdentitySecret): Promise<CreatedIdentitySecret> {
  return request<CreatedIdentitySecret>("POST", "/identity-secrets", secret, { workspace });
}

export function rotateIdentitySecret(
  workspace: string,
  id: string,
  publicKey?: string,
): Promise<CreatedIdentitySecret> {
  return request<CreatedIdentitySecret>(
    "POST",
    `/identity-secrets/${encodeURIComponent(id)}/rotations`,
    publicKey ? { public_key: publicKey } : {},
    { workspace },
  );
}

export function revokeIdentitySecret(workspace: string, id: string): Promise<IdentitySecret> {
  return request<IdentitySecret>(
    "PATCH",
    `/identity-secrets/${encodeURIComponent(id)}`,
    { status: "revoked", reason: "manual" },
    { workspace },
  );
}
