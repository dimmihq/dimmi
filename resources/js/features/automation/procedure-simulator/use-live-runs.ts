/*
 * Watching saved simulations run. A run replays several messages
 * against the real agent, so the browser gives it a stream id and
 * listens on the teammate's own channel: each customer message as it
 * is sent, each thing dimmi does while it works, each reply as it
 * lands, the checks being judged, then the verdict. Nothing here is
 * kept: once the run's request returns, the saved result takes over.
 */
import { useQuery } from "@tanstack/react-query";
import { useRef, useState } from "react";

import type { OperatorStep } from "@/lib/api/operator";
import type { TestResult } from "@/lib/api/procedures";
import { callerQuery } from "@/lib/api/workspace";
import type { Envelope } from "@/lib/realtime/bridge";
import { channels } from "@/lib/realtime/keys";
import { useChannel } from "@/lib/realtime/use-channel";

import type { ChipSources } from "../procedure-writer/procedure-suggestions";
import { type LiveStep, liveStep } from "./simulator";

type RanTurn = TestResult["turns"][number];

/*
 * LiveTurn is one turn of a run as it happens: what the customer sent,
 * what dimmi has done so far, and its reply once it lands.
 */
export interface LiveTurn {
  message: string;
  steps: OperatorStep[];
  result?: RanTurn;
}

/*
 * LiveRun is one simulation's run as it happens. The result is its
 * verdict, set when the run is over.
 */
export interface LiveRun {
  turns: LiveTurn[];
  total: number;
  judging: boolean;
  result?: TestResult;
}

interface RunEvent {
  stream_id?: string;
  test_case_id?: string;
  moment?: "started" | "turn_started" | "turn_finished" | "judging" | "finished";
  turns?: number;
  turn?: number;
  message?: string;
  result?: unknown;
  step?: LiveStep;
}

type Runs = Record<string, LiveRun>;

const empty: LiveRun = { turns: [], total: 0, judging: false };

/*
 * withTurn returns the run with one of its turns changed.
 */
function withTurn(run: LiveRun, at: number, change: (turn: LiveTurn) => LiveTurn): LiveRun {
  const turns = [...run.turns];
  turns[at] = change(turns[at] ?? { message: "", steps: [] });

  return { ...run, turns };
}

/*
 * moved is a run after one of its moments.
 */
function moved(run: LiveRun, data: RunEvent): LiveRun {
  const at = data.turn ?? 0;
  switch (data.moment) {
    case "started":
      return { ...empty, total: data.turns ?? 0 };
    case "turn_started":
      return withTurn(run, at, () => ({ message: data.message ?? "", steps: [] }));
    case "turn_finished":
      return withTurn(run, at, (turn) => ({ ...turn, result: data.result as RanTurn }));
    case "judging":
      return { ...run, judging: true };
    case "finished":
      return { ...run, judging: false, result: data.result as TestResult };
    default:
      return run;
  }
}

/*
 * useLiveRuns listens for the runs this browser started. Begin gives
 * the stream id to send with a run and forgets the runs before it;
 * end forgets them once the saved results are on screen.
 */
export function useLiveRuns(workspace: string, sources: ChipSources) {
  const stream = useRef("");
  const [runs, setRuns] = useState<Runs>({});
  const { data: caller } = useQuery(callerQuery(workspace));
  const member = caller?.member?.id ?? null;

  useChannel(member ? channels.member(member) : null, {
    onEnvelope: (envelope: Envelope) => {
      const data = (envelope.data ?? {}) as RunEvent;
      const id = data.stream_id ?? "";
      if (stream.current === "" || !id.startsWith(stream.current)) {
        return;
      }
      if (envelope.event === "procedure.simulation.run" && data.test_case_id) {
        const test = data.test_case_id;
        setRuns((now) => ({ ...now, [test]: moved(now[test] ?? empty, data) }));
      }
      if (envelope.event === "procedure.preview.step" && data.step) {
        const [, test, turn] = id.split(":");
        const step = liveStep(data.step, sources);
        setRuns((now) => ({
          ...now,
          [test]: withTurn(now[test] ?? empty, Number(turn), (t) => ({ ...t, steps: [...t.steps, step] })),
        }));
      }
    },
  });

  const begin = () => {
    stream.current = crypto.randomUUID();
    setRuns({});

    return stream.current;
  };
  const end = () => {
    stream.current = "";
    setRuns({});
  };

  return { runs, begin, end };
}
