/*
 * Following Operator's tries live. When Operator tries a workflow or a
 * series it built, the server sends each moment to the teammate: the
 * try starting, each line of the chat and each step as they happen, and
 * the result. A page showing that workflow or series keeps the tries of
 * this visit and calls onStart when one begins, so it can open its
 * Simulations panel and the teammate watches Operator work. Nothing is
 * kept: the tries are gone once the page is left.
 */
import { useQuery } from "@tanstack/react-query";
import { useRef, useState } from "react";

import type { SeriesSimulation } from "@/lib/api/workflow-simulations";
import { callerQuery } from "@/lib/api/workspace";
import type { Envelope } from "@/lib/realtime/bridge";
import { channels } from "@/lib/realtime/keys";
import { useChannel } from "@/lib/realtime/use-channel";

import { type LiveWfRun, movedOn, nothingYet, type WfRunEvent } from "./use-workflow-live-runs";

/* OperatorTry is one try of Operator's, as it happens and once done. */
export interface OperatorTry {
  id: string;
  kind: "workflow" | "series";
  /* A workflow try: the customer's first message, their situation, and the chat as it plays. */
  message?: string;
  situation?: string;
  run: LiveWfRun;
  /* A series try: how the person behaves, who they are, and the walk once done. */
  scenario?: string;
  name?: string;
  walk?: SeriesSimulation;
  failed?: string;
  done: boolean;
}

interface TryEvent extends WfRunEvent {
  kind?: "workflow" | "series";
  try_id?: string;
  workflow_id?: string;
  series_id?: string;
  message?: string;
  situation?: string;
  scenario?: string;
  name?: string;
  error?: string;
}

/* next is a try once one more moment has happened. */
function next(was: OperatorTry | undefined, data: TryEvent): OperatorTry {
  const now: OperatorTry = was ?? {
    id: data.try_id ?? "",
    kind: data.kind ?? "workflow",
    message: data.message,
    situation: data.situation,
    scenario: data.scenario,
    name: data.name,
    run: nothingYet,
    done: false,
  };
  if (data.moment === "failed") {
    return { ...now, failed: data.error ?? "The try did not finish.", done: true };
  }
  if (now.kind === "series") {
    return data.moment === "finished" ? { ...now, walk: data.result as SeriesSimulation, done: true } : now;
  }

  return { ...now, run: movedOn(now.run, data), done: data.moment === "finished" };
}

/*
 * useOperatorTries keeps Operator's tries of one workflow or series, in
 * the order they started, and calls onStart as each begins.
 */
export function useOperatorTries(
  workspace: string,
  target: { kind: "workflow" | "series"; id: string },
  onStart: () => void,
): OperatorTry[] {
  const [tries, setTries] = useState<OperatorTry[]>([]);
  const starting = useRef(onStart);
  starting.current = onStart;
  const { data: caller } = useQuery(callerQuery(workspace));
  const member = caller?.member?.id ?? null;

  useChannel(member ? channels.member(member) : null, {
    onEnvelope: (envelope: Envelope) => {
      const data = (envelope.data ?? {}) as TryEvent;
      const about = target.kind === "workflow" ? data.workflow_id : data.series_id;
      if (envelope.event !== "operator.simulation" || about !== target.id || !data.try_id) {
        return;
      }
      if (data.moment === "started") {
        starting.current();
      }
      setTries((now) => {
        const at = now.findIndex((t) => t.id === data.try_id);
        if (at < 0) {
          return [...now, next(undefined, data)];
        }

        return now.map((t, i) => (i === at ? next(t, data) : t));
      });
    },
  });

  return tries;
}
