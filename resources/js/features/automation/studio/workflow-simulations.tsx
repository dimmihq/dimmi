/*
 * A workflow's simulations, the way an agent's work: chats the workflow
 * must keep handling the way the team means, each with the checks that
 * judge it, run against the draft or the live version. An AI customer
 * plays each one: it taps buttons, answers what the workflow asks and
 * lets timeouts pass, and dimmi answers inside agent steps. Nothing is
 * sent and no action runs for real. Each simulation keeps how it last
 * went on both copies and how its latest runs went, so it reads as
 * passing, flaky or failing. Opening one shows its past runs, the
 * answers its HTTP requests, actions and extract steps get in place of
 * real calls, the chat, each step and path the workflow took (a called
 * workflow's steps nested under the call), and its checks; Compare puts
 * the draft and live runs side by side. Operator's tries, when it tests
 * what it built, play live above the list. A new simulation starts the way
 * a chat with the agent does: the customer's first message typed in the
 * composer at the bottom, with the plus to pick who writes it, a real
 * customer or a made-up one. Sending saves it and runs it at once.
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
import { useEffect, useRef, useState } from "react";
import { Avatar } from "@/components/base/avatar/avatar";
import { Chip, type ChipColor } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Input } from "@/components/base/input/input";
import { AlertDialog } from "@/components/base/modal/modal";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { Select, SelectItem } from "@/components/base/select/select";
import { Textarea } from "@/components/base/textarea/textarea";
import { toast } from "@/components/base/toast/toast";
import { Resize, Swap } from "@/features/inbox/motion";
import { avatarColor } from "@/features/shell/avatar-color";
import { ScrollArea } from "@/features/shell/page";
import type { AnswerAs } from "@/lib/api/agents";
import type { SimPersona } from "@/lib/api/procedures";
import {
  createWorkflowSimulation,
  deleteWorkflowSimulation,
  runWorkflowSimulations,
  simHistoryOf,
  simResultOf,
  updateWorkflowSimulation,
  type WorkflowAssertion,
  type WorkflowSimResult,
  type WorkflowSimulation,
  type WorkflowSimulationRun,
  workflowSimulationRunsQuery,
  workflowSimulationsQuery,
} from "@/lib/api/workflow-simulations";
import { workflowQuery } from "@/lib/api/workflows";
import { formatRelative } from "@/lib/i18n/format";
import { cx } from "@/utils/cx";

import { Dots } from "../../agents/agent-sim-history";
import { KindMark } from "../parts";
import { PreviewAddMenu } from "../procedure-simulator/preview-add-menu";
import { Added, PreviewComposer } from "../procedure-simulator/preview-composer";
import { card, fieldLabel, hint } from "../procedure-simulator/simulator";
import type { ChipSources } from "../procedure-writer/procedure-suggestions";
import { useChipSources } from "../procedure-writer/use-chip-sources";
import { stepLook } from "../step-kinds";
import { pathName, type StepKind } from "../workflows";
import type { OperatorTry } from "./use-operator-tries";
import { type LiveWfRun, useWorkflowLiveRuns } from "./use-workflow-live-runs";
import { WorkflowSimCompare } from "./workflow-sim-compare";
import { CheckLine, LiveWfTalk, Trace, WfTalk } from "./workflow-sim-parts";
import {
  lined,
  liveStatus,
  type MockKind,
  mockableSteps,
  mockDrafts,
  mocksOf,
  ranWhen,
  resultStatus,
  type Steps,
  simStanding,
} from "./workflow-sim-words";

/* The copies a simulation runs against, and where a new one starts from. */
const copies = [
  ["draft", "Draft"],
  ["live", "Live"],
] as const;

/* statusOfSim is how a simulation stands: its latest runs, or where its run is while it goes. */
function statusOfSim(sim: WorkflowSimulation, as: AnswerAs, busy: boolean, live?: LiveWfRun) {
  return busy ? liveStatus(live) : simStanding(simResultOf(sim, as), simHistoryOf(sim, as));
}

/* statusOfTry is where one of Operator's tries stands: running, or how its one run went. */
function statusOfTry(sim: WorkflowSimulation, busy: boolean, live?: LiveWfRun) {
  return busy ? liveStatus(live) : resultStatus(sim.draft_result);
}

