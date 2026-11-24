/*
 * Watching workflow simulations run. A run plays a customer through the
 * workflow, which takes a while, so the browser gives it a stream id
 * and listens on the teammate's own channel: each line of the chat as
 * it is written, each step the workflow takes, dimmi answering inside
 * agent steps, the checks being judged, then the verdict. Nothing here
 * is kept: once the run's request returns, the saved result takes over.
 */
import { useQuery } from "@tanstack/react-query";
import { useRef, useState } from "react";

import type { ChatLine, DryStep, WorkflowSimResult } from "@/lib/api/workflow-simulations";
import { callerQuery } from "@/lib/api/workspace";
import type { Envelope } from "@/lib/realtime/bridge";
import { channels } from "@/lib/realtime/keys";
import { useChannel } from "@/lib/realtime/use-channel";

/* LiveWfRun is one workflow simulation's run as it happens. */
export interface LiveWfRun {
  lines: ChatLine[];
  steps: DryStep[];
  working: boolean;
  judging: boolean;
  result?: WorkflowSimResult;
}

interface WfRunEvent {
  stream_id?: string;
  simulation_id?: string;
  moment?: "started" | "line" | "step" | "turn_started" | "turn_finished" | "judging" | "finished";
  line?: ChatLine;
  step?: DryStep;
  result?: unknown;
}

const nothingYet: LiveWfRun = { lines: [], steps: [], working: false, judging: false };

function movedOn(run: LiveWfRun, data: WfRunEvent): LiveWfRun {
  switch (data.moment) {
    case "started":
      return nothingYet;
    case "line":
      return data.line ? { ...run, lines: [...run.lines, data.line], working: false } : run;
    case "step":
      return data.step ? { ...run, steps: [...run.steps, data.step] } : run;
    case "turn_started":
      return { ...run, working: true };
    case "turn_finished":
      return { ...run, working: false };
    case "judging":
      return { ...run, judging: true };
    case "finished":
      return { ...run, judging: false, working: false, result: data.result as WorkflowSimResult };
    default:
      return run;
  }
}

/*
 * useWorkflowLiveRuns listens for the workflow simulation runs this
 * browser started, on the teammate's own channel. Begin gives the
 * stream id to send with a run; end forgets the runs once the saved
 * results are on screen.
 */
export function useWorkflowLiveRuns(workspace: string) {
  const stream = useRef("");
  const [runs, setRuns] = useState<Record<string, LiveWfRun>>({});
  const { data: caller } = useQuery(callerQuery(workspace));
  const member = caller?.member?.id ?? null;

  useChannel(member ? channels.member(member) : null, {
    onEnvelope: (envelope: Envelope) => {
      const data = (envelope.data ?? {}) as WfRunEvent;
      const id = data.stream_id ?? "";
      if (stream.current === "" || id !== stream.current || envelope.event !== "procedure.simulation.run") {
        return;
      }
      const sim = data.simulation_id;
      if (sim) {
        setRuns((now) => ({ ...now, [sim]: movedOn(now[sim] ?? nothingYet, data) }));
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
