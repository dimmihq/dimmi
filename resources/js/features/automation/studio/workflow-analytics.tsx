/*
 * A workflow's Analytics, laid out like a procedure's. Pick a range and
 * a version, and optionally a second version to put beside it. Numbers
 * lead: runs that started, how many the customer wrote during, how many
 * completed, went to a person, failed, and reached the goal when the
 * workflow has one. Under them, every step in reading order with how
 * many runs reached it, wait there now, ended there and failed there,
 * so the drop-off shows at a glance. Only live runs that did something
 * count. A range with no runs says so once and shows nothing else.
 */
import { useQuery } from "@tanstack/react-query";
import { type ReactNode, useState } from "react";

import { StatCards } from "@/components/application/dashboard/stat-cards";
import { Chip } from "@/components/base/badges/chip";
import { Meter } from "@/components/base/meter/meter";
import { Skeleton } from "@/components/base/skeleton/skeleton";
import { Action } from "@/features/settings/controls";
import { Row, Section } from "@/features/settings/layout";
import {
  type SpecStep,
  type WorkflowMetrics,
  workflowMetricsQuery,
  workflowQuery,
  workflowVersionsQuery,
} from "@/lib/api/workflows";
import { formatNumber } from "@/lib/i18n/format";

import type { Choices } from "../choices";
import { Pick } from "../pick";
import { daysBack, goalWords, type PlacedStep, placeSteps, rate, stepLabel } from "./workflow-numbers";

const dayChoices: Choices = [
  ["7", "Last 7 days"],
  ["30", "Last 30 days"],
  ["90", "Last 90 days"],
];

const tiles = "grid grid-cols-2 gap-2 @xl:grid-cols-3";
const line = "flex min-h-11 items-center gap-3 border-b border-separator-border py-2.5 pe-3 last:border-b-0";
const figure = "shrink-0 text-end text-body-2-medium text-text-primary tabular-nums";

function Quiet({ children }: { children: ReactNode }) {
  return <p className="py-3 pe-3 text-body-2-regular text-text-tertiary">{children}</p>;
}

/* Figure is one number the tiles and the compare rows show. */
interface Figure {
  label: string;
  hint: string;
  value: (m: WorkflowMetrics) => number;
  /* The total a rate is of; the count itself when left out. */
  of?: (m: WorkflowMetrics) => number;
}

const figures: Figure[] = [
  { label: "Triggered", hint: "Live runs that started in the range and did something.", value: (m) => m.triggered },
  {
    label: "Engaged",
    hint: "Runs the customer wrote during, by typing or tapping a button.",
    value: (m) => m.engaged,
    of: (m) => m.triggered,
  },
  {
    label: "Completed",
    hint: "Runs that reached the end of a path.",
    value: (m) => m.completed,
    of: (m) => m.triggered,
  },
  {
    label: "Handed over",
    hint: "Runs that assigned the conversation, escalated it, or whose agent step handed it to a person.",
    value: (m) => m.handed_over,
    of: (m) => m.triggered,
  },
  {
    label: "Failed",
    hint: "Runs that failed, or in which a call, an action or a classify step failed.",
    value: (m) => m.failed,
    of: (m) => m.triggered,
  },
];

const goalFigure: Figure = {
  label: "Reached the goal",
  hint: "Runs whose conversation met the workflow's goal inside its window.",
  value: (m) => m.goal_met ?? 0,
  of: (m) => m.triggered,
};

function shown(m: WorkflowMetrics, f: Figure): string {
  return f.of ? rate(f.value(m), f.of(m)) : formatNumber(f.value(m));
}

function Tiles({ m, against, name }: { m: WorkflowMetrics; against?: WorkflowMetrics; name: string }) {
  const list = m.goal ? [...figures, goalFigure] : figures;

  return (
    <StatCards
      variant="compact"
      columns={2}
      className={`gap-2 lg:grid-cols-3`}
      stats={list.map((f) => ({
        label: f.label,
        hint: f.hint,
        value: shown(m, f),
        caption: against
          ? `${name} ${shown(against, f)}`
          : f.of
            ? `${formatNumber(f.value(m))} of ${formatNumber(f.of(m))}`
            : "Runs",
      }))}
    />
  );
}