/* pathChoices lists every path of every step, to check which one a step takes. */
function pathChoices(steps: Steps): { id: string; name: string; step: string; path: string }[] {
  const out: { id: string; name: string; step: string; path: string }[] = [];
  const walk = (list: Steps) => {
    for (const step of list) {
      const look = stepLook[step.kind as StepKind];
      for (const path of step.paths ?? []) {
        out.push({
          id: `${step.id}|${path.id}`,
          name: `${look?.name ?? step.kind}: ${pathName(path) || path.id}`,
          step: step.id,
          path: path.id,
        });
        walk(path.steps ?? []);
      }
    }
  };
  walk(steps);

  return out;
}

function Checks({
  sim,
  result,
  steps,
  sources,
  readOnly,
  onChange,
}: {
  sim: WorkflowSimulation;
  result: WorkflowSimResult | null;
  steps: Steps;
  sources: ChipSources;
  readOnly: boolean;
  onChange: (assertions: WorkflowAssertion[]) => void;
}) {
  const [adding, setAdding] = useState("");
  const results = lined(sim.assertions, result);
  const paths = pathChoices(steps);
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
        {sim.assertions.map((check, index) => (
          <CheckLine
            // biome-ignore lint/suspicious/noArrayIndexKey: checks change in place.
            key={index}
            check={check}
            outcome={results[index]}
            steps={steps}
            sources={sources}
          >
            {readOnly ? null : (
              <IconButton
                variant="quiet"
                size="small"
                icon={RiCloseLine}
                aria-label="Remove check"
                onClick={() => onChange(sim.assertions.filter((_, at) => at !== index))}
              />
            )}
          </CheckLine>
        ))}
      </ul>
      {readOnly ? null : (
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-2">
            <Input
              aria-label="Add a check"
              size="small"
              placeholder="Add a check: the workflow should…"
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
          {paths.length > 0 ? (
            <Select
              aria-label="Check the path a step takes"
              size="sm"
              placeholder="Check the path a step takes"
              selectedKey={null}
              onSelectionChange={(key) => {
                const picked = paths.find((p) => p.id === String(key));
                if (picked) {
                  onChange([...sim.assertions, { type: "path_taken", step_id: picked.step, branch: picked.path }]);
                }
              }}
              popoverClassName="w-72"
            >
              {paths.map((p) => (
                <SelectItem key={p.id} id={p.id} textValue={p.name}>
                  {p.name}
                </SelectItem>
              ))}
            </Select>
          ) : null}
        </div>
      )}
    </div>
  );
}

/*
 * Scenario is who the simulation's customer is and what they came for.
 * A change waits for Save, since the next run plays from it.
 */
function Scenario({
  sim,
  readOnly,
  onSave,
}: {
  sim: WorkflowSimulation;
  readOnly: boolean;
  onSave: (scenario: string) => Promise<void>;
}) {
  const [text, setText] = useState(sim.scenario);
  const [saving, setSaving] = useState(false);
  const changed = text.trim() !== sim.scenario;

  return (
    <div className="flex flex-col gap-2">
      <Textarea
        label="Customer's situation"
        hint="Who they are and what they came for. The simulated customer taps and answers from this."
        placeholder="A customer on the Pro plan was charged twice and wants a refund. They would tap Billing."
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
          <Button
            size="xs"
            loading={saving}
            onClick={() => {
              setSaving(true);
              void onSave(text.trim()).finally(() => setSaving(false));
            }}
          >
            Save
          </Button>
        </span>
      ) : null}
    </div>
  );
}

/* What each kind of mocked step answers, in words, with an example body. */
const mockWords: Record<MockKind, { hint: string; example: string; status: boolean }> = {
  http_request: {
    hint: "The response the request gets. A status of 400 or more takes the failure path.",
    example: '{"plan": "pro"}',
    status: true,
  },
  run_action: {
    hint: "The action's result, by response field. A status outside 2xx takes the failure path.",
    example: '{"plan": "pro"}',
    status: true,
  },
  extract: {
    hint: "The values it finds, by key. Left empty, the AI reads what the customer wrote.",
    example: '{"order": "A-1234"}',
    status: false,
  },
};

/*
 * Mocks are what the steps that would leave dimmi answer in a run: an
 * HTTP request or an action its status and body, an extract step the
 * values it finds. No request or action runs for real either way. A
 * change waits for Save, since the next run plays from it.
 */
