/*
 * Apps a person connects through OAuth, such as AI agents: the request an
 * app sends, the person's decision on it, and the apps they connected,
 * each of which they can disconnect. An app's request travels as its
 * original query, which the server checks again on every call.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export interface AppRequest {
  object: "oauth_authorization";
  client_id: string;
  client_name: string;
  self_registered: boolean;
  redirect_host: string;
  loopback: boolean;
  scopes: string[];
  workspaces: { id: string; name: string; role: string }[];
}

export interface ConnectedApp {
  id: string;
  object: "connected_app";
  client_id: string;
  name: string;
  workspace_id?: string;
  workspace_name: string;
  scopes: string[];
  created_at: string;
  last_used_at: string;
}

export const connectedAppsKey = ["user", "connected-apps"] as const;

export function appRequestQuery(query: string) {
  return queryOptions({
    queryKey: ["oauth", "authorization", query],
    queryFn: () => request<AppRequest>("GET", `/oauth/authorization?query=${encodeURIComponent(query)}`),
    retry: false,
    staleTime: Number.POSITIVE_INFINITY,
  });
}

export function decideAppRequest(
  query: string,
  decision: { approve: true; workspace: string; scopes: string[] } | { approve: false },
) {
  const body = decision.approve
    ? { query, decision: "approve", workspace_id: decision.workspace, scopes: decision.scopes }
    : { query, decision: "deny" };

  return request<{ redirect_to: string }>("POST", "/oauth/authorizations", body);
}

export const connectedAppsQuery = queryOptions({
  queryKey: connectedAppsKey,
  queryFn: () => request<ConnectedApp[]>("GET", "/user/connected-apps"),
});

export function disconnectApp(id: string) {
  return request<void>("DELETE", `/user/connected-apps/${encodeURIComponent(id)}`);
}
