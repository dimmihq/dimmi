/*
 * The agent as the AI pages read and change it. A change shows at once
 * and is saved in the background, with a toast saying it was kept; when
 * the server refuses it, the page goes back to what was saved and the
 * toast says why, such as a teammate without the right to change the
 * agent. Who may change the agent and its connections follows the
 * permissions the server gives.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { toast } from "@/components/base/toast/toast";
import { usePermission } from "@/features/settings/inboxes";
import { useAccount } from "@/features/settings/use-account";
import { type AgentView, agentQuery, aiKey, updateAgent } from "@/lib/api/ai";
import { ApiError } from "@/lib/api/client";

import { type AgentChange, applyChange } from "./format";

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
 * refusal is what a toast says when a save fails.
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

/*
 * useAgentSave saves one change to the agent, showing it before the
 * server answers.
 */
export function useAgentSave(workspace: string) {
  const client = useQueryClient();
  const key = agentQuery(workspace).queryKey;

  const save = useMutation({
    mutationFn: (change: AgentChange) => updateAgent(workspace, change),
    onMutate: async (change) => {
      await client.cancelQueries({ queryKey: key });
      const before = client.getQueryData<AgentView>(key);
      if (before) {
        client.setQueryData<AgentView>(key, { ...before, agent: applyChange(before.agent, change) });
      }

      return { before };
    },
    onError: (error, _change, context) => {
      if (context?.before) {
        client.setQueryData(key, context.before);
      }
      toast.error("Not saved", { description: refusal(error, "Check your connection and try again.") });
    },
    onSuccess: () => {
      toast.success("Saved");
    },
    onSettled: () => client.invalidateQueries({ queryKey: aiKey(workspace) }),
  });

  return (change: AgentChange) => save.mutate(change);
}
