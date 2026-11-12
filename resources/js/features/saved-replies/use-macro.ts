/*
 * A saved reply picked in the composer: its text, filled in for the
 * conversation's customer, goes into the draft at once, and its macro
 * actions wait beside the draft until the message is sent. Then they
 * apply through the conversation's own update, the way a teammate
 * changing it by hand would, so the server checks each one. Applying on
 * send rather than on insert means a teammate who changes their mind
 * just removes the queued macro, or the text, and nothing happened.
 */
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { conversationQuery, updateConversation } from "@/lib/api/inbox";
import type { SavedReply } from "@/lib/api/saved-replies";

import { hasActions, macroChanges } from "./macro";
import { renderReply } from "./render";

export function useMacro(workspace: string, conversation: string, sender: string) {
  const client = useQueryClient();
  const { data } = useQuery({ ...conversationQuery(workspace, conversation), enabled: workspace !== "" });
  const [queued, setQueued] = useState<SavedReply | null>(null);
  const [failure, setFailure] = useState<string | null>(null);

  /*
   * take returns the reply's text for this conversation and queues its
   * actions, when it has any.
   */
  const take = (reply: SavedReply): string => {
    setFailure(null);
    if (hasActions(reply.actions)) {
      setQueued(reply);
    }

    return renderReply(reply.body, { contact: data?.contact, sender });
  };

  /*
   * apply runs the queued actions once the message went; a send that
   * itself closed or snoozed keeps that status.
   */
  const apply = async (then?: "close" | "snooze") => {
    const reply = queued;
    if (!reply) {
      return;
    }
    setQueued(null);
    const changes = macroChanges(reply.actions, then !== undefined);
    if (!changes) {
      return;
    }
    try {
      await updateConversation({ workspace, mutation: crypto.randomUUID() }, conversation, changes);
      await client.invalidateQueries({ queryKey: conversationQuery(workspace, conversation).queryKey });
    } catch (error) {
      setFailure(`${reply.title} was sent, but its actions failed: ${error instanceof Error ? error.message : ""}`);
    }
  };

  return { queued, failure, take, apply, clear: () => setQueued(null) };
}

export type Macro = ReturnType<typeof useMacro>;