function Mocks({
  sim,
  steps,
  readOnly,
  onSave,
}: {
  sim: WorkflowSimulation;
  steps: Steps;
  readOnly: boolean;
  onSave: (mocks: WorkflowSimulation["action_mocks"]) => Promise<void>;
}) {
  const list = mockableSteps(steps);
  const start = mockDrafts(
    sim.action_mocks,
    list.map((s) => s.id),
  );
  const [drafts, setDrafts] = useState(start);
  const [saving, setSaving] = useState(false);
  if (list.length === 0) {
    return null;
  }
  const { mocks, bad } = mocksOf(sim.action_mocks, drafts);
  const changed = JSON.stringify(drafts) !== JSON.stringify(start);

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-col">
        <span className={fieldLabel}>Answers in place of real calls</span>
        <span className={hint}>
          Nothing leaves dimmi. A request or action with no answer here succeeds with nothing.
        </span>
      </div>
      {list.map(({ id, kind }) => {
        const look = stepLook[kind as StepKind];
        const said = mockWords[kind];
        const draft = drafts[id] ?? { status: "", body: "" };
        const set = (next: Partial<typeof draft>) => setDrafts({ ...drafts, [id]: { ...draft, ...next } });

        return (
          <div key={id} className="flex flex-col gap-2">
            <span className="flex items-center gap-2">
              {look ? <KindMark icon={look.icon} tint={look.tint} size="sm" /> : null}
              <span className="text-body-2-medium text-text-primary">{look?.name ?? kind}</span>
              <span className={hint}>{id}</span>
            </span>
            {said.status ? (
              <Input
                label="Status"
                size="small"
                inputMode="numeric"
                placeholder="200"
                isReadOnly={readOnly}
                value={draft.status}
                onChange={(status) => set({ status })}
              />
            ) : null}
            <Textarea
              label={kind === "extract" ? "Values" : "Body"}
              hint={bad.includes(id) ? "Write a status from 100 to 599 and a body in JSON." : said.hint}
              isInvalid={bad.includes(id)}
              placeholder={said.example}
              rows={2}
              autoResize
              maxRows={8}
              isReadOnly={readOnly}
              value={draft.body}
              onChange={(body) => set({ body })}
            />
          </div>
        );
      })}
      {changed && !readOnly ? (
        <span className="flex items-center justify-end gap-2">
          <Button variant="secondary" size="xs" disabled={saving} onClick={() => setDrafts(start)}>
            Discard
          </Button>
          <Button
            size="xs"
            loading={saving}
            disabled={bad.length > 0}
            onClick={() => {
              setSaving(true);
              void onSave(mocks).finally(() => setSaving(false));
            }}
          >
            Save
          </Button>
        </span>
      ) : null}
    </div>
  );
}

/* RunRow is one kept run in the list of past runs. */
function RunRow({
  run,
  latest,
  open,
  onView,
}: {
  run: WorkflowSimulationRun;
  latest: boolean;
  open: boolean;
  onView: () => void;
}) {
  const label = run.errored ? "Did not finish" : run.passed ? "Passed" : "Failed";
  const dot = run.errored
    ? "bg-foreground-icon-quaternary"
    : run.passed
      ? "bg-status-lime-text"
      : "bg-status-rose-text";

  return (
    <li>
      <button
        type="button"
        aria-pressed={open}
        onClick={onView}
        className={cx(
          "flex w-full cursor-pointer items-center gap-2.5 rounded-xl px-2.5 py-1.5 text-start outline-none transition-colors duration-150 hover:bg-background-primary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring",
          open && "bg-background-primary-hover",
        )}
      >
        <span aria-hidden className={cx("size-1.5 shrink-0 rounded-full", dot)} />
        <span className="text-body-2-regular text-text-primary">{label}</span>
        <span className={cx(hint, "min-w-0 flex-1 truncate")}>
          {formatRelative(run.ran_at)}
          {latest ? " · latest" : ""}
        </span>
        {run.counted || run.errored ? null : <span className={hint}>Before the last change</span>}
      </button>
    </li>
  );
}

