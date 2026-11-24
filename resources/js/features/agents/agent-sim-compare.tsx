/*
 * One simulation answered by the draft and by the live version, side by
 * side: how each went, what each said turn by turn, and the checks each
 * failed. Run both saves the draft, then runs it and the live version at
 * the same time, each column streaming its own run as it happens. Each
 * column is a fixed height: its chat scrolls and follows the run as it
 * is written, and its checks sit in a card pinned under it, always in
 * view.
 */
import {
  RiCheckboxBlankCircleLine,
  RiCheckboxCircleFill,
  RiCloseCircleFill,
  RiPlayFill,
  RiQuestionFill,
} from "@remixicon/react";
import { useQueryClient } from "@tanstack/react-query";
import { type ReactNode, useEffect, useRef, useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { MessageBody } from "@/features/inbox/message-body";
import { ActivityLog } from "@/features/operator/activity-log";
import {
  type AgentSimulation,
  type AgentSimulationHistory,
  type AnswerAs,
  agentSimulationsQuery,
  runAgentSimulations,
} from "@/lib/api/agents";
import type { TestResult } from "@/lib/api/procedures";
import { formatRelative } from "@/lib/i18n/format";
import { cx } from "@/utils/cx";

import { Did } from "../automation/procedure-simulator/parts";
import { fieldLabel, fromCustomer, hint, sentenceOf, words } from "../automation/procedure-simulator/simulator";
import { type LiveRun, useLiveRuns } from "../automation/procedure-simulator/use-live-runs";
import type { ChipSources } from "../automation/procedure-writer/procedure-suggestions";
import { choicesOf, judgedOf, sayOf, statusNow } from "./agent-sim-words";
import { Transferred } from "./agent-transferred";

const pressedConfirm = "Pressed Confirm on the card";

/*
 * A customer's message where the chat sits on a primary surface, such
 * as a dialog: the bubble steps down to the secondary surface so it
 * still reads as a bubble.
 */
const fromCustomerOnPrimary = cx(fromCustomer, "bg-background-secondary-default shadow-none");

type CheckResult = TestResult["assertion_results"][number];

export function Mark({ result }: { result?: CheckResult }) {
  if (!result || result.skipped) {
    return (
      <RiCheckboxBlankCircleLine aria-label="Not run" className="size-4.5 shrink-0 text-foreground-icon-quaternary" />
    );
  }

  if (result.unsure) {
    return <RiQuestionFill aria-label="Unsure" className="size-4.5 shrink-0 text-status-yellow-text" />;
  }

  return result.passed ? (
    <RiCheckboxCircleFill aria-label="Passed" className="size-4.5 shrink-0 text-notification-success-foreground" />
  ) : (
    <RiCloseCircleFill aria-label="Failed" className="size-4.5 shrink-0 text-foreground-icon-error" />
  );
}

/*
 * CheckList is a simulation's checks as one run left them: each with
 * its mark, and why it failed when it did. Before a run, and while one
 * is going, the marks are empty.
 */
export function CheckList({
  sim,
  result,
  name,
  sources,
}: {
  sim: AgentSimulation;
  result: TestResult | null;
  name: string;
  sources: ChipSources;
}) {
  const choices = choicesOf(sources);
  /* Results line up with the checks only until the checks are edited. */
  const lined = result && result.assertion_results.length === sim.assertions.length ? result.assertion_results : [];

  if (sim.assertions.length === 0) {
    return <p className={hint}>This simulation has no checks.</p>;
  }

  return (
    <ul className="flex flex-col gap-1">
      {sim.assertions.map((a, index) => {
        const outcome = lined[index];
        const said = judgedOf(outcome);

        return (
          // biome-ignore lint/suspicious/noArrayIndexKey: checks never reorder while shown.
          <li key={index} className="flex items-start gap-2">
            <span className="flex h-7 items-center">
              <Mark result={outcome} />
            </span>
            <span className="flex min-w-0 flex-1 flex-col py-1">
              <span className="text-body-2-regular text-text-primary">{sayOf(sentenceOf(a, choices), name)}</span>
              {said ? (
                <span
                  className={cx("text-body-2-regular", said.failed ? "text-text-error-primary" : "text-text-secondary")}
                >
                  {said.text}
                </span>
              ) : null}
            </span>
          </li>
        );
      })}
    </ul>
  );
}

/*
 * Talk is a simulated conversation as it ran, or the saved chat when it
 * has not run yet.
 */
export function Talk({
  sim,
  result,
  sources,
  bubble = fromCustomer,
}: {
  sim: AgentSimulation;
  result: TestResult | null;
  sources: ChipSources;
  bubble?: string;
}) {
  if (!result || result.turns.length === 0) {
    return (
      <div className="flex flex-col gap-3">
        {sim.messages.map((m, index) => (
          // biome-ignore lint/suspicious/noArrayIndexKey: a simulation's messages never reorder.
          <p key={index} className={bubble}>
            {m.text || pressedConfirm}
          </p>
        ))}
      </div>
    );
  }

  return (
    <ol aria-label="Simulated conversation" className="flex flex-col gap-5">
      {result.turns.map((t, index) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: a run's turns never reorder.
        <li key={index} className="flex flex-col gap-5">
          <p className={bubble}>{t.confirm || !t.message ? pressedConfirm : t.message}</p>
          {t.transfer ? <Transferred transfer={t.transfer} /> : null}
          <div className="flex flex-col gap-2">
            <MessageBody text={t.reply || words(t.outcome)} className="text-body-regular text-text-primary" />
            <Did calls={t.calls} outcome={t.outcome} sources={sources} />
          </div>
        </li>
      ))}
    </ol>
  );
}

