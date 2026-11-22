/*
 * How a procedure did over the last 7, 30 or 90 days, for every version
 * or one. Four numbers lead: the conversations it ran in, how many dimmi
 * resolved (counted as the dimmi agent report counts them), how many
 * went to the team and how customers rated them. Under them, how far
 * conversations got through each step and which side each rule took,
 * why conversations were handed over, how often the guardrails stepped
 * in, how its actions fared, and the runs behind the numbers, each
 * opening its conversation. A range with no conversations says so once
 * and shows nothing else.
 */
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";

import { StatCards } from "@/components/application/dashboard/stat-cards";
import { Chip, type ChipColor } from "@/components/base/badges/chip";
import { Meter, type MeterTone } from "@/components/base/meter/meter";
import { Skeleton } from "@/components/base/skeleton/skeleton";
import { Section } from "@/features/settings/layout";
import { type ProcedureMetrics, type ProcedureStep, procedureMetricsQuery, versionsQuery } from "@/lib/api/procedures";
import { countOf, formatNumber, formatRelative } from "@/lib/i18n/format";

import type { Choices } from "./choices";
import { Pick } from "./pick";
import { type ChipSources, labelOf } from "./procedure-writer/procedure-suggestions";
import { stepNumbers } from "./procedures";
import { stepTitle } from "./step-words";
import type { useProcedureDraft } from "./use-procedure-draft";

type Draft = ReturnType<typeof useProcedureDraft>;

const dayChoices: Choices = [
  ["7", "Last 7 days"],
  ["30", "Last 30 days"],
  ["90", "Last 90 days"],
];

const guardWords: Record<string, string> = {
  prerequisite_blocked: "Actions called out of order",
  compliance_retry: "Replies sent back for a skipped action",
  compliance_failed_handoff: "Handovers after a skipped action",
  compliance_warned: "Skipped actions only noted",
  awaiting_input: "Asked the customer for an action's input",
};

const outcomes: Record<string, { label: string; color: ChipColor }> = {
  answered: { label: "Answered", color: "lime" },
  resolved: { label: "Resolved", color: "lime" },
  handoff: { label: "Handed over", color: "orange" },
};

function percent(rate: number | null): string {
  return rate === null ? "–" : `${Math.round(rate * 100)}%`;
}

function words(value: string): string {
  const spaced = value.replaceAll("_", " ");

  return spaced.charAt(0).toUpperCase() + spaced.slice(1);
}

/*
 * stepsById finds every step of a procedure, nested ones too.
 */
function stepsById(steps: ProcedureStep[], found = new Map<string, ProcedureStep>()): Map<string, ProcedureStep> {
  for (const step of steps) {
    found.set(step.id, step);
    stepsById(step.then_steps ?? [], found);
    stepsById(step.else_steps ?? [], found);
  }

  return found;
}

const tiles = "grid grid-cols-2 gap-2 @xl:grid-cols-4";
const line = "flex min-h-11 items-center gap-3 border-b border-separator-border py-2.5 pe-3 last:border-b-0";
const figure = "shrink-0 text-end text-body-2-medium text-text-primary tabular-nums";

/*
 * Share is one line of a breakdown: what it counts, a bar for its part
 * of the whole, and the count.
 */
function Share({ label, count, of, tone }: { label: string; count: number; of: number; tone?: MeterTone }) {
  return (
    <div className="flex flex-col gap-2 border-b border-separator-border py-3 pe-3 last:border-b-0">
      <div className="flex items-start justify-between gap-3">
        <span className="min-w-0 text-body-2-regular text-text-primary">{label}</span>
        <span className={figure}>{count}</span>
      </div>
      <Meter aria-label={label} value={count} maxValue={Math.max(of, 1)} size="sm" tone={tone} />
    </div>
  );
}

function Quiet({ children }: { children: ReactNode }) {
  return <p className="py-3 pe-3 text-body-2-regular text-text-tertiary">{children}</p>;
}

