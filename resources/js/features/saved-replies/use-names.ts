/*
 * Naming what a macro points at: the workspace's teammates, teams and
 * tags by id, read from the lists the screen already keeps.
 */
import { useQuery } from "@tanstack/react-query";

import { tagsQuery } from "@/lib/api/tags";
import { teamsQuery } from "@/lib/api/teams";
import { membersQuery } from "@/lib/api/workspace";

import type { Names } from "./macro";

export function useNames(workspace: string, enabled = true): Names {
  const on = enabled && workspace !== "";
  const { data: members = [] } = useQuery({ ...membersQuery(workspace), enabled: on });
  const { data: teams = [] } = useQuery({ ...teamsQuery(workspace), enabled: on });
  const { data: tags = [] } = useQuery({ ...tagsQuery(workspace), enabled: on });
  const names = new Map<string, string>([
    ...members.map((member) => [member.id, member.user.name] as [string, string]),
    ...teams.map((team) => [team.id, team.name] as [string, string]),
    ...tags.map((tag) => [tag.id, tag.name] as [string, string]),
  ]);

  return (id) => names.get(id);
}
