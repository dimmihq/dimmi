/*
 * A procedure's simulations: customer messages dimmi must keep getting
 * right. Each is a chat, written here or saved from the preview, with
 * checks on what dimmi did. The list shows each one with how it last
 * did, and runs one or all of them against the working copy, so a
 * change that breaks one is seen before it is published.
 *
 * A new simulation asks for two things: what the customer writes, and,
 * if wanted, what dimmi should do, in plain words for dimmi to judge.
 * A simulation opens to its conversation and its checks, where a check
 * can be removed or another added.
 */
import {
  RiArrowLeftSLine,
  RiArrowRightSLine,
  RiCheckboxBlankCircleLine,
  RiCheckboxCircleFill,
  RiCloseCircleFill,
  RiCloseLine,
  RiDeleteBinLine,
  RiPlayFill,
  RiQuestionFill,
} from "@remixicon/react";
import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";

import { Chip, type ChipColor } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Input } from "@/components/base/input/input";
import { AlertDialog } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { MessageBody } from "@/features/inbox/message-body";
import { Swap } from "@/features/inbox/motion";
import { ActivityLog } from "@/features/operator/activity-log";
import { ScrollArea } from "@/features/shell/page";
import { Reveal } from "@/features/shell/reveal";
import {
  type Assertion,
  deleteTestCase,
  type ProcedureTestCase,
  runTestCases,
  type TestResult,
  testCasesQuery,
  updateTestCase,
} from "@/lib/api/procedures";
import { formatDateTime, formatRelative } from "@/lib/i18n/format";
import { cx } from "@/utils/cx";

import type { ChipSources } from "../procedure-writer/procedure-suggestions";
import { Did } from "./parts";
import {
  card,
  criterionChoices,
  type Draft,
  fieldLabel,
  fromCustomer,
  hint,
  sentenceOf,
  statusOf,
  words,
} from "./simulator";
import { type LiveRun, useLiveRuns } from "./use-live-runs";

/*
 * sayOf reads a check as one plain sentence. The brand keeps its small
 * letter even at the start.
 */
function sayOf(said: { kind: string; claim: string }): string {
  if (said.kind === "Reply") {
    return `dimmi's reply ${said.claim}`;
  }

  return said.claim.startsWith("dimmi") ? said.claim : words(said.claim);
}

type CheckResult = TestResult["assertion_results"][number];

function Mark({ result }: { result?: CheckResult }) {
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
 * Talk is the simulation's conversation: how it went when it last ran,
 * or what the customer writes when it has not run yet.
 */
const pressedConfirm = "Pressed Confirm on the card";

/* A run before its first event arrives. */
const idle: LiveRun = { turns: [], total: 0, judging: false };

function Talk({ sim, sources }: { sim: ProcedureTestCase; sources: ChipSources }) {
  const result = sim.last_result;
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
          <div className="flex flex-col items-end gap-1">
            <p className={fromCustomer}>{t.confirm || !t.message ? pressedConfirm : t.message}</p>
            {t.improvised ? (
              <span className={hint}>The saved chat never gave this, so the customer said they did not know.</span>
            ) : null}
          </div>
          <div className="flex flex-col gap-2">
            <MessageBody text={t.reply || words(t.outcome)} className="text-body-regular text-text-primary" />
            <Did calls={t.calls} outcome={t.outcome} sources={sources} />
          </div>
        </li>
      ))}
    </ol>
  );
}

/*
 * LiveTalk is a simulation's conversation while it runs: each message
 * the customer sends, what dimmi does as it works, and each reply as it
 * lands. The newest line stays in view.
 */