function PastRuns({
  workspace,
  workflow,
  sim,
  as,
  viewing,
  onView,
}: {
  workspace: string;
  workflow: string;
  sim: WorkflowSimulation;
  as: AnswerAs;
  viewing: string | null;
  onView: (result: WorkflowSimResult | null, id: string | null) => void;
}) {
  const { data: runs = [] } = useQuery(workflowSimulationRunsQuery(workspace, workflow, sim.id, as));
  if (runs.length < 2) {
    return null;
  }

  return (
    <div className="flex flex-col gap-2.5">
      <span className={fieldLabel}>Past runs</span>
      <ul className={cx("flex flex-col gap-0.5 p-1", card)}>
        {runs.map((run, i) => (
          <RunRow
            key={run.id}
            run={run}
            latest={i === 0}
            open={viewing === run.id || (viewing === null && i === 0)}
            onView={() => onView(i === 0 ? null : run.result, i === 0 ? null : run.id)}
          />
        ))}
      </ul>
      <p className={hint}>
        Runs made before the workflow or this simulation changed are kept here, and no longer count.
      </p>
    </div>
  );
}

/* DetailBar is an opened simulation's header: back, its name, delete, compare and run. */
function DetailBar({
  name,
  busy,
  onBack,
  onDelete,
  onCompare,
  onRun,
}: {
  name: string;
  busy: boolean;
  onBack: () => void;
  onDelete?: () => void;
  onCompare?: () => void;
  onRun?: () => void;
}) {
  return (
    <div className="flex h-12 shrink-0 items-center gap-2.5 ps-3 pe-3">
      <IconButton
        variant="quiet"
        size="small"
        icon={RiArrowLeftSLine}
        aria-label="Back to simulations"
        onClick={onBack}
      />
      <span className="min-w-0 flex-1 truncate text-body-medium text-text-primary">{name}</span>
      <span className="flex shrink-0 items-center gap-1">
        {onDelete ? (
          <IconButton
            variant="quiet"
            size="small"
            icon={RiDeleteBinLine}
            aria-label={`Delete ${name}`}
            onClick={onDelete}
          />
        ) : null}
        {onCompare ? (
          <Button variant="tertiary" size="xs" leadingIcon={RiGitPullRequestLine} disabled={busy} onClick={onCompare}>
            Compare
          </Button>
        ) : null}
        {onRun ? (
          <Button variant="tertiary" size="xs" leadingIcon={RiPlayFill} loading={busy} disabled={busy} onClick={onRun}>
            Run
          </Button>
        ) : null}
      </span>
    </div>
  );
}

/*
 * Ran is one run of a simulation: what went wrong if it could not
 * finish, its chat and the steps the workflow took. Live is the run as
 * it happens, left out once it is over.
 */
function Ran({
  sim,
  result,
  running,
  live,
  steps,
  sources,
}: {
  sim: WorkflowSimulation;
  result: WorkflowSimResult | null;
  running: boolean;
  live?: LiveWfRun;
  steps: Steps;
  sources: ChipSources;
}) {
  const shown = running ? (live?.result ?? null) : result;

  return (
    <>
      {shown?.error ? <p className="text-body-2-regular text-text-error-primary">{shown.error}</p> : null}
      <div className="flex flex-col gap-2.5">
        <span className={fieldLabel}>Conversation</span>
        {running ? (
          <LiveWfTalk run={live} sources={sources} />
        ) : (
          <WfTalk messages={sim.messages} result={result} sources={sources} />
        )}
      </div>
      <div className="flex flex-col gap-2.5">
        <span className={fieldLabel}>Steps</span>
        <Trace trace={shown?.trace} steps={steps} live={running ? (live?.steps ?? []) : undefined} />
      </div>
    </>
  );
}