type OpenFailures = ((step?: string) => void) | undefined;

function StepLine({
  placed,
  m,
  most,
  onOpenFailures,
}: {
  placed: PlacedStep;
  m: WorkflowMetrics;
  most: number;
  onOpenFailures: OpenFailures;
}) {
  const s = m.steps[placed.step.id];
  const reached = s?.reached ?? 0;
  const detail = [
    s?.waiting ? `${formatNumber(s.waiting)} here now` : "",
    s?.dropped ? `${formatNumber(s.dropped)} ended here (${rate(s.dropped, reached)})` : "",
  ]
    .filter(Boolean)
    .join(" · ");

  return (
    <div className={line} style={{ paddingInlineStart: `${placed.depth * 16}px` }}>
      <span className="w-12 shrink-0 text-body-2-medium text-text-tertiary tabular-nums">{placed.number}</span>
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-body-2-regular text-text-primary" title={stepLabel(placed.step)}>
          {stepLabel(placed.step)}
        </span>
        <span className="truncate text-caption-1-medium text-text-tertiary tabular-nums">
          {detail || placed.under || "Not yet reached"}
        </span>
      </span>
      {s?.failed ? (
        onOpenFailures ? (
          <button
            type="button"
            onClick={() => onOpenFailures(placed.step.id)}
            className="cursor-pointer rounded-md outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring"
          >
            <Chip variant="caption" color="rose">
              {`${formatNumber(s.failed)} failed`}
            </Chip>
          </button>
        ) : (
          <Chip variant="caption" color="rose">
            {`${formatNumber(s.failed)} failed`}
          </Chip>
        )
      ) : null}
      <Meter aria-label={`Step ${placed.number} reached`} value={reached} maxValue={most} size="sm" className="w-24" />
      <span className={`${figure} w-12`}>{formatNumber(reached)}</span>
    </div>
  );
}

/*
 * Steps lists every step in reading order, then any step that ran but
 * is no longer in the workflow.
 */
function Steps({
  m,
  placed,
  onOpenFailures,
}: {
  m: WorkflowMetrics;
  placed: PlacedStep[];
  onOpenFailures: OpenFailures;
}) {
  const most = Math.max(1, ...Object.values(m.steps).map((s) => s.reached));
  const known = new Set(placed.map((p) => p.step.id));
  const gone = Object.keys(m.steps).filter((id) => !known.has(id));

  return (
    <Section title="Steps" description="How many runs reached each step, and where they stopped.">
      {placed.length === 0 && gone.length === 0 ? <Quiet>No steps.</Quiet> : null}
      {placed.map((p) => (
        <StepLine key={p.step.id} placed={p} m={m} most={most} onOpenFailures={onOpenFailures} />
      ))}
      {gone.map((id) => (
        <StepLine
          key={id}
          placed={{ step: { id, kind: "" }, number: "–", under: "No longer in the workflow", depth: 0 }}
          m={m}
          most={most}
          onOpenFailures={onOpenFailures}
        />
      ))}
    </Section>
  );
}

/*
 * Compare puts each number of two versions side by side.
 */
function Compare({ a, b, nameA, nameB }: { a: WorkflowMetrics; b: WorkflowMetrics; nameA: string; nameB: string }) {
  const list = a.goal || b.goal ? [...figures, goalFigure] : figures;

  return (
    <Section title="Compare" description={`${nameA} against ${nameB}, over the same days.`}>
      {list.map((f) => (
        <Row key={f.label} label={f.label}>
          <span className="flex items-center gap-3 text-body-2-medium tabular-nums">
            <span className="text-text-primary" title={nameA}>
              {shown(a, f)}
            </span>
            <span className="text-text-tertiary" title={nameB}>
              {shown(b, f)}
            </span>
          </span>
        </Row>
      ))}
    </Section>
  );
}

/*
 * Breakdown is everything under the numbers, shown once the workflow ran
 * in the range.
 */
