/*
 * One workflow simulation on the draft and on the live version, side by
 * side: how each went, the chat each had, the steps and paths each took,
 * and the checks each failed. Run both saves the draft, then runs it and
 * the live version at the same time, each column streaming its own run.
 * Each column is a fixed height: its chat and steps scroll, and its
 * checks sit in a card pinned under them, always in view.
 */
import { RiPlayFill } from "@remixicon/react";
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import type { AnswerAs } from "@/lib/api/agents";
import {
  runWorkflowSimulations,
  type WorkflowSimResult,
  type WorkflowSimulation,
  workflowSimulationsQuery,
} from "@/lib/api/workflow-simulations";
import { formatRelative } from "@/lib/i18n/format";
import { cx } from "@/utils/cx";

import { fieldLabel, fromCustomer, hint } from "../procedure-simulator/simulator";
import type { ChipSources } from "../procedure-writer/procedure-suggestions";
import { type LiveWfRun, useWorkflowLiveRuns } from "./use-workflow-live-runs";
import { LiveWfTalk, Trace, WfCheckList, WfTalk } from "./workflow-sim-parts";
import { liveStatus, resultStatus, type Steps } from "./workflow-sim-words";

/* A customer's bubble on the dialog's primary surface steps down to the secondary one. */
const onPrimary = cx(fromCustomer, "bg-background-secondary-default shadow-none");

function Side({
  title,
  sim,
  result,
  running,
  live,
  steps,
  sources,
}: {
  title: string;
  sim: WorkflowSimulation;
  result: WorkflowSimResult | null;
  running: boolean;
  live?: LiveWfRun;
  steps: Steps;
  sources: ChipSources;
}) {
  const status = running ? liveStatus(live) : resultStatus(result);
  const shown = running ? (live?.result ?? null) : result;

  return (
    <section aria-label={title} className="flex h-[min(68vh,680px)] min-w-0 flex-col gap-3">
      <span className="flex shrink-0 flex-wrap items-center gap-2">
        <span className="text-body-medium text-text-primary">{title}</span>
        <Chip variant="caption" color={status.color} className="py-0.5">
          {status.label}
        </Chip>
        {result && !running ? <span className={hint}>{formatRelative(result.ran_at)}</span> : null}
      </span>
      {!running && result?.error ? (
        <p className="shrink-0 text-body-2-regular text-text-error-primary">{result.error}</p>
      ) : null}
      <div className="flex min-h-0 flex-1 flex-col gap-5 overflow-y-auto pe-2 pb-2 [scrollbar-width:thin]">
        {running ? (
          <LiveWfTalk run={live} sources={sources} bubble={onPrimary} />
        ) : (
          <WfTalk messages={sim.messages} result={result} sources={sources} bubble={onPrimary} />
        )}
        <div className="flex flex-col gap-2">
          <span className={fieldLabel}>Steps</span>
          <Trace trace={shown?.trace} steps={steps} live={running ? live?.steps : undefined} />
        </div>
      </div>
      <div className="flex max-h-[40%] shrink-0 flex-col gap-1.5 overflow-y-auto rounded-xl bg-background-secondary-default p-3 [scrollbar-width:thin]">
        <span className={fieldLabel}>Checks</span>
        <WfCheckList checks={sim.assertions} result={shown} steps={steps} sources={sources} />
      </div>
    </section>
  );
}

export function WorkflowSimCompare({
  workspace,
  workflow,
  sim,
  steps,
  sources,
  liveVersion,
  flush,
  onClose,
}: {
  workspace: string;
  workflow: string;
  sim: WorkflowSimulation;
  steps: Steps;
  sources: ChipSources;
  /* The live version's number, when known, for the column's title. */
  liveVersion?: number;
  flush?: () => Promise<void>;
  onClose: () => void;
}) {
  const queryClient = useQueryClient();
  const [running, setRunning] = useState(false);
  const draft = useWorkflowLiveRuns(workspace);
  const live = useWorkflowLiveRuns(workspace);
  const streams = { draft, live };

  const runOne = (version: AnswerAs) =>
    runWorkflowSimulations(workspace, workflow, {
      simulation_ids: [sim.id],
      version,
      stream_id: streams[version].begin(),
    });

  const runBoth = async () => {
    setRunning(true);
    try {
      if (flush) {
        await flush();
      }
      const ran = await Promise.allSettled([runOne("draft"), runOne("live")]);
      await queryClient.invalidateQueries(workflowSimulationsQuery(workspace, workflow));
      const failed = ran.find((r) => r.status === "rejected");
      if (failed) {
        toast.error(failed.reason instanceof Error ? failed.reason.message : "The simulation did not run.");
      }
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
            running={running}
            live={draft.runs[sim.id]}
            steps={steps}
            sources={sources}
          />
          <Side
            title={liveVersion ? `Live · version ${String(liveVersion)}` : "Live"}
            sim={sim}
            result={sim.live_result}
            running={running}
            live={live.runs[sim.id]}
            steps={steps}
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