function LiveTalk({ run, sources }: { run: LiveRun; sources: ChipSources }) {
  const end = useRef<HTMLDivElement>(null);

  // biome-ignore lint/correctness/useExhaustiveDependencies: the newest line is kept in view as the run goes on.
  useEffect(() => {
    end.current?.scrollIntoView({ block: "end" });
  }, [run]);

  return (
    <ol aria-label="Simulated conversation" aria-live="polite" className="flex flex-col gap-5">
      {run.turns.map((t, index) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: a run's turns never reorder.
        <li key={index} className="flex flex-col gap-5">
          <Reveal>
            <p className={fromCustomer}>{t.message || pressedConfirm}</p>
          </Reveal>
          <div className="flex flex-col gap-3">
            <ActivityLog steps={t.steps} working={!t.result} />
            {t.result ? (
              <Reveal>
                <div className="flex flex-col gap-2">
                  <MessageBody
                    text={t.result.reply || words(t.result.outcome)}
                    className="text-body-regular text-text-primary"
                  />
                  <Did calls={t.result.calls} outcome={t.result.outcome} sources={sources} />
                </div>
              </Reveal>
            ) : null}
          </div>
        </li>
      ))}
      {run.turns.length === 0 ? (
        <li>
          <ActivityLog steps={[]} working />
        </li>
      ) : null}
      {run.judging ? <li className={hint}>Judging the checks…</li> : null}
      <div ref={end} />
    </ol>
  );
}

/*
 * statusNow is a simulation's chip: how its last run went, or while it
 * runs, how far it is, then its verdict as soon as it has one.
 */
function statusNow(sim: ProcedureTestCase, busy: boolean, live?: LiveRun): { label: string; color: ChipColor } {
  if (!busy) {
    return statusOf(sim.last_result);
  }
  if (live?.result) {
    return statusOf(live.result);
  }
  if (live?.judging) {
    return { label: "Judging", color: "blue" };
  }
  /* The customer answers as the chat goes, so the number of turns is only known at the end. */
  const turn = live && live.turns.length > 0 ? ` · turn ${live.turns.length}` : "";

  return { label: `Running${turn}`, color: "blue" };
}

/*
 * Checks are what must hold for the simulation to pass, each a sentence
 * with how it did. A new one is written in plain words.
 */
function Checks({
  sim,
  draft,
  sources,
  readOnly,
  onChange,
}: {
  sim: ProcedureTestCase;
  draft: Draft;
  sources: ChipSources;
  readOnly: boolean;
  onChange: (assertions: Assertion[]) => void;
}) {
  const [adding, setAdding] = useState("");
  const choices = criterionChoices(draft, sources);
  const result = sim.last_result;
  /* Results line up with the checks only until the checks are edited. */
  const lined = result && result.assertion_results.length === sim.assertions.length ? result.assertion_results : [];
  const add = () => {
    const rubric = adding.trim();
    if (rubric) {
      onChange([...sim.assertions, { type: "llm_rubric", rubric }]);
      setAdding("");
    }
  };

  return (
    <div className="flex flex-col gap-2.5">
      <div className="flex flex-col">
        <span className={fieldLabel}>Checks</span>
        <span className={hint}>The simulation passes when all of these are true.</span>
      </div>
      <ul className="flex flex-col gap-1">
        {sim.assertions.map((a, index) => {
          const said = sentenceOf(a, choices);
          const outcome = lined[index];

          return (
            // biome-ignore lint/suspicious/noArrayIndexKey: checks change in place.
            <li key={index} className="flex items-start gap-2">
              <span className="flex h-7 items-center">
                <Mark result={outcome} />
              </span>
              <span className="flex min-w-0 flex-1 flex-col py-1">
                <span className="text-body-2-regular text-text-primary">{sayOf(said)}</span>
                {outcome && !outcome.passed && !outcome.skipped ? (
                  <span
                    className={cx(
                      "text-body-2-regular",
                      outcome.unsure ? "text-text-secondary" : "text-text-error-primary",
                    )}
                  >
                    {outcome.message}
                  </span>
                ) : null}
              </span>
              {readOnly ? null : (
                <IconButton
                  variant="quiet"
                  size="small"
                  icon={RiCloseLine}
                  aria-label="Remove check"
                  onClick={() => onChange(sim.assertions.filter((_, at) => at !== index))}
                />
              )}
            </li>
          );
        })}
      </ul>
      {readOnly ? null : (
        <div className="flex items-center gap-2">
          <Input
            aria-label="Add a check"
            size="small"
            placeholder="Add a check: dimmi should…"
            className="min-w-0 flex-1"
            value={adding}
            onChange={setAdding}
            onKeyDown={(event) => {
              if (event.key === "Enter") {
                add();
              }
            }}
          />
          <Button variant="secondary" size="small" disabled={!adding.trim()} onClick={add}>
            Add
          </Button>
        </div>
      )}
    </div>
  );
}

