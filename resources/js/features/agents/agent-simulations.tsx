/*
 * An agent's simulations, the way a procedure's work: chats the agent
 * must keep getting right, each with the checks that judge it, run
 * against the draft or the live version. Each keeps how it last went
 * on both, and how its latest runs went: an agent's answers vary, so a
 * simulation reads as passing, flaky or failing over its last runs on
 * the agent as it stands now. The list says how many pass, and flags a
 * simulation whose draft and live results disagree; opening one slides
 * in its past runs first, where choosing a run slides that run's chat
 * and checks into place, then its
 * conversation and checks, where a check is removed or one is added in
 * plain words, and Compare puts its draft and live runs side by side.
 */
import {
  RiArrowLeftSLine,
  RiArrowRightSLine,
  RiCloseLine,
  RiDeleteBinLine,
  RiGitPullRequestLine,
  RiPlayFill,
} from "@remixicon/react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Input } from "@/components/base/input/input";
import { AlertDialog } from "@/components/base/modal/modal";
import { Textarea } from "@/components/base/textarea/textarea";
import { toast } from "@/components/base/toast/toast";
import { Resize, Swap } from "@/features/inbox/motion";
import { ScrollArea } from "@/features/shell/page";
import {
  type AgentSimulation,
  type AgentSimulationRun,
  type AnswerAs,
  agentSimulationsQuery,
  deleteAgentSimulation,
  historyOf,
  resultOf,
  runAgentSimulations,
  updateAgentSimulation,
} from "@/lib/api/agents";
import type { Assertion, TestResult } from "@/lib/api/procedures";
import { cx } from "@/utils/cx";

import { card, fieldLabel, hint, sentenceOf } from "../automation/procedure-simulator/simulator";
import { type LiveRun, useLiveRuns } from "../automation/procedure-simulator/use-live-runs";
import type { ChipSources } from "../automation/procedure-writer/procedure-suggestions";
import { LiveTalk, Mark, SimCompare, Talk } from "./agent-sim-compare";
import { Dots, PastRuns } from "./agent-sim-history";
import { choicesOf, judgedOf, lastRun, sayOf, statusNow } from "./agent-sim-words";

