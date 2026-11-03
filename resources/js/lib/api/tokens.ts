/*
 * The person's personal access tokens in the workspace on screen: the
 * queries that list them and the abilities a token may be given, and the
 * calls that mint, rename and revoke one. A token's secret comes back
 * once, in the answer that minted it, and is never listed again.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export interface PersonalAccessToken {
  id: string;
  object: "personal_access_token";
  name: string;
  prefix: string;
  abilities: string[];
  expires_at: string | null;
  last_used_at: string | null;
  last_used_ip: string | null;
  revoked_at: string | null;
  member: { id: string; name: string };
  created_at: string;
}

export interface MintedToken extends PersonalAccessToken {
  plain_text_token: string;
}

export interface TokenAbility {
  ability: string;
  permissions: string[];
  allowed: boolean;
}

export interface NewToken {
  name: string;
  abilities: string[];
  expires_at: string | null;
}

/*
 * allAbilities is the ability that grants every group the person's role
 * reaches, now and as groups are added.
 */
export const allAbilities = "*";

export function tokensQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "tokens"],
    queryFn: () => request<PersonalAccessToken[]>("GET", "/tokens?limit=100", undefined, { workspace }),
  });
}

export function tokenAbilitiesQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "token-abilities"],
    queryFn: () => request<TokenAbility[]>("GET", "/tokens/abilities", undefined, { workspace }),
    staleTime: Number.POSITIVE_INFINITY,
  });
}

export function createToken(workspace: string, token: NewToken): Promise<MintedToken> {
  return request<MintedToken>("POST", "/tokens", token, { workspace });
}

export function revokeToken(workspace: string, id: string): Promise<void> {
  return request<void>("DELETE", `/tokens/${encodeURIComponent(id)}`, undefined, { workspace });
}