function Tiles({ m }: { m: ProcedureMetrics }) {
  return (
    <StatCards
      variant="compact"
      className="gap-2"
      stats={[
        {
          label: "Conversations",
          hint: "Conversations in which dimmi followed this procedure.",
          value: formatNumber(m.matched_conversations),
          caption: "Followed it",
        },
        {
          label: "Resolved by AI",
          hint: `Counted the way the dimmi agent report counts: ${m.ai_confirmed} confirmed by the customer and ${m.ai_assumed} assumed, of the ${m.ai_involved} the AI was in.`,
          value: percent(m.ai_resolution_rate),
          caption: `${m.ai_resolved} of ${m.ai_involved}`,
        },
        {
          label: "Handed over",
          hint: "Conversations that went to the team while following this procedure.",
          value: percent(m.handover_rate),
          caption: `${m.handed_off} to the team`,
        },
        {
          label: "CSAT",
          hint: "Of the customers who rated the conversation, how many rated it 4 or 5.",
          value: percent(m.csat.positive_rate),
          caption: m.csat.mean === null ? "No ratings" : `${m.csat.mean.toFixed(1)} of 5 · ${m.csat.rated} rated`,
        },
      ]}
    />
  );
}

type StepCount = ProcedureMetrics["steps"][number];

function StepLine({ step: s, number, title, most }: { step: StepCount; number: string; title: string; most: number }) {
  const rule = s.took_then + s.took_else > 0;

  return (
    <div className={line}>
      <span className="w-7 shrink-0 text-body-2-medium text-text-tertiary tabular-nums">{number}</span>
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-body-2-regular text-text-primary" title={title}>
          {title}
        </span>
        <span className="truncate text-caption-1-medium text-text-tertiary tabular-nums">
          {rule ? `If so ${s.took_then} · if not ${s.took_else}` : `${s.done} of ${s.reached} finished`}
        </span>
      </span>
      <Meter aria-label={`Step ${number} reached`} value={s.reached} maxValue={most} size="sm" className="w-24" />
      <span className={`${figure} w-10`}>{rule ? s.reached : percent(s.completion_rate)}</span>
    </div>
  );
}

function RunLine({ run: r }: { run: ProcedureMetrics["runs"][number] }) {
  const outcome = outcomes[r.outcome ?? ""] ?? {
    label: r.outcome ? words(r.outcome) : "Running",
    color: "neutral" as const,
  };
  const guardEvents = r.procedure_trace?.guard_events?.length ?? 0;
  const detail = [
    r.handoff_reason ? words(r.handoff_reason) : "",
    guardEvents > 0 ? countOf(guardEvents, "guardrail event", "guardrail events") : "",
    r.rating ? `Rated ${r.rating} of 5` : "",
  ]
    .filter(Boolean)
    .join(" · ");

  return (
    <div className={line}>
      <span className="w-32 shrink-0 truncate text-body-2-regular text-text-primary">
        {r.conversation_id ? (
          <Link
            to="/d/inbox/$view/$conversation"
            params={{ view: "all", conversation: r.conversation_id }}
            className="hover:underline"
          >
            Conversation #{r.conversation_number}
          </Link>
        ) : (
          "Test run"
        )}
      </span>
      <Chip variant="caption" color={outcome.color}>
        {outcome.label}
      </Chip>
      <span className="min-w-0 flex-1 truncate text-body-2-regular text-text-tertiary" title={detail}>
        {detail}
      </span>
      <span className="shrink-0 text-caption-1-medium text-text-tertiary">
        {formatRelative(r.created_at, { style: "short" })}
      </span>
    </div>
  );
}

/*
 * Breakdown is everything under the four numbers, shown once the
 * procedure ran in the range.
 */
