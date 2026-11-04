/*
 * Walking the list from an open conversation: J and K open the next and
 * previous rows of the list on screen, and both neighbours are fetched
 * ahead, conversation and thread, so the next one opens without a
 * spinner. Closing or snoozing moves on to the next row by itself.
 */
import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useEffect, useMemo } from "react";

import { type Conversation, conversationQuery, messagesQuery } from "@/lib/api/inbox";
import { realtimeKeys } from "@/lib/realtime/keys";

import { useConversationMoves } from "./actions/action-context";
import type { CaseControls } from "./case";

/*
 * neighbours returns the rows before and after the conversation in the
 * list it sits in.
 */
export function neighbours(rows: { id: string }[] | undefined, id: string) {
  const at = rows?.findIndex((r) => r.id === id) ?? -1;
  if (!rows || at < 0) {
    return { previous: undefined, next: undefined };
  }

  return { previous: rows[at - 1]?.id, next: rows[at + 1]?.id };
}

/*
 * useListRows returns the rows of the list on screen that holds the
 * conversation.
 */
function useListRows(workspace: string, id: string) {
  const client = useQueryClient();
  const lists = client.getQueriesData<Conversation[]>({
    queryKey: realtimeKeys.conversations(workspace),
    type: "active",
  });

  return lists.map(([, rows]) => rows).find((rows) => rows?.some((r) => r.id === id));
}

export function useWalk(view: string, workspace: string, conversation: Conversation, caze: CaseControls) {
  const client = useQueryClient();
  const navigate = useNavigate();
  const rows = useListRows(workspace, conversation.id);
  const { previous, next } = neighbours(rows, conversation.id);

  useEffect(() => {
    for (const id of [next, previous]) {
      if (id) {
        void client.prefetchQuery(conversationQuery(workspace, id));
        void client.prefetchQuery(messagesQuery(workspace, id));
      }
    }
  }, [client, workspace, next, previous]);

  const moves = useMemo(() => {
    const open = (id: string | undefined) => {
      if (id) {
        void navigate({ to: "/d/inbox/$view/$conversation", params: { view, conversation: id } });
      }
    };
    const thenNext = (move: () => void) => () => {
      move();
      open(next ?? previous);
    };

    return {
      id: conversation.id,
      status: conversation.status,
      close: thenNext(() => caze.setStatus("closed")),
      reopen: () => caze.setStatus("open"),
      snooze: thenNext(() => caze.setStatus("snoozed")),
      assignToMe: caze.assignToMe,
      next: () => open(next),
      previous: () => open(previous),
    };
  }, [caze, conversation.id, conversation.status, navigate, next, previous, view]);

  useConversationMoves(moves);
}
