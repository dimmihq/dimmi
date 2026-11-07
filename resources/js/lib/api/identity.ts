/*
 * The secrets a workspace's server signs messenger visitors' identity
 * tokens with: the query that lists them and the calls that create,
 * rotate and revoke one. A shared secret comes back once, in the answer
 * that made it, and is never listed again.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export type IdentityAlgorithm = "HS256" | "RS256" | "EdDSA";

export interface IdentitySecret {
  id: string;
  object: "identity_secret";
  name: string;
  algorithm: IdentityAlgorithm;
  kid: string;
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
