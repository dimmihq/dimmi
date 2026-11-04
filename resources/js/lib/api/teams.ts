/*
 * The teams of the workspace on screen: the queries that read them and
 * the calls that change them. Every query key starts with the
 * workspace's, so a change to members or teams refreshes both.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";
import type { Member } from "./workspace";

export interface Team {
  id: string;
  name: string;
  member_count: number;
  members?: Member[];
  created_at: string;
  updated_at: string;
}

/*
 * TeamMemberResult is what adding one member came to: added, already in
 * the team, skipped because deactivated, or not found.
 */
export interface TeamMemberResult {
  member_id: string;
  result: "added" | "already_member" | "inactive" | "not_found";
}

export function teamsQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "teams"],
    queryFn: () => request<Team[]>("GET", "/teams?limit=100", undefined, { workspace }),
  });
}

export function teamQuery(workspace: string, team: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "teams", team],
    queryFn: () => request<Team>("GET", `/teams/${encodeURIComponent(team)}?include=members`, undefined, { workspace }),
    retry: false,
  });
}

export function createTeam(workspace: string, name: string): Promise<Team> {
  return request<Team>("POST", "/teams", { name }, { workspace });
}

export function renameTeam(workspace: string, team: string, name: string): Promise<Team> {
  return request<Team>("PATCH", `/teams/${encodeURIComponent(team)}`, { name }, { workspace });
}

/*
 * ConversationCounts are how many conversations a team holds, by status.
 */
export interface ConversationCounts {
  open: number;
  snoozed: number;
  closed: number;
}

export function teamCountsQuery(workspace: string, team: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "teams", team, "counts"],
    queryFn: () =>
      request<Team & { conversation_counts: ConversationCounts }>(
        "GET",
        `/teams/${encodeURIComponent(team)}?include=conversation_counts`,
        undefined,
        { workspace },
      ).then((t) => t.conversation_counts),
  });
}

/*
 * ReassignTo is where a deleted team's conversations go: another team,
 * a teammate, or nowhere.
 */
export type ReassignTo = { team: string } | { member: string } | null;

export function deleteTeam(workspace: string, team: string, to: ReassignTo = null): Promise<void> {
  const query = new URLSearchParams();
  if (to && "team" in to) {
    query.set("reassign_to_team_id", to.team);
  }
  if (to && "member" in to) {
    query.set("reassign_to_member_id", to.member);
  }
  const suffix = query.size > 0 ? `?${query}` : "";

  return request<void>("DELETE", `/teams/${encodeURIComponent(team)}${suffix}`, undefined, { workspace });
}

export function addTeamMembers(workspace: string, team: string, members: string[]): Promise<TeamMemberResult[]> {
  return request<TeamMemberResult[]>(
    "POST",
    `/teams/${encodeURIComponent(team)}/members`,
    { member_ids: members },
    { workspace },
  );
}

export function removeTeamMember(workspace: string, team: string, member: string): Promise<void> {
  return request<void>(
    "DELETE",
    `/teams/${encodeURIComponent(team)}/members/${encodeURIComponent(member)}`,
    undefined,
    { workspace },
  );
}
