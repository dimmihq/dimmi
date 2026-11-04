/*
 * The browsers and apps signed in to the person's account, and signing
 * them out: one at a time, or every one but the browser asking.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export interface Session {
  id: string;
  object: "session";
  kind: "browser" | "app";
  current: boolean;
  auth_method: string;
  auth_methods: string[];
  two_factor_used: boolean;
  authenticated_at: string;
  device_label: string | null;
  client: string | null;
  platform: string | null;
  app_version: string | null;
  ip: string | null;
  country: string | null;
  last_activity_at: string;
  created_at: string;
  expires_at: string;
}

export const sessionsQuery = queryOptions({
  queryKey: ["sessions"],
  queryFn: () => request<Session[]>("GET", "/sessions"),
});

export function endSession(id: string): Promise<void> {
  return request<void>("DELETE", `/sessions/${encodeURIComponent(id)}`);
}

export function endOtherSessions(): Promise<void> {
  return request<void>("DELETE", "/sessions");
}
