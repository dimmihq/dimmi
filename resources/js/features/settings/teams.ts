/*
 * What the teams pages share: who may change teams, how a team's size
 * reads, and how a refused change is explained.
 */
import { useQuery } from "@tanstack/react-query";

import { ApiError } from "@/lib/api/client";
import { callerQuery } from "@/lib/api/workspace";

/*
 * failureText is what to tell the person about a failed change: the
 * server's message for the field it blamed, or its general message.
 */
export function failureText(error: Error | null): string {
  if (!error) {
    return "";
  }

  return error instanceof ApiError ? (Object.values(error.fields)[0] ?? error.message) : error.message;
}

/*
 * memberCount says how many people are in a team, in words.
 */
export function memberCount(count: number): string {
  return count === 1 ? "1 member" : `${count} members`;
}

/*
 * useCanManageMembers reports whether the server lets the person change
 * the workspace's members and teams.
 */
export function useCanManageMembers(workspace: string): boolean {
  const { data: caller } = useQuery(callerQuery(workspace));

  return caller?.member?.permissions.includes("members:write") ?? false;
}
