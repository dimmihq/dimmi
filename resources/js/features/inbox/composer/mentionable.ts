/*
 * Who a note may mention, as the at-sign suggestions list them:
 * teammates first, then teams. A teammate who cannot see the conversation
 * is still listed, with a warning, since the writer cannot change who
 * sees what.
 */
import type { Mentionables } from "@/lib/api/inbox";

export interface Mentionable {
  kind: "member" | "team";
  id: string;
  name: string;
  detail: string;
  warning: string | null;
}

export function toMentionables(found: Mentionables): Mentionable[] {
  return [
    ...found.members.map((member) => ({
      kind: "member" as const,
      id: member.id,
      name: member.name,
      detail: member.availability === "away" ? "Away" : "",
      warning: member.can_see ? null : "Can't see this conversation",
    })),
    ...found.teams.map((team) => ({
      kind: "team" as const,
      id: team.id,
      name: team.name,
      detail: team.member_count === 1 ? "1 member" : `${team.member_count} members`,
      warning: null,
    })),
  ];
}
