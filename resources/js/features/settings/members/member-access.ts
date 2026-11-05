/*
 * The words and the rules of a member's conversation access: what each
 * level lets them see, which levels name teams, and the change to send.
 * A member always keeps seeing the conversations assigned to them.
 */
import type { ConversationAccess, MemberChanges } from "@/lib/api/workspace";

/*
 * levels are the access levels in the order the dialog offers them.
 */
export const levels: ConversationAccess[] = ["all", "assigned", "teams", "all_except_teams"];

/*
 * levelLabel names a level in a few words.
 */
export function levelLabel(level: ConversationAccess): string {
  const labels: Record<ConversationAccess, string> = {
    all: "All conversations",
    assigned: "Only their own",
    teams: "Only some teams",
    all_except_teams: "All except some teams",
  };

  return labels[level];
}

/*
 * levelSentence says in one sentence what the member sees at a level.
 */
export function levelSentence(level: ConversationAccess): string {
  const sentences: Record<ConversationAccess, string> = {
    all: "They see every conversation in the workspace.",
    assigned: "They see only the conversations assigned to them.",
    teams: "They see the conversations of the teams you pick, and those assigned to them.",
    all_except_teams: "They see every conversation except those of the teams you pick, unless assigned to them.",
  };

  return sentences[level];
}

/*
 * needsTeams reports whether the level only makes sense with teams.
 */
export function needsTeams(level: ConversationAccess): boolean {
  return level === "teams" || level === "all_except_teams";
}

/*
 * AccessDraft is the access being edited in the dialog.
 */
export interface AccessDraft {
  level: ConversationAccess;
  unassigned: boolean;
  teams: string[];
}

/*
 * accessChanges is the change to send for the draft, or null while a
 * level that needs teams has none picked.
 */
export function accessChanges(draft: AccessDraft): MemberChanges | null {
  if (needsTeams(draft.level) && draft.teams.length === 0) {
    return null;
  }

  return {
    conversation_access: draft.level,
    conversation_access_unassigned: draft.level === "all" ? false : draft.unassigned,
    access_team_ids: needsTeams(draft.level) ? draft.teams : [],
  };
}
