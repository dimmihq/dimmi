/*
 * useHeld follows a reply waiting to leave: how many seconds are left and
 * the undo that takes it back, handing its text to onUndone.
 */
import { useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useState } from "react";

import { type Message, undoReply } from "@/lib/api/inbox";
import { useMutationId } from "@/lib/realtime/context";

export interface Held {
  message: Message | null;
  left: number;
  hold: (message: Message | undefined) => void;
  undo: () => void;
  failure: string | null;
}

/*
 * secondsLeft is how many whole seconds remain until the moment.
 */
export function secondsLeft(until: string, now: number): number {
  return Math.max(0, Math.ceil((Date.parse(until) - now) / 1000));
}

export function useHeld(workspace: string, conversation: string, onUndone: (body: string) => void): Held {
  const client = useQueryClient();
  const mutationId = useMutationId();
  const [message, setMessage] = useState<Message | null>(null);
  const [now, setNow] = useState(() => Date.now());
  const [failure, setFailure] = useState<string | null>(null);
  const until = message?.scheduled_send_at ?? null;
  const left = until ? secondsLeft(until, now) : 0;

  useEffect(() => {
    if (!until) {
      return;
    }
    const tick = window.setInterval(() => setNow(Date.now()), 250);

    return () => window.clearInterval(tick);
  }, [until]);

  useEffect(() => {
    if (message && left === 0) {
      setMessage(null);
    }
  }, [message, left]);

  const undo = useCallback(() => {
    if (!message) {
      return;
    }
    setMessage(null);
    undoReply({ workspace, mutation: mutationId() }, conversation, message.id)
      .then((body) => {
        onUndone(body);
        return client.invalidateQueries({ queryKey: ["workspace", workspace] });
      })
      .catch((error: unknown) => setFailure(error instanceof Error ? error.message : "Undo failed."));
  }, [client, conversation, message, mutationId, onUndone, workspace]);

  return {
    message,
    left,
    failure,
    undo,
    hold: (sent) => {
      setFailure(null);
      setNow(Date.now());
      setMessage(sent?.scheduled_send_at ? sent : null);
    },
  };
}
