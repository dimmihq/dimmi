/*
 * The roles of the workspace on screen: the built-in roles and the
 * workspace's custom ones, the permission catalog a custom role is
 * shaped from, and the calls that change roles and who holds them.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";
import type { Member } from "./workspace";

export interface Role {
  id: string | null;
  kind: "built_in" | "custom";
  name: string;
  description: string | null;
  base_role: "owner" | "admin" | "member";
  permissions: string[];
  member_count: number | null;
}

export interface Permission {
  key: string;
  group: string;
  label: string;
  description: string;
  assignable: boolean;
  base_roles: string[];
}

export interface RoleDraft {
  name: string;
  description: string;
  base_role: "admin" | "member";
  permissions: string[];
}

export function rolesQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "roles"],
    queryFn: () => request<Role[]>("GET", "/roles", undefined, { workspace }),
  });
}

export function permissionsQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "permissions"],
    queryFn: () => request<Permission[]>("GET", "/permissions", undefined, { workspace }),
    staleTime: Number.POSITIVE_INFINITY,
  });
}

export function createRole(workspace: string, draft: RoleDraft): Promise<Role> {
  return request<Role>("POST", "/roles", draft, { workspace });
}

export function updateRole(workspace: string, role: string, draft: Omit<RoleDraft, "base_role">): Promise<Role> {
  return request<Role>("PATCH", `/roles/${encodeURIComponent(role)}`, draft, { workspace });
}

export function deleteRole(workspace: string, role: string): Promise<void> {
  return request<void>("DELETE", `/roles/${encodeURIComponent(role)}`, undefined, { workspace });
}

export function assignRole(workspace: string, member: string, role: string | null): Promise<Member> {
  return request<Member>("PATCH", `/members/${encodeURIComponent(member)}`, { custom_role_id: role }, { workspace });
}