function Detail({
  workspace,
  workflow,
  sim,
  as,
  steps,
  sources,
  busy,
  live,
  onBack,
  onRun,
  onDelete,
  onChecks,
  onScenario,
  onMocks,
  onCompare,
}: {
  workspace: string;
  workflow: string;
  sim: WorkflowSimulation;
  as: AnswerAs;
  steps: Steps;
  sources: ChipSources;
  busy: boolean;
  live?: LiveWfRun;
  onBack: () => void;
  onRun: () => void;
  onDelete: () => void;
  onChecks: (assertions: WorkflowAssertion[]) => void;
  onScenario: (scenario: string) => Promise<void>;
  onMocks: (mocks: WorkflowSimulation["action_mocks"]) => Promise<void>;
  onCompare?: () => void;
}) {
  const latest = simResultOf(sim, as);
  const [past, setPast] = useState<{ id: string; result: WorkflowSimResult } | null>(null);
  const viewing = busy ? null : past;
  const result = viewing ? viewing.result : latest;
  const shown = busy ? (live?.result ?? null) : result;
  const status = statusOfSim(sim, as, busy, live);
  const history = simHistoryOf(sim, as);
  const when = busy ? "" : ranWhen(latest, history);

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <DetailBar
        name={sim.name}
        busy={busy}
        onBack={onBack}
        onDelete={onDelete}
        onCompare={onCompare}
        onRun={() => {
          setPast(null);
          onRun();
        }}
      />
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
              workflow={workflow}
              sim={sim}
              as={as}
              viewing={viewing?.id ?? null}
              onView={(r, id) => setPast(r && id ? { id, result: r } : null)}
            />
          )}
          <Scenario key={`${sim.id}:${sim.scenario}`} sim={sim} readOnly={busy} onSave={onScenario} />
          <Mocks
            key={`${sim.id}:${JSON.stringify(sim.action_mocks)}`}
            sim={sim}
            steps={steps}
            readOnly={busy}
            onSave={onMocks}
          />
          <Resize>
            <Swap
              view={busy ? "live" : (viewing?.id ?? `latest:${latest?.ran_at ?? ""}`)}
              forward
              className="flex flex-col gap-6"
            >
              <Ran sim={sim} result={result} running={busy} live={live} steps={steps} sources={sources} />
              <Checks
                sim={sim}
                result={shown}
                steps={steps}
                sources={sources}
                readOnly={busy || viewing !== null}
                onChange={onChecks}
              />
            </Swap>
          </Resize>
        </div>
      </ScrollArea>
    </div>
  );
}

/*
 * simOfTry reads one of Operator's tries as a simulation, so it shows
 * in the list and opens the way a saved one does.
 */
function simOfTry(t: OperatorTry, workflow: string): WorkflowSimulation {
  const message = t.message ?? "";

  return {
    id: t.id,
    object: "workflow_simulation",
    workflow_id: workflow,
    name: nameFrom(message) || "Operator's try",
    source_conversation_id: null,
    persona: {},
    messages: message ? [{ text: message }] : [],
    action_mocks: {},
    assertions: [],
    scenario: t.situation ?? "",
    draft_result: t.run.result ?? null,
    live_result: null,
    draft_history: null,
    live_history: null,
    simulated_at: null,
    created_at: "",
    updated_at: "",
  };
}

/*
 * TryDetail is one of Operator's tries, opened the way a simulation is:
 * where it stands, the customer's situation, the chat as it plays and
 * the steps. A try is not saved, so it has nothing to run again,
 * delete, check or look back on.
 */
function TryDetail({
  sim,
  busy,
  live,
  steps,
  sources,
  onBack,
}: {
  sim: WorkflowSimulation;
  busy: boolean;
  live?: LiveWfRun;
  steps: Steps;
  sources: ChipSources;
  onBack: () => void;
}) {
  const status = statusOfTry(sim, busy, live);

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <DetailBar name={sim.name} busy={busy} onBack={onBack} />
      <ScrollArea>
        <div className="flex flex-col gap-6 px-4 pt-2 pb-10">
          <span className="flex items-center gap-2">
            <Chip variant="caption" color={status.color} className="py-0.5">
              {status.label}
            </Chip>
            <span className={hint}>On the suggested version</span>
          </span>
          <Scenario sim={sim} readOnly onSave={async () => undefined} />
          <div className="flex flex-col gap-6">
            <Ran sim={sim} result={sim.draft_result} running={busy} live={live} steps={steps} sources={sources} />
          </div>
        </div>
      </ScrollArea>
    </div>
  );
}

/*
 * useOpensTries opens each try of Operator's as it starts, the way a new
 * simulation opens as it runs.
 */
function useOpensTries(tries: OperatorTry[], open: (id: string) => void) {
  const seen = useRef(new Set<string>());

  // biome-ignore lint/correctness/useExhaustiveDependencies: reacts to new tries only.
  useEffect(() => {
    const fresh = tries.find((t) => !seen.current.has(t.id));
    for (const t of tries) {
      seen.current.add(t.id);
    }
    if (fresh && !fresh.done) {
      open(fresh.id);
    }
  }, [tries]);
}

