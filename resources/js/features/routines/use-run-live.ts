/*
 * useRoutineWatch follows a routine's channel while its page is open:
 * each run that starts, finishes or is skipped reads the runs again, and
 * the run being watched hears its text and steps as Operator works.
 *
 * useRunLive is a running run as the thread shows it: the steps it kept
 * before the page opened, then each new one and its text as they arrive.
 */
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import type { OperatorStep } from "@/lib/api/operator";
import { operatorKeys } from "@/lib/api/operator";
import { type RoutineRun, routineKeys } from "@/lib/api/routines";
import type { Envelope } from "@/lib/realtime/bridge";
import { channels } from "@/lib/realtime/keys";
import { useChannel } from "@/lib/realtime/use-channel";

import type { Live } from "../operator/use-operator-chat";

interface Streamed {
  run_id?: string;
  thread_id?: string;
  status?: string;
  text?: string;
  step?: OperatorStep;
}

export function useRoutineWatch(workspace: string, routine: string, onEnvelope?: (e: Envelope) => void) {
  const client = useQueryClient();
  useChannel(routine ? channels.operatorRoutine(routine) : null, {
    onEnvelope: (envelope) => {
      if (envelope.event === "operator.routine_run.changed") {
        void client.invalidateQueries({ queryKey: routineKeys.runs(workspace, routine) });
        void client.invalidateQueries({ queryKey: routineKeys.one(workspace, routine) });
        const thread = (envelope.data as Streamed | undefined)?.thread_id;
        if (thread) {
          void client.invalidateQueries({ queryKey: operatorKeys.thread(workspace, thread) });
        }
      }
      onEnvelope?.(envelope);
    },
  });
}

export function useRunLive(workspace: string, run: RoutineRun | undefined): Live | null {
  const [streamed, setStreamed] = useState<{ run: string; steps: OperatorStep[]; text: string } | null>(null);
  const running = run?.status === "running";
  const mine = run && streamed?.run === run.id ? streamed : null;

  useRoutineWatch(workspace, run?.routine_id ?? "", (envelope) => {
    const data = (envelope.data ?? {}) as Streamed;
    if (!run?.thread_id || data.thread_id !== run.thread_id) {
      return;
    }
    const base = (current: typeof streamed) =>
      current?.run === run.id ? current : { run: run.id, steps: run.steps, text: "" };
    if (envelope.event === "operator.step" && data.step) {
      const step = data.step;
      setStreamed((current) => {
        const at = base(current);
        return { ...at, steps: [...at.steps, step] };
      });
    } else if (envelope.event === "operator.delta") {
      setStreamed((current) => {
        const at = base(current);
        return { ...at, text: at.text + (data.text ?? "") };
      });
    }
  });

  if (!run || !running) {
    return null;
  }
  const steps = mine && mine.steps.length >= run.steps.length ? mine.steps : run.steps;
  return { question: null, text: mine?.text ?? "", steps };
}