/*
 * Detail is one simulation opened: its name, running and deleting it,
 * how it went and its checks.
 */
function Detail({
  sim,
  draft,
  sources,
  readOnly,
  busy,
  live,
  onBack,
  onRun,
  onDelete,
  onChecks,
}: {
  sim: ProcedureTestCase;
  draft: Draft;
  sources: ChipSources;
  readOnly: boolean;
  busy: boolean;
  live?: LiveRun;
  onBack: () => void;
  onRun: () => void;
  onDelete: () => void;
  onChecks: (assertions: Assertion[]) => void;
}) {
  const status = statusNow(sim, busy, live);
  /* While it runs, the checks show the new verdict once there is one, and nothing before that. */
  const shown = busy ? { ...sim, last_result: live?.result ?? null } : sim;

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex h-12 shrink-0 items-center gap-2.5 ps-3 pe-3">
        <IconButton
          variant="quiet"
          size="small"
          icon={RiArrowLeftSLine}
          aria-label="Back to simulations"
          onClick={onBack}
        />
        <span className="min-w-0 flex-1 truncate text-body-medium text-text-primary">{sim.name}</span>
        {readOnly ? null : (
          <span className="flex shrink-0 items-center gap-1">
            <IconButton
              variant="quiet"
              size="small"
              icon={RiDeleteBinLine}
              aria-label={`Delete ${sim.name}`}
              onClick={onDelete}
            />
            <Button
              variant="tertiary"
              size="xs"
              leadingIcon={RiPlayFill}
              loading={busy}
              disabled={busy}
              onClick={onRun}
            >
              Run
            </Button>
          </span>
        )}
      </div>
      <ScrollArea>
        <div className="flex flex-col gap-6 px-4 pt-2 pb-10">
          <span className="flex items-center gap-2">
            <Chip variant="caption" color={status.color} className="py-0.5">
              {status.label}
            </Chip>
            {sim.last_result && !busy ? <span className={hint}>{formatRelative(sim.last_result.ran_at)}</span> : null}
          </span>
          {sim.simulated_at ? <p className={hint}>Runs as if it were {formatDateTime(sim.simulated_at)}.</p> : null}
          {shown.last_result?.error ? (
            <p className="text-body-2-regular text-text-error-primary">{shown.last_result.error}</p>
          ) : null}
          <div className="flex flex-col gap-2.5">
            <span className={fieldLabel}>Conversation</span>
            {busy ? <LiveTalk run={live ?? idle} sources={sources} /> : <Talk sim={sim} sources={sources} />}
          </div>
          <Checks sim={shown} draft={draft} sources={sources} readOnly={readOnly || busy} onChange={onChecks} />
        </div>
      </ScrollArea>
    </div>
  );
}

/*
 * said is what the customer writes first, shown under a simulation's
 * name unless the name already says it.
 */
function said(sim: ProcedureTestCase): string {
  const first = sim.messages[0]?.text ?? "";

  return first && !first.startsWith(sim.name) ? `“${first}”` : "";
}