/* TryRows lists Operator's tries as simulations, the newest first. */
function TryRows({
  tries,
  workflow,
  onOpen,
}: {
  tries: OperatorTry[];
  workflow: string;
  onOpen: (id: string) => void;
}) {
  return (
    <>
      {[...tries].reverse().map((t) => {
        const sim = simOfTry(t, workflow);

        return (
          <SimRow
            key={t.id}
            sim={sim}
            as="draft"
            busy={!t.done}
            live={t.run}
            differs={false}
            status={statusOfTry(sim, !t.done, t.run)}
            onOpen={() => onOpen(t.id)}
          />
        );
      })}
    </>
  );
}

/* What each copy is called after "on the". */
const copyWords: Record<AnswerAs, string> = { draft: "draft", live: "live version" };

/* OpenedTry is the try that is open, or nothing; its steps are the suggestion's when there is one. */
function OpenedTry({
  t,
  workflow,
  steps,
  suggested,
  sources,
  onBack,
}: {
  t: OperatorTry | undefined;
  workflow: string;
  steps: Steps;
  suggested?: Steps;
  sources: ChipSources;
  onBack: () => void;
}) {
  if (!t) {
    return null;
  }

  return (
    <TryDetail
      key={t.id}
      sim={simOfTry(t, workflow)}
      busy={!t.done}
      live={t.run}
      steps={suggested ?? steps}
      sources={sources}
      onBack={onBack}
    />
  );
}

/*
 * nameFrom names a simulation after the customer's first message: its
 * first line, cut at a word near 60 characters.
 */
function nameFrom(message: string): string {
  const line = message.trim().split("\n")[0] ?? "";
  if (line.length <= 60) {
    return line;
  }
  const cut = line.slice(0, 60);

  return `${cut.slice(0, Math.max(cut.lastIndexOf(" "), 40))}…`;
}

/*
 * NewSimulation is the composer under the list: the customer's first
 * message, and through the plus, who writes it. Sending saves the
 * simulation; its situation is written from the message and can be
 * changed once it is open.
 */
function NewSimulation({
  workspace,
  workflow,
  onMade,
}: {
  workspace: string;
  workflow: string;
  onMade: (sim: WorkflowSimulation) => void;
}) {
  const [message, setMessage] = useState("");
  const [persona, setPersona] = useState<SimPersona | null>(null);
  const [saving, setSaving] = useState(false);
  const who = persona?.name || persona?.email || "";

  const save = async () => {
    const text = message.trim();
    if (text === "" || saving) {
      return;
    }
    setSaving(true);
    try {
      const made = await createWorkflowSimulation(workspace, workflow, {
        name: nameFrom(text),
        messages: [{ text }],
        ...(persona ? { persona } : {}),
      });
      setMessage("");
      onMade(made);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "The simulation was not saved.");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="shrink-0 px-4 pb-4">
      <PreviewComposer
        value={message}
        onValueChange={setMessage}
        onSubmit={() => void save()}
        onStop={() => undefined}
        busy={saving}
        placeholder="Write what the customer says first"
        canSubmit={false}
        added={
          who ? (
            <div className="flex flex-wrap gap-1.5 px-0.5 pt-0.5">
              <Added
                mark={<Avatar size="md" color={avatarColor(who)} initials={who.charAt(0).toUpperCase()} />}
                title={who}
                detail={persona?.name && persona.email ? `Writing as ${persona.email}` : "Writing as this customer"}
                onRemove={() => setPersona(null)}
              />
            </div>
          ) : null
        }
        menu={
          <PreviewAddMenu
            workspace={workspace}
            replayable={false}
            onCustomer={setPersona}
            onConversation={() => undefined}
          />
        }
      />
    </div>
  );
}

function disagrees(sim: WorkflowSimulation): boolean {
  return sim.draft_result !== null && sim.live_result !== null && sim.draft_result.passed !== sim.live_result.passed;
}

