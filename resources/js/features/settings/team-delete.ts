/*
 * What deleting a team reads like before it happens: how many
 * conversations it holds, and where they go once a choice and its
 * target are picked. Kept apart from the dialog so the wording and the
 * choice can be tested without rendering it.
 */
import type { ReassignTo } from "@/lib/api/teams";

/*
 * choices are where a deleted team's conversations may go, before the
 * target is picked.
 */
export const choices = { team: "team", member: "member", none: "none" } as const;

export type Choice = (typeof choices)[keyof typeof choices];

/*
 * countLine says what the team holds, in one sentence.
 */
export function countLine(counts: { open: number; snoozed: number; closed: number }): string {
  const total = counts.open + counts.snoozed + counts.closed;
  if (total === 0) {
    return "It holds no conversations.";
  }

  return `It holds ${counts.open} open, ${counts.snoozed} snoozed and ${counts.closed} closed conversations.`;
}

/*
 * target turns the choice and the picked id into where the conversations
 * go, or undefined while a needed pick is missing.
 */
export function target(choice: Choice | null, picked: string | null): ReassignTo | undefined {
  if (choice === "none") {
    return null;
  }
  if (choice === "team" && picked) {
    return { team: picked };
  }
  if (choice === "member" && picked) {
    return { member: picked };
  }

  return undefined;
}