export function ProcedureSimulations({
  workspace,
  draft,
  sources,
  readOnly,
  list,
}: {
  workspace: string;
  draft: Draft;
  sources: ChipSources;
  readOnly: boolean;
  list: ProcedureTestCase[];
}) {
  const queryClient = useQueryClient();
  const procedure = draft.server.id;
  const [openId, setOpenId] = useState<string | null>(null);
  const [running, setRunning] = useState<string[]>([]);
  const [deleting, setDeleting] = useState<ProcedureTestCase | null>(null);
  const refresh = () => queryClient.invalidateQueries(testCasesQuery(workspace, procedure));
  const live = useLiveRuns(workspace, sources);
  const opened = list.find((sim) => sim.id === openId);
  const passing = list.filter((sim) => sim.last_result?.passed).length;

  const run = async (ids?: string[]) => {
    setRunning(ids ?? list.map((sim) => sim.id));
    if (ids?.length === 1) {
      setOpenId(ids[0]);
    }
    try {
      await draft.flush();
      const ran = await runTestCases(workspace, procedure, ids, live.begin());
      await refresh();
      const broken = ran.find((sim) => sim.last_result?.error);
      if (broken?.last_result?.error) {
        toast.error(`${broken.name}: ${broken.last_result.error}`);
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "The simulations did not run.");
    } finally {
      setRunning([]);
      live.end();
    }
  };

  const setChecks = async (sim: ProcedureTestCase, assertions: Assertion[]) => {
    try {
      await updateTestCase(workspace, procedure, sim.id, { assertions });
      await refresh();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "The check was not saved.");
    }
  };

  const confirm = (
    <AlertDialog
      isOpen={deleting !== null}
      onOpenChange={(open) => {
        if (!open) {
          setDeleting(null);
        }
      }}
      title={deleting ? `Delete ${deleting.name}?` : "Delete simulation?"}
      description="This cannot be undone."
      confirmLabel="Delete"
      onConfirm={() => {
        const sim = deleting;
        setDeleting(null);
        setOpenId(null);

        return sim ? deleteTestCase(workspace, procedure, sim.id).then(refresh) : undefined;
      }}
    />
  );

  const detail = opened ? (
    <Detail
      sim={opened}
      draft={draft}
      sources={sources}
      readOnly={readOnly}
      busy={running.includes(opened.id)}
      live={live.runs[opened.id]}
      onBack={() => setOpenId(null)}
      onRun={() => void run([opened.id])}
      onDelete={() => setDeleting(opened)}
      onChecks={(assertions) => void setChecks(opened, assertions)}
    />
  ) : null;

  const none = (
    <div className="flex flex-1 flex-col items-center justify-center gap-1 px-6 text-center">
      <span className="text-body-medium text-text-primary">No simulations yet</span>
      <span className={hint}>
        A simulation is a chat dimmi must keep getting right. Chat in Preview, then press Save as simulation.
      </span>
    </div>
  );

  const all = (
    <>
      <div className="flex h-11 shrink-0 items-center justify-between gap-2 ps-4 pe-3">
        <p className={cx(hint, "min-w-0 truncate tabular-nums")}>
          {passing} of {list.length} passing
        </p>
        {readOnly ? null : (
          <Button
            variant="tertiary"
            size="xs"
            leadingIcon={RiPlayFill}
            loading={running.length > 1}
            disabled={running.length > 0}
            onClick={() => void run()}
          >
            Run all
          </Button>
        )}
      </div>
      <ScrollArea>
        <ul className={cx("mx-4 mt-1 mb-10 flex flex-col gap-0.5 p-1", card)}>
          {list.map((sim) => {
            const status = statusNow(sim, running.includes(sim.id), live.runs[sim.id]);

            return (
              <li key={sim.id}>
                <button
                  type="button"
                  onClick={() => setOpenId(sim.id)}
                  className="flex w-full cursor-pointer items-center gap-3 rounded-xl px-2.5 py-2 text-start outline-none transition-colors duration-150 hover:bg-background-primary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
                >
                  <span className="flex min-w-0 flex-1 flex-col">
                    <span className="truncate text-body-regular text-text-primary">{sim.name}</span>
                    {said(sim) ? <span className={cx(hint, "truncate")}>{said(sim)}</span> : null}
                  </span>
                  <Chip variant="caption" color={status.color} className="shrink-0 py-0.5">
                    {status.label}
                  </Chip>
                  <RiArrowRightSLine
                    aria-hidden
                    className="size-5 shrink-0 text-foreground-icon-tertiary rtl:-scale-x-100"
                  />
                </button>
              </li>
            );
          })}
        </ul>
      </ScrollArea>
    </>
  );

  /* Opening a simulation slides it in from the end; going back slides the list in from the start. */
  return (
    <div className="relative flex min-h-0 flex-1 flex-col overflow-hidden">
      <Swap view={opened ? opened.id : "list"} forward={opened !== undefined} className="flex min-h-0 flex-1 flex-col">
        {detail ?? (list.length === 0 ? none : all)}
      </Swap>
      {confirm}
    </div>
  );
}
