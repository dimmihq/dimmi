/*
 * The runs view beside the canvas: the workflow's latest runs, newest
 * first, each with its status, what started it, when, and the
 * conversation it ran on. Opening a run lists what each of its steps did,
 * in order: the step as its card reads, how it ended, the path it took
 * and what it found. Steps since removed from the workflow read by their
 * kind's name.
 */
import { RiArrowDownSLine, RiCloseLine, RiHistoryLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Row, Section } from "@/features/settings/layout";
import { EmptyState, PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { type RunStep, type WorkflowRun, workflowRunsQuery } from "@/lib/api/workflows";
import { formatRelative } from "@/lib/i18n/format";
import { cx } from "@/utils/cx";

import { KindMark } from "./parts";
import { detailLine, runStatusName, runStatusTint, stepStatusName, stepStatusTint } from "./run-lines";
import { stepLook } from "./step-kinds";
import { type TriggerKind, triggers } from "./triggers";
import { findStep, pathName, type Step, type StepKind } from "./workflows";

interface Reading {
  steps: Step[];
  summarize: (step: Step) => string;
  workflows: ReadonlyMap<string, string>;
}

function StepLine({ done, reading }: { done: RunStep; reading: Reading }) {
  const look = stepLook[done.kind as StepKind];
  const step = findStep(reading.steps, done.step_id);
  const path = step?.paths?.find((current) => current.id === done.path_id);
  const took = path ? pathName(path) : done.outcome ? pathName({ id: "", outcome: done.outcome, steps: [] }) : "";
  const found = detailLine(done, reading.workflows);

  return (
    <li className="flex items-start gap-2.5 py-2">
      {look ? <KindMark icon={look.icon} tint={look.tint} size="sm" /> : null}
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="text-body-2-medium text-text-primary">
          {step ? reading.summarize(step) : (look?.name ?? done.kind)}
        </span>
        {took ? <span className="text-body-2-regular text-text-secondary">Took the path {took}</span> : null}
        {found ? <span className="break-words text-body-2-regular text-text-secondary">{found}</span> : null}
      </div>
      <Chip variant="caption" color={stepStatusTint[done.status] ?? "neutral"}>
        {stepStatusName[done.status] ?? done.status}
      </Chip>
    </li>
  );
}

function RunEntry({ run, reading }: { run: WorkflowRun; reading: Reading }) {
  const [open, setOpen] = useState(false);
  const trigger = triggers[run.trigger as TriggerKind]?.name ?? run.trigger;
  const when = formatRelative(run.started_at);

  return (
    <div className="border-b border-separator-border pe-2.5 last:border-b-0">
      <button
        type="button"
        aria-expanded={open}
        aria-label={`Run ${runStatusName[run.status] ?? run.status}, ${when}`}
        onClick={() => setOpen(!open)}
        className="flex min-h-[52px] w-full cursor-pointer items-center gap-3 py-2.5 text-start outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <span className="flex min-w-0 flex-1 flex-col">
          <span className="truncate text-body-regular text-text-primary">{trigger}</span>
          <span className="truncate text-body-2-regular text-text-secondary">
            {when}
            {run.parent_run_id ? " · from another workflow" : ""}
          </span>
        </span>
        <Chip variant="caption" color={runStatusTint[run.status] ?? "neutral"}>
          {runStatusName[run.status] ?? run.status}
        </Chip>
        <RiArrowDownSLine
          aria-hidden
          className={cx("size-4 shrink-0 text-foreground-icon-secondary transition-transform", open && "rotate-180")}
        />
      </button>
      {open ? (
        <div className="flex flex-col gap-1 pb-2.5">
          {run.end_reason ? (
            <p className="text-body-2-regular text-text-secondary">Ended: {run.end_reason.replaceAll("_", " ")}</p>
          ) : null}
          {run.conversation_id ? (
            <Link
              to="/d/inbox/$view/$conversation"
              params={{ view: "all", conversation: run.conversation_id }}
              className="text-body-2-medium text-text-primary underline underline-offset-2 outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring"
            >
              Open the conversation
            </Link>
          ) : null}
          {run.steps.length === 0 ? (
            <p className="text-body-2-regular text-text-secondary">No steps ran.</p>
          ) : (
            <ol aria-label="Steps of this run" className="flex flex-col">
              {run.steps.map((done, index) => (
                <StepLine key={`${done.step_id}-${String(index)}`} done={done} reading={reading} />
              ))}
            </ol>
          )}
        </div>
      ) : null}
    </div>
  );
}

export function WorkflowRuns({
  workspace,
  workflow,
  reading,
  onClose,
  className,
}: {
  workspace: string;
  workflow: string;
  reading: Reading;
  onClose?: () => void;
  className?: string;
}) {
  const { data: runs, isPending, isError } = useQuery(workflowRunsQuery(workspace, workflow));

  return (
    <Panel label="Recent runs" ground className={className}>
      <PageHeader title="Recent runs" level={2}>
        {onClose ? (
          <IconButton size="small" icon={RiCloseLine} aria-label="Close recent runs" onClick={onClose} />
        ) : null}
      </PageHeader>
      <ScrollArea ground className="px-4 pb-6">
        <div className="flex min-w-0 flex-col gap-6 pt-2">
          {isError ? (
            <Section title="Latest runs">
              <Row label="Runs could not be loaded" description="Try again in a moment." />
            </Section>
          ) : runs?.length === 0 ? (
            <EmptyState
              icon={RiHistoryLine}
              title="No runs yet"
              description="Runs show here once the workflow is published and starts."
            />
          ) : (
            <Section
              title="Latest runs"
              description="Newest first. Open a run to see what each step did."
              loading={isPending}
            >
              {(runs ?? []).map((run) => (
                <RunEntry key={run.id} run={run} reading={reading} />
              ))}
            </Section>
          )}
        </div>
      </ScrollArea>
    </Panel>
  );
}