export function LiveTalk({
  run,
  sources,
  bubble = fromCustomer,
}: {
  run?: LiveRun;
  sources: ChipSources;
  bubble?: string;
}) {
  const turns = run?.turns ?? [];

  return (
    <ol aria-label="Simulated conversation" aria-live="polite" className="flex flex-col gap-5">
      {turns.map((t, index) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: a run's turns never reorder.
        <li key={index} className="flex flex-col gap-5">
          <p className={bubble}>{t.message || pressedConfirm}</p>
          <div className="flex flex-col gap-3">
            <ActivityLog steps={t.steps} working={!t.result} />
            {t.result?.transfer ? <Transferred transfer={t.result.transfer} /> : null}
            {t.result ? (
              <div className="flex flex-col gap-2">
                <MessageBody
                  text={t.result.reply || words(t.result.outcome)}
                  className="text-body-regular text-text-primary"
                />
                <Did calls={t.result.calls} outcome={t.result.outcome} sources={sources} />
              </div>
            ) : null}
          </div>
        </li>
      ))}
      {turns.length === 0 ? (
        <li>
          <ActivityLog steps={[]} working />
        </li>
      ) : null}
      {run?.judging ? <li className={hint}>Judging the checks…</li> : null}
    </ol>
  );
}

/*
 * Following keeps a chat's newest line in view while it is being
 * written: the box scrolls to its end each time what is inside grows.
 * A finished chat is left where the reader put it.
 */
function Following({ follow, children }: { follow: boolean; children: ReactNode }) {
  const box = useRef<HTMLDivElement>(null);
  const inside = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const scroller = box.current;
    const content = inside.current;
    if (!follow || !scroller || !content) {
      return;
    }
    const toEnd = () => {
      scroller.scrollTop = scroller.scrollHeight;
    };
    toEnd();
    const watch = new ResizeObserver(toEnd);
    watch.observe(content);

    return () => watch.disconnect();
  }, [follow]);

  return (
    <div ref={box} className="min-h-0 flex-1 overflow-y-auto pe-2 [scrollbar-width:thin]">
      <div ref={inside} className="pb-2">
        {children}
      </div>
    </div>
  );
}

