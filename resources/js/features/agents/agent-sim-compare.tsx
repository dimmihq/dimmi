/*
 * One simulation answered by the draft and by the live version, side by
 * side: how each went, what each said turn by turn, and the checks each
 * failed. Run both runs the draft, saved first, then the live version,
 * so the two columns always describe the same simulation.
 */
import { RiPlayFill } from "@remixicon/react";
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { MessageBody } from "@/features/inbox/message-body";
import { type AgentSimulation, type AnswerAs, agentSimulationsQuery, runAgentSimulations } from "@/lib/api/agents";
import type { TestResult } from "@/lib/api/procedures";
import { formatRelative } from "@/lib/i18n/format";

import { Did } from "../automation/procedure-simulator/parts";
import { fieldLabel, fromCustomer, hint, statusOf, words } from "../automation/procedure-simulator/simulator";
import type { ChipSources } from "../automation/procedure-writer/procedure-suggestions";

const pressedConfirm = "Pressed Confirm on the card";

/*
 * Talk is a simulated conversation as it ran, or the saved chat when it
 * has not run yet.
 */
export function Talk({
  sim,
  result,
  sources,
}: {
  sim: AgentSimulation;
  result: TestResult | null;
  sources: ChipSources;
}) {
  if (!result || result.turns.length === 0) {
    return (
      <div className="flex flex-col gap-3">
        {sim.messages.map((m, index) => (
          // biome-ignore lint/suspicious/noArrayIndexKey: a simulation's messages never reorder.
          <p key={index} className={fromCustomer}>
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
          <p className={fromCustomer}>{t.confirm || !t.message ? pressedConfirm : t.message}</p>
          <div className="flex flex-col gap-2">
            <MessageBody text={t.reply || words(t.outcome)} className="text-body-regular text-text-primary" />
            <Did calls={t.calls} outcome={t.outcome} sources={sources} />
          </div>
        </li>
      ))}
    </ol>
  );
}

function Side({
  title,
  sim,
  result,
  sources,
}: {
  title: string;
  sim: AgentSimulation;
  result: TestResult | null;
  sources: ChipSources;
}) {
  const status = statusOf(result);
  const failed = (result?.assertion_results ?? []).filter((a) => !a.passed && !a.skipped);

  return (
    <section aria-label={title} className="flex min-w-0 flex-col gap-4">
      <span className="flex flex-wrap items-center gap-2">
        <span className="text-body-medium text-text-primary">{title}</span>
        <Chip variant="caption" color={status.color} className="py-0.5">
          {status.label}
        </Chip>
        {result ? <span className={hint}>{formatRelative(result.ran_at)}</span> : null}
      </span>
      {result?.error ? <p className="text-body-2-regular text-text-error-primary">{result.error}</p> : null}
      {failed.length > 0 ? (
        <ul className="flex flex-col gap-1">
          {failed.map((a) => (
            <li key={a.message} className="text-body-2-regular text-text-error-primary">
              {a.message}
            </li>
          ))}
        </ul>
      ) : null}
      <div className="flex flex-col gap-2.5">
        <span className={fieldLabel}>Conversation</span>
        <Talk sim={sim} result={result} sources={sources} />
      </div>
    </section>
  );
}

export function SimCompare({
  workspace,
  agent,
  sim,
  liveVersion,
  sources,
  flush,
  onClose,
}: {
  workspace: string;
  agent: string;
  sim: AgentSimulation;
  liveVersion: number;
  sources: ChipSources;
  flush: () => Promise<void>;
  onClose: () => void;
}) {
  const queryClient = useQueryClient();
  const [running, setRunning] = useState(false);

  const runBoth = async () => {
    setRunning(true);
    try {
      await flush();
      for (const version of ["draft", "live"] as AnswerAs[]) {
        await runAgentSimulations(workspace, agent, { simulation_ids: [sim.id], version });
      }
      await queryClient.invalidateQueries(agentSimulationsQuery(workspace, agent));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "The simulation did not run.");
    } finally {
      setRunning(false);
    }
  };

  return (
    <Modal isOpen onOpenChange={(open) => (open ? undefined : onClose())} size="xl">
      <ModalHeader title={sim.name} description="The same simulation on your draft and on what customers get now." />
      <ModalBody>
        <div className="grid grid-cols-1 gap-8 pb-4 md:grid-cols-2">
          <Side title="Draft" sim={sim} result={sim.draft_result} sources={sources} />
          <Side title={`Live · version ${liveVersion}`} sim={sim} result={sim.live_result} sources={sources} />
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
