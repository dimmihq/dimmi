/*
 * useOperatorChat runs one thread with Operator. A question shows at
 * once; while Operator works, its text and steps arrive on the
 * teammate's own channel and grow in place, and once the request answers
 * the thread is read again with the stored answer and the changes it
 * proposed. Stop cancels the request, which stops Operator; what it had
 * written is kept. Another tab's answer refreshes the thread too.
 */
import { type QueryKey, useQuery, useQueryClient } from "@tanstack/react-query";
import { useRef, useState } from "react";

import { ApiError } from "@/lib/api/client";
import {
  askOperator,
  type ChangesetDecision,
  decideChangeset,
  type OperatorStep,
  type OperatorThreadView,
  operatorKeys,
} from "@/lib/api/operator";
import { callerQuery } from "@/lib/api/workspace";
import type { Envelope } from "@/lib/realtime/bridge";
import { channels } from "@/lib/realtime/keys";
import { useChannel } from "@/lib/realtime/use-channel";

/*
 * Live is a turn in progress: the question, null for a brief, and what
 * Operator has written and done so far.
 */
export interface Live {
  question: string | null;
  text: string;
  steps: OperatorStep[];
}

interface Streamed {
  thread_id?: string;
  text?: string;
  step?: OperatorStep;
}

/*
 * failureText says why a request failed, in words a teammate reads.
 */
export function failureText(error: unknown): string {
  if (error instanceof ApiError) {
    return error.message;
  }

  return "Operator could not answer just now. Try again in a moment.";
}

export function useOperatorChat(workspace: string, thread: OperatorThreadView | undefined, threadKey: QueryKey) {
  const client = useQueryClient();
  const { data: caller } = useQuery(callerQuery(workspace));
  const member = caller?.member?.id ?? null;
  const [live, setLive] = useState<Live | null>(null);
  const [failure, setFailure] = useState<string | null>(null);
  const running = useRef<AbortController | null>(null);
  const refresh = () =>
    Promise.all([
      client.invalidateQueries({ queryKey: threadKey }),
      client.invalidateQueries({ queryKey: operatorKeys.threads(workspace) }),
    ]);

  const onEnvelope = (envelope: Envelope) => {
    const data = (envelope.data ?? {}) as Streamed;
    if (!thread || data.thread_id !== thread.id) {
      return;
    }
    if (envelope.event === "operator.delta") {
      setLive((current) => current && { ...current, text: current.text + (data.text ?? "") });
    } else if (envelope.event === "operator.step" && data.step) {
      const step = data.step;
      setLive((current) => current && { ...current, steps: [...current.steps, step] });
    } else if (envelope.event === "operator.answered" && !running.current) {
      void refresh();
    }
  };
  useChannel(member ? channels.member(member) : null, { onEnvelope });

  const run = async (question: string | null, call: (signal: AbortSignal) => Promise<unknown>) => {
    const controller = new AbortController();
    running.current = controller;
    setFailure(null);
    setLive({ question, text: "", steps: [] });
    try {
      await call(controller.signal);
    } catch (error) {
      if (!controller.signal.aborted) {
        setFailure(failureText(error));
      }
    } finally {
      running.current = null;
      await refresh();
      setLive(null);
    }
  };

  const ask = (question: string) => {
    if (!thread || running.current) {
      return;
    }
    void run(question, (signal) => askOperator(workspace, thread.id, question, signal));
  };

  const decide = async (id: string, decision: ChangesetDecision) => {
    await decideChangeset(workspace, id, decision);
    await Promise.all([refresh(), client.invalidateQueries({ queryKey: ["workspace", workspace] })]);
  };

  return {
    live,
    failure,
    busy: live !== null,
    ask,
    run,
    decide,
    stop: () => running.current?.abort(),
  };
}

export type OperatorChat = ReturnType<typeof useOperatorChat>;