function Side({
  title,
  sim,
  result,
  history,
  running,
  live,
  name,
  sources,
}: {
  title: string;
  sim: AgentSimulation;
  result: TestResult | null;
  history: AgentSimulationHistory | null;
  running: boolean;
  live?: LiveRun;
  name: string;
  sources: ChipSources;
}) {
  const status = statusNow(result, running, live, history);

  return (
    <section aria-label={title} className="flex h-[min(68vh,680px)] min-w-0 flex-col gap-3">
      <span className="flex shrink-0 flex-wrap items-center gap-2">
        <span className="text-body-medium text-text-primary">{title}</span>
        <Chip variant="caption" color={status.color} className="py-0.5">
          {status.label}
        </Chip>
        {result && !running ? <span className={hint}>{formatRelative(result.ran_at)}</span> : null}
      </span>
      {running ? null : result?.error ? (
        <p className="shrink-0 text-body-2-regular text-text-error-primary">{result.error}</p>
      ) : null}
      <Following follow={running}>
        {running ? (
          <LiveTalk run={live} sources={sources} bubble={fromCustomerOnPrimary} />
        ) : (
          <Talk sim={sim} result={result} sources={sources} bubble={fromCustomerOnPrimary} />
        )}
      </Following>
      <div className="flex max-h-[45%] shrink-0 flex-col gap-1.5 overflow-y-auto rounded-xl bg-background-secondary-default p-3 [scrollbar-width:thin]">
        <span className={fieldLabel}>Checks</span>
        <CheckList sim={sim} result={running ? (live?.result ?? null) : result} name={name} sources={sources} />
      </div>
    </section>
  );
}

export function SimCompare({
  workspace,
  agent,
  sim,
  name,
  liveVersion,
  sources,
  flush,
  onClose,
}: {
  workspace: string;
  agent: string;
  sim: AgentSimulation;
  name: string;
  liveVersion: number;
  sources: ChipSources;
  flush: () => Promise<void>;
  onClose: () => void;
}) {
  const queryClient = useQueryClient();
  const [running, setRunning] = useState(false);
  const draft = useLiveRuns(workspace, sources);
  const live = useLiveRuns(workspace, sources);
  const streams = { draft, live };

  const runOne = (version: AnswerAs) =>
    runAgentSimulations(workspace, agent, {
      simulation_ids: [sim.id],
      version,
      stream_id: streams[version].begin(),
    });

  const runBoth = async () => {
    setRunning(true);
    try {
      await flush();
      const ran = await Promise.allSettled([runOne("draft"), runOne("live")]);
      await queryClient.invalidateQueries(agentSimulationsQuery(workspace, agent));
      const failed = ran.find((r) => r.status === "rejected");
      if (failed) {
        toast.error(failed.reason instanceof Error ? failed.reason.message : "The simulation did not run.");
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "The simulation did not run.");
    } finally {
      setRunning(false);
      draft.end();
      live.end();
    }
  };

  return (
    <Modal isOpen onOpenChange={(open) => (open ? undefined : onClose())} size="xl">
      <ModalHeader title={sim.name} description="The same simulation on your draft and on what customers get now." />
      <ModalBody>
        <div className="grid grid-cols-1 gap-8 pb-2 md:grid-cols-2">
          <Side
            title="Draft"
            sim={sim}
            result={sim.draft_result}
            history={sim.draft_history}
            running={running}
            live={draft.runs[sim.id]}
            name={name}
            sources={sources}
          />
          <Side
            title={`Live · version ${liveVersion}`}
            sim={sim}
            result={sim.live_result}
            history={sim.live_history}
            running={running}
            live={live.runs[sim.id]}
            name={name}
            sources={sources}
          />
        </div>
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" size="small" onClick={onClose}>
          Close
        </Button>
        <Button size="small" leadingIcon={RiPlayFill} loading={running} onClick={() => void runBoth()}>
          Run both
        </Button>
      </ModalFooter>
    </Modal>
  );
}
