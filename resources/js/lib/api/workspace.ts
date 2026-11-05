/*
 * The workspace a page works in and its members: the queries that read
 * them and the calls that change them. Every call names the workspace
 * in the Dimmi-Workspace header, so a person in several workspaces
 * always changes the one on screen.
 */
import { queryOptions } from "@tanstack/react-query";

import type { LinkSafety } from "@/features/inbox/links/link-checks";

import { request, requestFile } from "./client";

export interface Workspace {
  id: string;
  name: string;
  slug: string;
  timezone: string;
  locale: string;
  test_mode: boolean;
  require_two_factor: boolean;
  link_safety?: LinkSafety;
  retention?: Retention;
  seen_receipt_shows_name: boolean;
  customer_facing_name: string | null;
  onboarding: WorkspaceOnboarding;
  deleted_at: string | null;
  purge_after: string | null;
}

/*
 * WorkspaceOnboarding is how far a workspace's setup has come: the step
 * its owners reached, and when they finished, null until then.
 */
export interface WorkspaceOnboarding {
  step: string;
  completed_at: string | null;
}

export interface Member {
  id: string;
  role: "owner" | "admin" | "member";
  status: "active" | "deactivated";
  custom_role?: { id: string; name: string } | null;
  user: { id: string; name: string; email: string; email_verified: boolean; avatar_url: string | null };
  sign_in_locked: boolean | null;
  two_factor_enabled: boolean | null;
  last_active_at: string | null;
}

/*
 * MemberFilter narrows the member list: twoFactor keeps only members
 * with, or without, a second factor. Only those who manage members may
 * narrow by it.
 */
export interface MemberFilter {
  twoFactor?: boolean;
}

/*
 * memberParams writes the filter as the list's query string.
 */
function memberParams(filter: MemberFilter): URLSearchParams {
  const params = new URLSearchParams();
  if (filter.twoFactor !== undefined) {
    params.set("filter[two_factor]", String(filter.twoFactor));
  }

  return params;
}

/*
 * Caller is who the person is in the workspace: their member id and what
 * they may do there. Pages decide what to offer from the permissions,
 * never from the role.
 */
export interface Caller {
  member: {
    id: string;
    role: string;
    permissions: string[];
    conversation_access: ConversationAccess;
    conversation_access_unassigned: boolean;
    access_team_ids: string[];
    availability: "online" | "away";
    away_reassign: boolean;
  } | null;
}

export interface WorkspaceChanges {
  name?: string;
  slug?: string;
  timezone?: string;
  locale?: string;
  require_two_factor?: boolean;
  link_safety?: Partial<LinkSafety>;
  retention?: Partial<Retention>;
  seen_receipt_shows_name?: boolean;
  customer_facing_name?: string | null;
}

export interface MemberChanges {
  role?: Member["role"];
  status?: Member["status"];
  custom_role_id?: string | null;
}

export function workspaceQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace],
    queryFn: () => request<Workspace>("GET", "/workspace", undefined, { workspace }),
  });
}

/*
 * APILimits are requests a minute per token, per signed-in person and
 * per workspace's tokens together; zero means no limit.
 */
export interface APILimits {
  token_per_minute: number;
  user_per_minute: number;
  workspace_per_minute: number;
}

export function entitlementsQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "entitlements"],
    queryFn: () => request<{ api: APILimits }>("GET", "/workspace/entitlements", undefined, { workspace }),
  });
}

export function membersQuery(workspace: string, filter: MemberFilter = {}) {
  const params = memberParams(filter);
  params.set("limit", "100");

  return queryOptions({
    queryKey: ["workspace", workspace, "members", filter],
    queryFn: () => request<Member[]>("GET", `/members?${params.toString()}`, undefined, { workspace }),
  });
}

/*
 * exportMembers fetches the member list, narrowed by the filter, as a
 * CSV file.
 */
export function exportMembers(workspace: string, filter: MemberFilter) {
  const params = memberParams(filter);
  params.set("format", "csv");

  return requestFile(`/members?${params.toString()}`, "text/csv", { workspace });
}

/*
 * ConversationAccess is how much of the workspace's conversations a
 * member sees.
 */
