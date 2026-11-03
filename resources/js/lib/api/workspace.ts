/*
 * The workspace a page works in and its members: the queries that read
 * them and the calls that change them. Every call names the workspace
 * in the Dimmi-Workspace header, so a person in several workspaces
 * always changes the one on screen.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export interface Workspace {
  id: string;
  name: string;
  slug: string;
  timezone: string;
  locale: string;
  test_mode: boolean;
}

export interface Member {
  id: string;
  role: "owner" | "admin" | "member";
  status: "active" | "deactivated";
  user: { id: string; name: string; email: string; email_verified: boolean };
}

/*
 * Caller is who the person is in the workspace: their member id and what
 * they may do there. Pages decide what to offer from the permissions,
 * never from the role.
 */
export interface Caller {
  member: { id: string; role: string; permissions: string[] } | null;
}

export interface WorkspaceChanges {
  name?: string;
  slug?: string;
  timezone?: string;
}

export interface MemberChanges {
  role?: Member["role"];
  status?: Member["status"];
}

export function workspaceQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace],
    queryFn: () => request<Workspace>("GET", "/workspace", undefined, { workspace }),
  });
}

export function membersQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "members"],
    queryFn: () => request<Member[]>("GET", "/members?limit=100", undefined, { workspace }),
  });
}

export function callerQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "caller"],
    queryFn: () => request<Caller>("GET", "/me", undefined, { workspace }),
  });
}

export function updateWorkspace(workspace: string, changes: WorkspaceChanges): Promise<Workspace> {
  return request<Workspace>("PATCH", "/workspace", changes, { workspace });
}

export function updateMember(workspace: string, member: string, changes: MemberChanges): Promise<Member> {
  return request<Member>("PATCH", `/members/${encodeURIComponent(member)}`, changes, { workspace });
}

export function leaveWorkspace(membership: string): Promise<void> {
  return request<void>("DELETE", `/user/memberships/${encodeURIComponent(membership)}`);
}

export function confirmPassword(password: string): Promise<unknown> {
  return request<unknown>("POST", "/user/confirmation", { method: "password", password });
}