function Checks({
  sim,
  result,
  name,
  sources,
  readOnly,
  onChange,
}: {
  sim: AgentSimulation;
  result: TestResult | null;
  name: string;
  sources: ChipSources;
  readOnly: boolean;
  onChange: (assertions: Assertion[]) => void;
}) {
  const [adding, setAdding] = useState("");
  const choices = choicesOf(sources);
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
          const outcome = lined[index];
          const said = judgedOf(outcome);

          return (
            // biome-ignore lint/suspicious/noArrayIndexKey: checks change in place.
            <li key={index} className="flex items-start gap-2">
              <span className="flex h-7 items-center">
                <Mark result={outcome} />
              </span>
              <span className="flex min-w-0 flex-1 flex-col py-1">
                <span className="text-body-2-regular text-text-primary">{sayOf(sentenceOf(a, choices), name)}</span>
                {said ? (
                  <span
                    className={cx(
                      "text-body-2-regular",
                      said.failed ? "text-text-error-primary" : "text-text-secondary",
                    )}
                  >
                    {said.text}
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
            placeholder={`Add a check: ${name} should…`}
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
 * how it went on the copy shown and its checks.
 */
/*
 * Scenario is who the simulation's customer is and what they came for,
 * in the team's words. It is written when the simulation is saved and
 * edited here; a change waits for Save, since the next run plays from it.
 */
function Scenario({
  sim,
  readOnly,
  onSave,
}: {
  sim: AgentSimulation;
  readOnly: boolean;
  onSave: (scenario: string) => Promise<void>;
}) {
  const [text, setText] = useState(sim.scenario);
  const [saving, setSaving] = useState(false);
  const changed = text.trim() !== sim.scenario;
  const save = async () => {
    setSaving(true);
    await onSave(text.trim());
    setSaving(false);
  };

  return (
    <div className="flex flex-col gap-2">
      <Textarea
        label="Customer's situation"
        hint="Who they are and what they came for. The simulated customer answers from this, and only agrees to what it says they want."
        placeholder="A prospect shipping 300 parcels a month wants a plan recommendation. They have not asked to buy anything."
        rows={3}
        autoResize
        maxRows={8}
        maxLength={1000}
        isReadOnly={readOnly}
        value={text}
        onChange={setText}
      />
      {changed && !readOnly ? (
        <span className="flex items-center justify-end gap-2">
          <Button variant="secondary" size="xs" disabled={saving} onClick={() => setText(sim.scenario)}>
            Discard
          </Button>
          <Button size="xs" loading={saving} onClick={() => void save()}>
            Save
          </Button>
        </span>
      ) : null}
    </div>
  );
}

/*
 * usePastRun holds which past run is on screen in place of the latest,
 * and whether the one just chosen is newer than the one before it, so
 * its chat slides in from the side time moves. A run in progress shows
 * itself, whatever was chosen.
 */
function usePastRun(busy: boolean, latest: TestResult | null) {
  const [past, setPast] = useState<{ run: AgentSimulationRun; at: number } | null>(null);
  const [newer, setNewer] = useState(true);
  const view = (run: AgentSimulationRun | null, at: number) => {
    setNewer(at < (past?.at ?? 0));
    setPast(run ? { run, at } : null);
  };

  const viewing = busy ? null : (past?.run ?? null);
  const showing = busy ? "live" : (viewing?.id ?? `latest:${latest?.ran_at ?? ""}`);

  return { viewing, newer, view, result: viewing ? viewing.result : latest, showing };
}

/*
 * Ran is one run of a simulation: what went wrong if it could not
 * finish, its chat, and its checks. Live is the run as it happens,
 * null before its first moment and left out once it is over.
 */
function Ran({
  sim,
  name,
  sources,
  result,
  shown,
  live,
  readOnly,
  onChecks,
}: {
  sim: AgentSimulation;
  name: string;
  sources: ChipSources;
  result: TestResult | null;
  shown: TestResult | null;
  live?: LiveRun | null;
  readOnly: boolean;
  onChecks: (assertions: Assertion[]) => void;
}) {
  return (
    <>
      {shown?.error ? <p className="text-body-2-regular text-text-error-primary">{shown.error}</p> : null}
      <div className="flex flex-col gap-2.5">
        <span className={fieldLabel}>Conversation</span>
        {live === undefined ? (
          <Talk sim={sim} result={result} sources={sources} />
        ) : (
          <LiveTalk run={live ?? undefined} sources={sources} />
        )}
      </div>
      <Checks sim={sim} result={shown} name={name} sources={sources} readOnly={readOnly} onChange={onChecks} />
    </>
  );
}

function Detail({
  workspace,
  sim,
  as,
  name,
  sources,
  busy,
  live,
  onBack,
  onRun,
  onDelete,
  onChecks,
  onScenario,
  onCompare,
}: {
  workspace: string;
  sim: AgentSimulation;
  as: AnswerAs;
  name: string;
  sources: ChipSources;
  busy: boolean;
  live?: LiveRun;
  onBack: () => void;
  onRun: () => void;
  onDelete: () => void;
  onChecks: (assertions: Assertion[]) => void;
  onScenario: (scenario: string) => Promise<void>;
  onCompare?: () => void;
}) {
  const history = historyOf(sim, as);
  const latest = resultOf(sim, as);
  const { viewing, newer, view, result, showing } = usePastRun(busy, latest);
  const status = statusNow(latest, busy, live, history);
  const shown = busy ? (live?.result ?? null) : result;
  const when = busy ? "" : lastRun(latest, history);

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
        <span className="flex shrink-0 items-center gap-1">
          <IconButton
            variant="quiet"
            size="small"
            icon={RiDeleteBinLine}
            aria-label={`Delete ${sim.name}`}
            onClick={onDelete}
          />
          {onCompare ? (
            <Button variant="tertiary" size="xs" leadingIcon={RiGitPullRequestLine} disabled={busy} onClick={onCompare}>
              Compare
            </Button>
          ) : null}
          <Button
            variant="tertiary"
            size="xs"
            leadingIcon={RiPlayFill}
            loading={busy}
            disabled={busy}
            onClick={() => {
              view(null, 0);
              onRun();
            }}
          >
            Run
          </Button>
        </span>
      </div>
      <ScrollArea>
        <div className="flex flex-col gap-6 px-4 pt-2 pb-10">
          <span className="flex items-center gap-2">
            <Chip variant="caption" color={status.color} className="py-0.5">
              {status.label}
            </Chip>
            {history && !busy ? <Dots history={history} /> : null}
            <span className={hint}>
              {as === "draft" ? "On the draft" : "On the live version"}
              {when}
            </span>
          </span>
          {busy ? null : (
            <PastRuns
              workspace={workspace}
              agent={sim.agent_id}
              simulation={sim.id}
              as={as}
              viewing={viewing?.id ?? null}
              onView={view}
            />
          )}
          <Scenario key={`${sim.id}:${sim.scenario}`} sim={sim} readOnly={busy} onSave={onScenario} />
          <Resize>
            <Swap view={showing} forward={newer} className="flex flex-col gap-6">
              <Ran
                sim={sim}
                name={name}
                sources={sources}
                result={result}
                shown={shown}
                live={busy ? (live ?? null) : undefined}
                readOnly={busy || viewing !== null}
                onChecks={onChecks}
              />
            </Swap>
          </Resize>
        </div>
      </ScrollArea>
    </div>
  );
}

/* disagrees says the draft and the live version last went different ways. */
function disagrees(sim: AgentSimulation): boolean {
  return sim.draft_result !== null && sim.live_result !== null && sim.draft_result.passed !== sim.live_result.passed;
}

function said(sim: AgentSimulation): string {
  const first = sim.messages[0]?.text ?? "";

  return first && !first.startsWith(sim.name) ? `“${first}”` : "";
}

export function AgentSimulations({
  workspace,
  agent,
  name,
  as,
  liveVersion,
  sources,
  flush,
}: {
  workspace: string;
  agent: string;
  name: string;
  as: AnswerAs;
  /* The live version's number, 0 before the first publish. */
  liveVersion: number;
  sources: ChipSources;
  flush: () => Promise<void>;
}) {
  const queryClient = useQueryClient();
  const { data: list = [] } = useQuery(agentSimulationsQuery(workspace, agent));
  const [openId, setOpenId] = useState<string | null>(null);
  const [running, setRunning] = useState<string[]>([]);
  const [deleting, setDeleting] = useState<AgentSimulation | null>(null);
  const [comparing, setComparing] = useState(false);
  const refresh = () => queryClient.invalidateQueries(agentSimulationsQuery(workspace, agent));
  const live = useLiveRuns(workspace, sources);
  const opened = list.find((sim) => sim.id === openId);
  const standings = list.map((sim) => historyOf(sim, as)?.state ?? "none");
  const count = (state: string) => standings.filter((s) => s === state).length;
  const summary = [
    `${count("passing")} of ${list.length} passing`,
    count("flaky") > 0 ? `${count("flaky")} flaky` : "",
    count("failing") > 0 ? `${count("failing")} failing` : "",
    count("none") > 0 ? `${count("none")} not run` : "",
  ]
    .filter(Boolean)
    .join(", ");

  const run = async (ids?: string[]) => {
    setRunning(ids ?? list.map((sim) => sim.id));
    if (ids?.length === 1) {
      setOpenId(ids[0]);
    }
    try {
      if (as === "draft") {
        await flush();
      }
      const ran = await runAgentSimulations(workspace, agent, {
        simulation_ids: ids,
        version: as,
        stream_id: live.begin(),
      });
      await refresh();
      const broken = ran.map((sim) => [sim, resultOf(sim, as)] as const).find(([, r]) => r?.error);
      if (broken) {
        toast.error(`${broken[0].name}: ${broken[1]?.error}`);
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "The simulations did not run.");
    } finally {
      setRunning([]);
      live.end();
    }
  };

  const setChecks = async (sim: AgentSimulation, assertions: Assertion[]) => {
    try {
      await updateAgentSimulation(workspace, agent, sim.id, { assertions });
      await refresh();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "The check was not saved.");
    }
  };

  const setScenario = async (sim: AgentSimulation, scenario: string) => {
    try {
      await updateAgentSimulation(workspace, agent, sim.id, { scenario });
      await refresh();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "The customer's situation was not saved.");
    }
  };

  const confirm = (
    <AlertDialog
      isOpen={deleting !== null}
      onOpenChange={(open) => (open ? undefined : setDeleting(null))}
      title={deleting ? `Delete ${deleting.name}?` : "Delete simulation?"}
      description="This cannot be undone."
      confirmLabel="Delete"
      onConfirm={() => {
        const sim = deleting;
        setDeleting(null);
        setOpenId(null);

        return sim ? deleteAgentSimulation(workspace, agent, sim.id).then(refresh) : undefined;
      }}
    />
  );

  const detail = opened ? (
    <Detail
      key={`${opened.id}:${as}`}
      workspace={workspace}
      sim={opened}
      as={as}
      name={name}
      sources={sources}
      busy={running.includes(opened.id)}
      live={live.runs[opened.id]}
      onBack={() => setOpenId(null)}
      onRun={() => void run([opened.id])}
      onDelete={() => setDeleting(opened)}
      onChecks={(assertions) => void setChecks(opened, assertions)}
      onScenario={(scenario) => setScenario(opened, scenario)}
      onCompare={liveVersion > 0 ? () => setComparing(true) : undefined}
    />
  ) : null;

  const none = (
    <div className="flex flex-1 flex-col items-center justify-center gap-1 px-6 text-center">
      <span className="text-body-medium text-text-primary">No simulations yet</span>
      <span className={hint}>
        A simulation is a chat {name} must keep getting right. Chat in Preview, then press Save as simulation.
      </span>
    </div>
  );

  const all = (
    <>
      <div className="flex h-11 shrink-0 items-center justify-between gap-2 ps-4 pe-3">
        <p className={cx(hint, "min-w-0 truncate tabular-nums")}>
          {summary} on the {as === "draft" ? "draft" : "live version"}
        </p>
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
      </div>
      <ScrollArea>
        <ul className={cx("mx-4 mt-1 mb-10 flex flex-col gap-0.5 p-1", card)}>
          {list.map((sim) => {
            const history = historyOf(sim, as);
            const busy = running.includes(sim.id);
            const status = statusNow(resultOf(sim, as), busy, live.runs[sim.id], history);

            return (
              <li key={sim.id}>
                <button
                  type="button"
                  onClick={() => setOpenId(sim.id)}
                  className="flex w-full cursor-pointer items-center gap-3 rounded-xl px-2.5 py-2 text-start outline-none transition-colors duration-150 hover:bg-background-primary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
                >
                  <span className="flex min-w-0 flex-1 flex-col">
                    <span className="truncate text-body-regular text-text-primary">{sim.name}</span>
                    <span className="flex min-w-0 items-center gap-2">
                      {history && !busy ? <Dots history={history} /> : null}
                      {said(sim) ? <span className={cx(hint, "truncate")}>{said(sim)}</span> : null}
                    </span>
                  </span>
                  {liveVersion > 0 && disagrees(sim) ? (
                    <Chip variant="caption" color="orange" className="shrink-0 py-0.5">
                      Live differs
                    </Chip>
                  ) : null}
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

  return (
    <div className="relative flex min-h-0 flex-1 flex-col overflow-hidden">
      <Swap view={opened ? opened.id : "list"} forward={opened !== undefined} className="flex min-h-0 flex-1 flex-col">
        {detail ?? (list.length === 0 ? none : all)}
      </Swap>
      {confirm}
      {comparing && opened ? (
        <SimCompare
          workspace={workspace}
          agent={agent}
          sim={opened}
          name={name}
          liveVersion={liveVersion}
          sources={sources}
          flush={flush}
          onClose={() => setComparing(false)}
        />
      ) : null}
    </div>
  );
}
