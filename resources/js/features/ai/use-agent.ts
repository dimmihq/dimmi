/*
 * The agent as the AI pages read it, and what to say when the server
 * refuses a change to it, such as a teammate without the right to
 * change the agent. Who may change the agent and its connections
 * follows the permissions the server gives.
 */
import { useQuery } from "@tanstack/react-query";

import { usePermission } from "@/features/settings/inboxes";
import { useAccount } from "@/features/settings/use-account";
import { agentQuery } from "@/lib/api/ai";
import { ApiError } from "@/lib/api/client";

/*
 * useAgent loads the agent view, and says whether the reader may change
 * the agent and manage its connections.
 */
export function useAgent() {
  const workspace = useAccount().workspace.id;
  const view = useQuery({ ...agentQuery(workspace), enabled: workspace !== "" });
  const canWrite = usePermission(workspace, "ai_agent:manage");
  const canKeys = usePermission(workspace, "ai:keys");

  return { workspace, view, canWrite, canKeys };
}

/*
 * refusal is what to say when a save fails.
 */
export function refusal(error: unknown, fallback: string): string {
  if (error instanceof ApiError && error.status === 403) {
    return "Only admins can change this.";
  }
  if (error instanceof ApiError) {
    return Object.values(error.fields)[0] ?? error.message;
  }

  return fallback;
}