/* summaryOf says how many simulations pass, are flaky, fail or have not run on the copy. */
function summaryOf(list: WorkflowSimulation[], as: AnswerAs): string {
  const standings = list.map((sim) => simHistoryOf(sim, as)?.state ?? "none");
  const count = (state: string) => standings.filter((s) => s === state).length;

  return [
    `${count("passing")} of ${list.length} passing`,
    count("flaky") > 0 ? `${count("flaky")} flaky` : "",
    count("failing") > 0 ? `${count("failing")} failing` : "",
    count("none") > 0 ? `${count("none")} not run` : "",
  ]
    .filter(Boolean)
    .join(", ");
}

/* SimRow is one simulation in the list: its name, first message, how it stands, and whether live differs. */
function SimRow({
  sim,
  as,
  busy,
  live,
  differs,
  onOpen,
  status = statusOfSim(sim, as, busy, live),
}: {
  sim: WorkflowSimulation;
  as: AnswerAs;
  busy: boolean;
  live?: LiveWfRun;
  differs: boolean;
  onOpen: () => void;
  /* Where it stands, when not read from its saved runs, such as one of Operator's tries. */
  status?: { label: string; color: ChipColor };
}) {
  const history = simHistoryOf(sim, as);
  const first = sim.messages[0]?.text ?? "";

  return (
    <li>
      <button
        type="button"
        onClick={onOpen}
        className="flex w-full cursor-pointer items-center gap-3 rounded-xl px-2.5 py-2 text-start outline-none transition-colors duration-150 hover:bg-background-primary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <span className="flex min-w-0 flex-1 flex-col">
          <span className="truncate text-body-regular text-text-primary">{sim.name}</span>
          <span className="flex min-w-0 items-center gap-2">
            {history && !busy ? <Dots history={history} /> : null}
            {first && !first.startsWith(sim.name) ? <span className={cx(hint, "truncate")}>“{first}”</span> : null}
          </span>
        </span>
        {differs ? (
          <Chip variant="caption" color="orange" className="shrink-0 py-0.5">
            Live differs
          </Chip>
        ) : null}
        <Chip variant="caption" color={status.color} className="shrink-0 py-0.5">
          {status.label}
        </Chip>
        <RiArrowRightSLine aria-hidden className="size-5 shrink-0 text-foreground-icon-tertiary rtl:-scale-x-100" />
      </button>
    </li>
  );
}

/* NoSimulations is the panel before the workflow has any simulation. */
function NoSimulations() {
  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-3 px-6 text-center">
      <span className="flex flex-col gap-1">
        <span className="text-body-medium text-text-primary">No simulations yet</span>
        <span className={hint}>
          A simulation is a chat this workflow must keep handling the way you mean. Write what a customer says first,
          below. An AI customer plays the rest; nothing is sent.
        </span>
      </span>
    </div>
  );
}

