/*
 * What a workflow simulation shows, shared by the list and the compare
 * view: the chat as the customer met it (the workflow's messages with
 * their buttons, what the customer tapped or typed, a wait they let run
 * out, and dimmi's replies inside agent steps), the workflow's trace
 * (whether the trigger started it, then each step in a row with the path
 * it took, a called workflow's steps naming it), and the checks as a run
 * left them. What the workflow did to the
 * conversation, such as tags, notes and assignments, shows in the chat
 * the way the inbox shows it.
 * A run in progress streams its lines and steps as they happen.
 */
import { RiArrowRightSLine, RiCursorLine, RiTimerLine } from "@remixicon/react";
import { Fragment } from "react";

import { Chip } from "@/components/base/badges/chip";
import { MessageBody } from "@/features/inbox/message-body";
import type { ChatLine, DryStep, DryTrace, WorkflowAssertion, WorkflowSimResult } from "@/lib/api/workflow-simulations";
import { cx } from "@/utils/cx";

import { Mark } from "../../agents/agent-sim-compare";
import { judgedOf } from "../../agents/agent-sim-words";
import { KindMark } from "../parts";
import { Did } from "../procedure-simulator/parts";
import { fromCustomer, hint, words } from "../procedure-simulator/simulator";
import type { ChipSources } from "../procedure-writer/procedure-suggestions";
import { stepLook } from "../step-kinds";
import type { StepKind } from "../workflows";
import type { LiveWfRun } from "./use-workflow-live-runs";
import { Event } from "./workflow-sim-events";
import { moments, useNames } from "./workflow-sim-moments";
import { type CheckResult, checkSentence, lined, pathNamed, type Steps, stepNote } from "./workflow-sim-words";

/* CheckLine is one check with its mark and, after a run, why it failed or what the judge said. */
export function CheckLine({
  check,
  outcome,
  steps,
  sources,
  children,
}: {
  check: WorkflowAssertion;
  outcome?: CheckResult;
  steps: Steps;
  sources: ChipSources;
  children?: React.ReactNode;
}) {
  const said = judgedOf(outcome as Parameters<typeof judgedOf>[0]);

  return (
    <li className="flex items-start gap-2">
      <span className="flex h-7 items-center">
        <Mark result={outcome as Parameters<typeof Mark>[0]["result"]} />
      </span>
      <span className="flex min-w-0 flex-1 flex-col py-1">
        <span className="text-body-2-regular text-text-primary">{checkSentence(check, steps, sources)}</span>
        {said ? (
          <span className={cx("text-body-2-regular", said.failed ? "text-text-error-primary" : "text-text-secondary")}>
            {said.text}
          </span>
        ) : null}
      </span>
      {children}
    </li>
  );
}

/* WfCheckList is a simulation's checks as one run left them. */
export function WfCheckList({
  checks,
  result,
  steps,
  sources,
}: {
  checks: WorkflowAssertion[];
  result: WorkflowSimResult | null;
  steps: Steps;
  sources: ChipSources;
}) {
  if (checks.length === 0) {
    return <p className={hint}>This simulation has no checks.</p>;
  }
  const results = lined(checks, result);

  return (
    <ul className="flex flex-col gap-1">
      {checks.map((check, index) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: checks never reorder while shown.
        <CheckLine key={index} check={check} outcome={results[index]} steps={steps} sources={sources} />
      ))}
    </ul>
  );
}

