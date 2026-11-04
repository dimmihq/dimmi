/*
 * The moves a teammate makes on a case: who holds it, its status and
 * its priority. Each goes to the server, which records it in the
 * thread as an event so the history says who did what; the open
 * conversation, its timeline, the lists and the sidebar's counts are
 * then read again. Each change carries an id, so its realtime echo is
 * not applied a second time.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";

import {
  type Conversation,
  type ConversationChanges,
  type ConversationStatus,
  conversationQuery,
  updateConversation,
} from "@/lib/api/inbox";
import { useMutationId } from "@/lib/realtime/context";

export const statusLabel: Record<ConversationStatus, string> = { open: "Open", snoozed: "Snoozed", closed: "Done" };

export type Priority = "normal" | "priority";

export const priorityLabel: Record<Priority, string> = { normal: "Normal", priority: "Priority" };

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

  return {
    pending: change.isPending,
    error: change.error,
    assign: (memberId: string | null) => change.mutate({ assignee_id: memberId }),
    setStatus: (status: ConversationStatus) =>
      change.mutate(status === "snoozed" ? { status, snooze_preset: "tomorrow" } : { status }),
    setPriority: (priority: Priority) => change.mutate({ priority: priority === "priority" }),
    addTag: (name: string) => change.mutate({ add_tags: [name] }),
    removeTag: (id: string) => change.mutate({ remove_tags: [id] }),
  };
}

export type CaseControls = ReturnType<typeof useCase>;