export type ConversationAccess = "all" | "assigned" | "teams" | "all_except_teams";

export function callerQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "caller"],
    queryFn: () => request<Caller>("GET", "/me", undefined, { workspace }),
  });
}

/*
 * createWorkspace makes a workspace the caller owns, such as their first
 * one during onboarding.
 */
export function createWorkspace(name: string): Promise<Workspace> {
  return request<Workspace>("POST", "/workspaces", { name });
}

/*
 * updateOnboarding records the setup step the workspace reached, and
 * finishes setup when completed is set.
 */
export function updateOnboarding(workspace: string, step: string, completed = false): Promise<Workspace> {
  return request<Workspace>("PATCH", "/workspace/onboarding", { step, completed }, { workspace });
}

/*
 * createTestConversations starts the five made-up conversations a new
 * workspace practices on.
 */
export function createTestConversations(workspace: string): Promise<{ count: number }> {
  return request<{ count: number }>("POST", "/workspace/onboarding/test-conversations", undefined, { workspace });
}

export function updateWorkspace(workspace: string, changes: WorkspaceChanges): Promise<Workspace> {
  return request<Workspace>("PATCH", "/workspace", changes, { workspace });
}

/*
 * deleteWorkspace moves the workspace to the trash, where it can be
 * restored for a while. The person types the workspace's address as
 * confirmSlug, so a slip of the mouse never deletes one.
 */
export function deleteWorkspace(workspace: string, confirmSlug: string): Promise<Workspace> {
  return request<Workspace>("DELETE", "/workspace", { confirm_slug: confirmSlug }, { workspace });
}

/*
 * restoreWorkspace takes a deleted workspace out of the trash.
 */
export function restoreWorkspace(workspace: string): Promise<Workspace> {
  return request<Workspace>("PATCH", `/workspaces/${encodeURIComponent(workspace)}`, { trashed: false });
}

export function updateMember(workspace: string, member: string, changes: MemberChanges): Promise<Member> {
  return request<Member>("PATCH", `/members/${encodeURIComponent(member)}`, changes, { workspace });
}

/*
 * startTwoFactorReset mails the member a link that turns off the second
 * factor they lost.
 */
export function startTwoFactorReset(workspace: string, member: string): Promise<unknown> {
  return request<unknown>("POST", `/members/${encodeURIComponent(member)}/two-factor-resets`, {}, { workspace });
}

/*
 * clearSignInLock lifts the lock failed guesses set on the member's
 * account.
 */
export function clearSignInLock(workspace: string, member: string): Promise<void> {
  return request<void>("DELETE", `/members/${encodeURIComponent(member)}/sign-in-lock`, undefined, { workspace });
}

export function leaveWorkspace(membership: string): Promise<void> {
  return request<void>("DELETE", `/user/memberships/${encodeURIComponent(membership)}`);
}

export function confirmPassword(password: string): Promise<unknown> {
  return request<unknown>("POST", "/user/confirmation", { method: "password", password });
}

/*
 * MemberImport is the report of a member import: how many rows created,
 * changed, left alone or failed, and each row's own outcome.
 */
export interface MemberImport {
  dry_run: boolean;
  created: number;
  updated: number;
  unchanged: number;
  failed: number;
  rows: {
    row: number;
    email: string;
    result: "created" | "updated" | "unchanged" | "failed";
    changes: string[];
    error?: { code: string; message: string };
  }[];
}

/*
 * importMembers sends a member import file, or with dryRun only checks
 * what it would do.
 */
export function importMembers(workspace: string, file: File, dryRun: boolean): Promise<MemberImport> {
  const form = new FormData();
  form.append("file", file);
  form.append("dry_run", String(dryRun));

  return request<MemberImport>("POST", "/member-imports", form, { workspace });
}

export interface Retention {
  closed_conversation_days: number | null;
  spam_days: number;
  trash_days: number;
  raw_email_days: number;
  export_days: number;
}

export function previewRetention(workspace: string, days: number): Promise<{ count: number }> {
  return request<{ count: number }>("POST", "/retention-previews", { closed_conversation_days: days }, { workspace });
}