function Breakdown({ m, draft, sources }: { m: ProcedureMetrics; draft: Draft; sources: ChipSources }) {
  const numbers = stepNumbers(draft.definition.steps);
  const steps = stepsById(draft.definition.steps);
  const guards = Object.entries(m.guardrails).sort((a, b) => b[1] - a[1]);
  const guardTotal = guards.reduce((sum, [, count]) => sum + count, 0);
  const reached = Math.max(1, ...m.steps.map((s) => s.reached));
  /* In the procedure's own order; steps since removed go last. */
  const order = [...numbers.keys()];
  const place = (id: string) => (order.includes(id) ? order.indexOf(id) : order.length);
  const ordered = [...m.steps].sort((a, b) => place(a.step_id) - place(b.step_id));

  return (
    <>
      <Section title="Steps" description="How many conversations reached each step, and how many finished it.">
        {m.steps.length === 0 ? <Quiet>No step was reached.</Quiet> : null}
        {ordered.map((s) => (
          <StepLine
            key={s.step_id}
            step={s}
            number={numbers.get(s.step_id) ?? "–"}
            title={stepTitle(steps.get(s.step_id), sources)}
            most={reached}
          />
        ))}
      </Section>
      <div className="grid grid-cols-1 gap-8 @xl:grid-cols-2 @xl:gap-4">
        <Section title="Handovers by reason">
          {m.handover_reasons.length === 0 ? <Quiet>No handovers.</Quiet> : null}
          {m.handover_reasons.map((h) => (
            <Share
              key={`${h.kind}:${h.reason}`}
              label={words(h.reason)}
              count={h.conversations}
              of={m.handed_off}
              tone="warning"
            />
          ))}
        </Section>
        <Section title="Guardrails">
          {guards.length === 0 ? <Quiet>The guardrails never stepped in.</Quiet> : null}
          {guards.map(([type, count]) => (
            <Share key={type} label={guardWords[type] ?? words(type)} count={count} of={guardTotal} />
          ))}
        </Section>
      </div>
      <Section title="Actions">
        {m.actions.length === 0 ? <Quiet>No actions ran.</Quiet> : null}
        {m.actions.map((a) => (
          <div key={a.action_id} className={line}>
            <span className="flex min-w-0 flex-1 flex-col">
              <span className="truncate text-body-2-regular text-text-primary">
                {labelOf(sources, { kind: "action", ref: a.action_id }) || a.name || a.action_id}
              </span>
              <span className="truncate text-caption-1-medium text-text-tertiary tabular-nums">
                {countOf(a.calls, "call", "calls")} · {Math.round(a.mean_latency_ms)} ms on average
              </span>
            </span>
            {a.failed > 0 ? (
              <Chip variant="caption" color="rose">
                {a.failed} failed · {percent(a.error_rate)}
              </Chip>
            ) : (
              <Chip variant="caption" color="lime">
                No failures
              </Chip>
            )}
          </div>
        ))}
      </Section>
      <Section title="Recent runs">
        {m.runs.length === 0 ? <Quiet>No runs.</Quiet> : null}
        {m.runs.slice(0, 20).map((r) => (
          <RunLine key={r.run_id} run={r} />
        ))}
      </Section>
    </>
  );
}

export function ProcedureAnalytics({
  workspace,
  draft,
  sources,
}: {
  workspace: string;
  draft: Draft;
  sources: ChipSources;
}) {
  const id = draft.server.id;
  const [days, setDays] = useState("30");
  const [version, setVersion] = useState("all");
  const versions = useQuery(versionsQuery(workspace, id)).data ?? [];
  const metrics = useQuery(procedureMetricsQuery(workspace, id, Number(days), version));
  const m = metrics.data;

  const range = (dayChoices.find(([value]) => value === days)?.[1] ?? "").toLowerCase();
  const quiet = m ? m.matched_conversations === 0 && m.runs.length === 0 : false;

  return (
    <div className="@container mx-auto flex w-full max-w-[760px] flex-col gap-8 px-6 pt-6 pb-16">
      <div className="flex flex-col gap-3">
        <div className="flex flex-wrap items-center gap-2">
          <span className="flex gap-2">
            <Pick label="Range" choices={dayChoices} value={days} onChange={setDays} />
            <Pick
              label="Version"
              choices={[
                ["all", "Every version"],
                ...versions.map((v) => [String(v.version), `Version ${v.version}`] as const),
              ]}
              value={version}
              onChange={setVersion}
            />
          </span>
        </div>
        {m ? (
          <Tiles m={m} />
        ) : (
          <div className={tiles}>
            {["a", "b", "c", "d"].map((key) => (
              <Skeleton key={key} className="h-[100px] rounded-2xl" />
            ))}
          </div>
        )}
      </div>
      {m && quiet ? (
        <div className="flex flex-col items-center gap-1 rounded-2xl bg-background-secondary-default px-6 py-12 text-center">
          <span className="text-body-medium text-text-primary">No conversations in the {range}</span>
          <span className="text-body-2-regular text-text-tertiary">
            {draft.server.status === "live"
              ? "Steps, handovers, guardrails, actions and runs show here once dimmi follows this procedure."
              : "Publish this procedure for dimmi to follow it. Its numbers show here."}
          </span>
        </div>
      ) : null}
      {m && !quiet ? <Breakdown m={m} draft={draft} sources={sources} /> : null}
    </div>
  );
}