function Breakdown({
  m,
  b,
  names,
  steps,
  onOpenFailures,
}: {
  m: WorkflowMetrics;
  b?: WorkflowMetrics;
  names: [string, string];
  steps: SpecStep[];
  onOpenFailures: OpenFailures;
}) {
  return (
    <>
      {b ? <Compare a={m} b={b} nameA={names[0]} nameB={names[1]} /> : null}
      <Section title="Goal">
        <Row
          label={goalWords(m.goal)}
          description={
            m.goal
              ? `${formatNumber(m.goal_met ?? 0)} of ${formatNumber(m.triggered)} runs reached it.`
              : "Set a goal in the workflow's settings to count the runs that worked."
          }
        >
          {m.goal ? <span className={figure}>{rate(m.goal_met ?? 0, m.triggered)}</span> : null}
        </Row>
      </Section>
      {m.failed > 0 ? (
        <Section title="Failures">
          <Row
            label={`${formatNumber(m.failed)} runs failed`}
            description="A call, an action, a classify step or the run itself failed."
          >
            {onOpenFailures ? <Action onPress={() => onOpenFailures()}>See them</Action> : null}
          </Row>
        </Section>
      ) : null}
      <Steps m={m} placed={placeSteps(steps)} onOpenFailures={onOpenFailures} />
    </>
  );
}

export function WorkflowAnalytics({
  workspace,
  workflow,
  onOpenFailures,
}: {
  workspace: string;
  workflow: string;
  /* Opens the failure list, narrowed to a step when one is given. */
  onOpenFailures?: (step?: string) => void;
}) {
  const [days, setDays] = useState("30");
  const [version, setVersion] = useState("0");
  const [against, setAgainst] = useState("0");
  const from = daysBack(Number(days));
  const versions = useQuery(workflowVersionsQuery(workspace, workflow)).data ?? [];
  const spec = useQuery(workflowQuery(workspace, workflow)).data?.spec;
  const metrics = useQuery(workflowMetricsQuery(workspace, workflow, { from, version: Number(version) }));
  const other = useQuery({
    ...workflowMetricsQuery(workspace, workflow, { from, version: Number(against) }),
    enabled: against !== "0",
  });
  const m = metrics.data;
  const b = against !== "0" ? other.data : undefined;

  const versionChoices: Choices = [
    ["0", "Every version"],
    ...versions.map((v) => [String(v.number), `Version ${v.number}`] as const),
  ];
  const name = (key: string) => (key === "0" ? "Every version" : `Version ${key}`);
  const range = (dayChoices.find(([value]) => value === days)?.[1] ?? "").toLowerCase();
  const quiet = m ? m.triggered === 0 && Object.keys(m.steps).length === 0 : false;

  return (
    <div className="@container mx-auto flex w-full max-w-[760px] flex-col gap-8 px-6 pt-6 pb-16">
      <div className="flex flex-col gap-3">
        <div className="flex flex-wrap items-center gap-2">
          <Pick label="Range" choices={dayChoices} value={days} onChange={setDays} />
          <Pick label="Version" choices={versionChoices} value={version} onChange={setVersion} />
          {versions.length > 1 ? (
            <Pick
              label="Compare with"
              choices={[["0", "No comparison"], ...versionChoices.slice(1).filter(([v]) => v !== version)]}
              value={against}
              onChange={setAgainst}
            />
          ) : null}
        </div>
        {m ? (
          <Tiles m={m} against={b} name={name(against)} />
        ) : (
          <div className={tiles}>
            {["a", "b", "c", "d", "e", "f"].map((key) => (
              <Skeleton key={key} className="h-[100px] rounded-2xl" />
            ))}
          </div>
        )}
      </div>
      {m && quiet ? (
        <div className="flex flex-col items-center gap-1 rounded-2xl bg-background-secondary-default px-6 py-12 text-center">
          <span className="text-body-medium text-text-primary">No runs in the {range}</span>
          <span className="text-body-2-regular text-text-tertiary">
            Steps, drop-off and failures show here once this workflow runs on live conversations.
          </span>
        </div>
      ) : null}
      {m && !quiet ? (
        <Breakdown
          m={m}
          b={b}
          names={[name(version), name(against)]}
          steps={spec?.steps ?? []}
          onOpenFailures={onOpenFailures}
        />
      ) : null}
    </div>
  );
}
