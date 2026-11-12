/*
 * The moves a teammate makes on a case: who holds it, the team it sits
 * with, its status and its priority. Each goes to the server, which records it in the
 * thread as an event so the history says who did what; the open
 * conversation, its timeline, the lists and the sidebar's counts are
 * then read again. Each change carries an id, so its realtime echo is
 * not applied a second time. Moving a conversation to the trash offers
 * an undo.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";

import { toast } from "@/components/base/toast/toast";

import {
  type Conversation,
  type ConversationChanges,
  type ConversationPriority,
  type ConversationStatus,
  conversationQuery,
  restoreConversation,
  trashConversation,
  updateConversation,
} from "@/lib/api/inbox";
import { useMutationId } from "@/lib/realtime/context";

export const statusLabel: Record<ConversationStatus, string> = { open: "Open", snoozed: "Snoozed", closed: "Done" };

/*
 * useCase returns the case's controls for the conversation, and the
 * last change the server refused.
 */
export function useCase(workspace: string, conversation: Conversation) {
  const queryClient = useQueryClient();
  const mutationId = useMutationId();
  const change = useMutation({
    mutationFn: (changes: ConversationChanges) =>
      updateConversation({ workspace, mutation: mutationId() }, conversation.id, changes),
    onSuccess: (updated) => {
      queryClient.setQueryData(conversationQuery(workspace, conversation.id).queryKey, updated);

      return queryClient.invalidateQueries({ queryKey: ["workspace", workspace] });
    },
  });

  const refresh = () => queryClient.invalidateQueries({ queryKey: ["workspace", workspace] });
  /*
   * undoTrash puts the conversation back. It calls the server directly,
   * since the conversation's screen has usually moved on by then.
   */
  const undoTrash = async () => {
    try {
      await restoreConversation({ workspace, mutation: mutationId() }, conversation.id);
      toast.success("Conversation restored");
      await refresh();
    } catch {
      toast.error("The conversation could not be restored.");
    }
  };
  const trash = useMutation({
    mutationFn: () => trashConversation({ workspace, mutation: mutationId() }, conversation.id),
    onSuccess: () => {
      toast.success("Moved to the trash", {
        actions: [{ label: "Undo", onClick: () => void undoTrash() }],
        duration: 8000,
      });
      return refresh();
    },
    onError: () => toast.error("The conversation could not be moved to the trash."),
  });

  return {
    pending: change.isPending || trash.isPending,
    error: change.error,
    assign: (memberId: string | null) => change.mutate({ assignee_id: memberId }),
    assignTeam: (teamId: string | null) => change.mutate({ team_id: teamId }),
    assignToMe: () => change.mutate({ assignee_id: "me" }),
    snoozeWith: (option: string) => change.mutate({ status: "snoozed", snooze_option: option }),
    snoozeUntil: (until: string) => change.mutate({ status: "snoozed", snoozed_until: until }),
    setStatus: (status: ConversationStatus) =>
      change.mutate(status === "snoozed" ? { status, snooze_preset: "tomorrow" } : { status }),
    setPriority: (priority: ConversationPriority) => change.mutate({ priority }),
    addTag: (name: string) => change.mutate({ add_tags: [name] }),
    removeTag: (id: string) => change.mutate({ remove_tags: [id] }),
    trash: () => trash.mutate(),
  };
}

export type CaseControls = ReturnType<typeof useCase>;
