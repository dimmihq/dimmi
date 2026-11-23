/*
 * How a simulation has gone lately. An agent's answers vary from run to
 * run, so one pass proves little: the dots show its latest runs on the
 * agent as it stands now, newest first, and Past runs lists every run
 * kept, including those made before the agent or the simulation
 * changed, which no longer count. Choosing a past run shows its chat
 * and checks in place of the latest, and the highlight slides to it.
 */
import { useQuery } from "@tanstack/react-query";
import { Glide } from "@/features/inbox/motion";
import {
  type AgentSimulationHistory,
  type AgentSimulationRun,
  type AnswerAs,
  agentSimulationRunsQuery,
} from "@/lib/api/agents";
import { formatRelative } from "@/lib/i18n/format";
import { cx } from "@/utils/cx";

import { card, fieldLabel, hint } from "../automation/procedure-simulator/simulator";

/* Dots are the runs a score counts, newest first, with room left for the ones still to come. */
export function Dots({ history }: { history: AgentSimulationHistory }) {
  const empty = Math.max(history.window - history.runs.length, 0);

  return (
    <span
      role="img"
      aria-label={`Passed ${history.passed} of the last ${history.total} runs`}
      className="flex shrink-0 items-center gap-1"
    >
      {history.runs.map((run) => (
        <span
          key={run.id}
          className={cx("size-1.5 rounded-full", run.passed ? "bg-status-lime-text" : "bg-status-rose-text")}
        />
      ))}
      {Array.from({ length: empty }, (_, i) => `empty-${i}`).map((key) => (
        <span key={key} className="size-1.5 rounded-full bg-foreground-icon-quaternary/40" />
      ))}
    </span>
  );
}

function outcome(run: AgentSimulationRun): { label: string; dot: string } {
  if (run.errored) {
    return { label: "Did not finish", dot: "bg-foreground-icon-quaternary" };
  }

  return run.passed ? { label: "Passed", dot: "bg-status-lime-text" } : { label: "Failed", dot: "bg-status-rose-text" };
}

export function PastRuns({
  workspace,
  agent,
  simulation,
  as,
  viewing,
  onView,
}: {
  workspace: string;
  agent: string;
  simulation: string;
  as: AnswerAs;
  viewing: string | null;
  onView: (run: AgentSimulationRun | null, at: number) => void;
}) {
  const { data: runs = [] } = useQuery(agentSimulationRunsQuery(workspace, agent, simulation, as));
  if (runs.length < 2) {
    return null;
  }

  return (
    <div className="flex flex-col gap-2.5">
      <span className={fieldLabel}>Past runs</span>
      <ul className={cx("flex flex-col gap-0.5 p-1", card)}>
        {runs.map((run, i) => {
          const { label, dot } = outcome(run);
          const open = viewing === run.id || (viewing === null && i === 0);

          return (
            <li key={run.id}>
              <button
                type="button"
                aria-pressed={open}
                onClick={() => onView(i === 0 ? null : run, i)}
                className="relative flex w-full cursor-pointer items-center gap-2.5 rounded-xl px-2.5 py-1.5 text-start outline-none transition-colors duration-150 hover:bg-background-primary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
              >
                {open ? (
                  <Glide
                    id={`past-runs-${simulation}-${as}`}
                    className="absolute inset-0 rounded-xl bg-background-primary-hover"
                  />
                ) : null}
                <span aria-hidden className={cx("relative size-1.5 shrink-0 rounded-full", dot)} />
                <span className="relative text-body-2-regular text-text-primary">{label}</span>
                <span className={cx(hint, "relative min-w-0 flex-1 truncate")}>
                  {formatRelative(run.ran_at)}
                  {i === 0 ? " · latest" : ""}
                </span>
                {run.counted || run.errored ? null : (
                  <span className={cx(hint, "relative")}>Before the last change</span>
                )}
              </button>
            </li>
          );
        })}
      </ul>
      <p className={hint}>
        Runs made before the agent, its procedures or this simulation changed are kept here, and no longer count.
      </p>
    </div>
  );
}