export function WorkflowSimulations({
  workspace,
  workflow,
  flush,
  tries = [],
  suggested,
}: {
  workspace: string;
  workflow: string;
  /* Saves the builder's unsaved changes, so a run on the draft runs what is on screen. */
  flush?: () => Promise<void>;
  /* Operator's tries of this workflow, listed and opened like the saved simulations. */
  tries?: OperatorTry[];
  /* The steps of Operator's suggestion, which its tries play. */
  suggested?: Steps;
}) {
  const queryClient = useQueryClient();
  const { data: list = [] } = useQuery(workflowSimulationsQuery(workspace, workflow));
  const wf = useQuery(workflowQuery(workspace, workflow)).data;
  const sources = useChipSources(workspace, "");
  const [as, setAs] = useState<AnswerAs>("draft");
  const [openId, setOpenId] = useState<string | null>(null);
  const [running, setRunning] = useState<string[]>([]);
  const [deleting, setDeleting] = useState<WorkflowSimulation | null>(null);
  const [comparing, setComparing] = useState(false);
  const live = useWorkflowLiveRuns(workspace);
  const refresh = () => queryClient.invalidateQueries(workflowSimulationsQuery(workspace, workflow));
  const steps: Steps = wf?.spec?.steps ?? [];
  const hasLive = Boolean(wf?.live_version_id);
  const copy = hasLive ? as : "draft";
  const opened = list.find((sim) => sim.id === openId);
  const openedTry = tries.find((t) => t.id === openId);
  useOpensTries(tries, setOpenId);
  const summary = summaryOf(list, copy);

  const run = async (ids?: string[]) => {
    setRunning(ids ?? list.map((sim) => sim.id));
    try {
      if (copy === "draft" && flush) {
        await flush();
      }
      const ran = await runWorkflowSimulations(workspace, workflow, {
        simulation_ids: ids,
        version: copy,
        stream_id: live.begin(),
      });
      await refresh();
      const broken = ran.map((sim) => [sim, simResultOf(sim, copy)] as const).find(([, r]) => r?.error);
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

  const save = async (
    sim: WorkflowSimulation,
    change: Parameters<typeof updateWorkflowSimulation>[3],
    failed: string,
  ) => {
    try {
      await updateWorkflowSimulation(workspace, workflow, sim.id, change);
      await refresh();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : failed);
    }
  };

  const detail = opened ? (
    <Detail
      key={`${opened.id}:${copy}`}
      workspace={workspace}
      workflow={workflow}
      sim={opened}
      as={copy}
      steps={steps}
      sources={sources}
      busy={running.includes(opened.id)}
      live={live.runs[opened.id]}
      onBack={() => setOpenId(null)}
      onRun={() => void run([opened.id])}
      onDelete={() => setDeleting(opened)}
      onChecks={(assertions) => void save(opened, { assertions }, "The check was not saved.")}
      onScenario={(scenario) => save(opened, { scenario }, "The customer's situation was not saved.")}
      onMocks={(mocks) => save(opened, { action_mocks: mocks }, "The answers were not saved.")}
      onCompare={hasLive ? () => setComparing(true) : undefined}
    />
  ) : (
    <OpenedTry
      t={openedTry}
      workflow={workflow}
      steps={steps}
      suggested={suggested}
      sources={sources}
      onBack={() => setOpenId(null)}
    />
  );

  const all = (
    <>
      <div className="flex h-11 shrink-0 items-center justify-between gap-2 ps-4 pe-3">
        <p className={cx(hint, "min-w-0 truncate tabular-nums")}>
          {summary} on the {copyWords[copy]}
        </p>
        <span className="flex shrink-0 items-center gap-1">
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
        </span>
      </div>
      <ScrollArea>
        <ul className={cx("mx-4 mt-1 mb-6 flex flex-col gap-0.5 p-1", card)}>
          <TryRows tries={tries} workflow={workflow} onOpen={setOpenId} />
          {list.map((sim) => (
            <SimRow
              key={sim.id}
              sim={sim}
              as={copy}
              busy={running.includes(sim.id)}
              live={live.runs[sim.id]}
              differs={hasLive && disagrees(sim)}
              onOpen={() => setOpenId(sim.id)}
            />
          ))}
        </ul>
      </ScrollArea>
    </>
  );

  const listView = list.length + tries.length === 0 ? <NoSimulations /> : all;
  const showing = (opened ?? openedTry)?.id;

  return (
    <div className="relative flex min-h-0 flex-1 flex-col overflow-hidden">
      {hasLive && !showing ? (
        <div className="shrink-0 px-4 pt-3">
          <SegmentedControl
            aria-label="Run against"
            selectedKeys={new Set([copy])}
            onSelectionChange={(keys) => setAs([...keys][0] === "live" ? "live" : "draft")}
          >
            {copies.map(([id, label]) => (
              <SegmentedControlItem key={id} id={id}>
                {label}
              </SegmentedControlItem>
            ))}
          </SegmentedControl>
        </div>
      ) : null}
      <Swap view={showing ?? "list"} forward={showing !== undefined} className="flex min-h-0 flex-1 flex-col">
        {opened || openedTry ? (
          detail
        ) : (
          <>
            {listView}
            <NewSimulation
              workspace={workspace}
              workflow={workflow}
              onMade={(sim) => {
                void refresh().then(() => {
                  setOpenId(sim.id);
                  void run([sim.id]);
                });
              }}
            />
          </>
        )}
      </Swap>
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

          return sim ? deleteWorkflowSimulation(workspace, workflow, sim.id).then(refresh) : undefined;
        }}
      />
      {comparing && opened ? (
        <WorkflowSimCompare
          workspace={workspace}
          workflow={workflow}
          sim={opened}
          steps={steps}
          sources={sources}
          flush={flush}
          onClose={() => setComparing(false)}
        />
      ) : null}
    </div>
  );
}