/* Line is one line of the chat. */
function Line({
  line,
  turn,
  sources,
  bubble,
}: {
  line: ChatLine;
  turn?: WorkflowSimResult["turns"][number];
  sources: ChipSources;
  bubble: string;
}) {
  if (line.from === "customer") {
    if (line.silent) {
      return (
        <p className={cx(hint, "ms-auto flex items-center gap-1.5")}>
          <RiTimerLine aria-hidden className="size-4 text-foreground-icon-tertiary" />
          Said nothing, so the wait ran out
        </p>
      );
    }

    return (
      <div className="ms-auto flex max-w-[85%] flex-col items-end gap-1">
        <p className={cx(bubble, "max-w-full")}>{line.text}</p>
        {line.tapped ? (
          <span className={cx(hint, "flex items-center gap-1")}>
            <RiCursorLine aria-hidden className="size-3.5 text-foreground-icon-tertiary" />
            Tapped a button
          </span>
        ) : null}
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-2">
      {line.from === "agent" ? <span className={hint}>{line.name || "dimmi"}, inside the agent step</span> : null}
      <MessageBody text={line.text} className="text-body-regular text-text-primary" />
      {line.choices?.length ? (
        <span className="flex flex-wrap gap-1">
          {line.choices.map((choice) => (
            <Chip key={choice} variant="caption" color="neutral" className="py-0.5">
              {choice}
            </Chip>
          ))}
        </span>
      ) : null}
      {turn ? <Did calls={turn.calls} outcome={turn.outcome} sources={sources} /> : null}
    </div>
  );
}

/*
 * WfTalk is a simulation's chat as it ran, or the saved first message
 * when it has not run yet.
 */
export function WfTalk({
  messages,
  result,
  lines,
  sources,
  bubble = fromCustomer,
}: {
  messages: { text: string }[];
  result: WorkflowSimResult | null;
  lines?: ChatLine[];
  sources: ChipSources;
  bubble?: string;
}) {
  const shown = lines ?? result?.transcript ?? [];
  const names = useNames();
  if (shown.length === 0) {
    return (
      <div className="flex flex-col gap-3">
        {messages.map((m, index) => (
          // biome-ignore lint/suspicious/noArrayIndexKey: a simulation's messages never reorder.
          <p key={index} className={bubble}>
            {m.text}
          </p>
        ))}
      </div>
    );
  }
  let agentAt = 0;
  const shownMoments = lines ? moments(shown) : moments(shown, result?.trace);

  return (
    <ol aria-label="Simulated conversation" aria-live={lines ? "polite" : undefined} className="flex flex-col gap-5">
      {shownMoments.map((moment, at) => {
        if (moment.kind === "action") {
          return (
            <li key={`action-${moment.action.step_id}-${String(at)}`}>
              <Event action={moment.action} names={names} />
            </li>
          );
        }
        const { line, index } = moment;
        const turn = line.from === "agent" ? result?.turns[agentAt++] : undefined;

        return (
          <li key={`line-${String(index)}`}>
            <Line line={line} turn={turn} sources={sources} bubble={bubble} />
          </li>
        );
      })}
    </ol>
  );
}

const verdictWords: Record<string, string> = {
  trigger_settings: "the trigger's settings do not match",
  channel: "the customer writes on a channel it does not run on",
  audience: "the customer is not in its audience",
  schedule: "it is outside the office hours it runs in",
};

/*
 * plainOutcomes are the outcomes a step's chip leaves unsaid: the step
 * simply did its job.
 */
const plainOutcomes = new Set(["would_run", "sent", "matched", "button", "answered", "completed"]);

/*
 * StepChip is one step the run took, in a row that reads left to right:
 * its kind, the path it took, and its outcome when that is news, such as
 * a skip or a failure. A called workflow's steps say whose they are.
 */
function StepChip({ done, steps, opens }: { done: DryStep; steps: Steps; opens: boolean }) {
  const look = stepLook[done.kind as StepKind];
  const deep = (done.depth ?? 0) > 0;
  const took = done.path_id
    ? deep
      ? done.path_label
      : pathNamed(steps, done.step_id, done.path_id) || done.path_label
    : "";
  const note = stepNote(done);
  const outcome = done.skipped || !plainOutcomes.has(done.outcome) ? outcomeWord(done) : "";

  return (
    <li className="flex items-center gap-1.5">
      {opens ? <span className={hint}>Inside “{done.workflow}”:</span> : null}
      <span
        title={[outcomeWord(done), note].filter(Boolean).join(" · ")}
        className={cx(
          "flex items-center gap-1.5 rounded-full bg-background-primary-default py-0.5 ps-0.5 pe-2.5 shadow-card",
          done.skipped && "opacity-60",
        )}
      >
        {look ? <KindMark icon={look.icon} tint={look.tint} size="sm" /> : null}
        <span className="text-body-2-medium text-text-primary">{look?.name ?? words(done.kind)}</span>
        {took ? <span className="text-body-2-regular text-text-secondary">· {took}</span> : null}
        {outcome ? <span className="text-body-2-regular text-text-tertiary">· {outcome}</span> : null}
      </span>
    </li>
  );
}

/* outcomeWord is a step's outcome as its chip reads: an action the run did reads as done. */
function outcomeWord(done: DryStep): string {
  if (done.skipped && done.kind === "wait") {
    return "Skipped";
  }

  return !done.skipped && done.outcome === "would_run" ? "Done" : words(done.outcome);
}

/* Verdict says why the trigger would not start the workflow. */
function Verdict({ verdict }: { verdict: DryTrace["verdict"] }) {
  if (verdict.started) {
    return null;
  }
  const reason = verdict.reason ?? "";

  return (
    <p className="text-body-2-regular text-text-error-primary">
      The workflow would not start: {verdictWords[reason] ?? words(reason)}
      {verdict.failed_rule ? ` (${verdict.failed_rule})` : ""}.
    </p>
  );
}

/* Simulated says that what the workflow did stayed in the simulation. */
function Simulated({ actions }: { actions: DryTrace["actions"] }) {
  if (actions.length === 0) {
    return null;
  }

  return <p className={hint}>Simulated: no tag, assignment, ticket or request left the simulation.</p>;
}

/*
 * Trace is what the workflow did in a run: whether the trigger started
 * it, each step and path, the actions that would have run, and the
 * variables it set.
 */
export function Trace({ trace, steps, live }: { trace?: DryTrace; steps: Steps; live?: DryStep[] }) {
  const done = live ?? trace?.steps ?? [];
  if (!trace && !live) {
    return <p className={hint}>Run the simulation to see each step the workflow takes.</p>;
  }

  return (
    <div className="flex flex-col gap-3">
      {trace ? <Verdict verdict={trace.verdict} /> : null}
      {done.length > 0 ? (
        <ol aria-label="Steps the workflow took" className="flex flex-wrap items-center gap-x-1 gap-y-2">
          {done.map((step, index) => (
            <Fragment key={`${step.step_id}-${String(index)}`}>
              {index > 0 ? (
                <li aria-hidden className="text-foreground-icon-tertiary">
                  <RiArrowRightSLine className="size-4" />
                </li>
              ) : null}
              <StepChip
                done={step}
                steps={steps}
                opens={(step.depth ?? 0) > 0 && (done[index - 1]?.depth ?? 0) < (step.depth ?? 0)}
              />
            </Fragment>
          ))}
        </ol>
      ) : null}
      {trace ? <Simulated actions={trace.actions} /> : null}
      {trace?.end ? <p className={hint}>Ended: {words(trace.end).toLowerCase()}.</p> : null}
    </div>
  );
}

/* LiveTalk is a run's chat as it is written, with dimmi working when an agent step is answering. */
export function LiveWfTalk({
  run,
  sources,
  bubble = fromCustomer,
}: {
  run?: LiveWfRun;
  sources: ChipSources;
  bubble?: string;
}) {
  return (
    <div className="flex flex-col gap-5">
      <WfTalk messages={[]} result={null} lines={run?.lines ?? []} sources={sources} bubble={bubble} />
      {run?.working ? <p className={hint}>dimmi is answering…</p> : null}
      {!run || (run.lines.length === 0 && !run.judging) ? <p className={hint}>Starting the run…</p> : null}
      {run?.judging ? <p className={hint}>Judging the checks…</p> : null}
    </div>
  );
}
