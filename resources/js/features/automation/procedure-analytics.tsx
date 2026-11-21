/*
 * How a procedure did over the last 7, 30 or 90 days, for every version
 * or one: the conversations it ran in, how many dimmi resolved (counted
 * as the dimmi agent report counts them), how many went to the team and
 * why, how customers rated them, how often the guardrails stepped in,
 * how far conversations got through each step and which side each rule
 * took, and how its actions fared. The runs behind the numbers open the
 * conversation. Operator can review the runs that went wrong and propose
 * fixes and tests.
 */
import { useQuery } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { toast } from "@/components/base/toast/toast";
import { Row, Section } from "@/features/settings/layout";
import { askOperator, startWorkspaceThread } from "@/lib/api/operator";
import { procedureMetricsQuery, versionsQuery } from "@/lib/api/procedures";

import type { Choices } from "./choices";
import { Pick } from "./pick";
import { stepNumbers } from "./procedures";
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

function percent(rate: number | null): string {
  return rate === null ? "–" : `${Math.round(rate * 100)}%`;
}

export function ProcedureAnalytics({ workspace, draft }: { workspace: string; draft: Draft }) {
  const id = draft.server.id;
  const navigate = useNavigate();
  const [days, setDays] = useState("30");
  const [version, setVersion] = useState("all");
  const [reviewing, setReviewing] = useState(false);
  const versions = useQuery(versionsQuery(workspace, id)).data ?? [];
  const metrics = useQuery(procedureMetricsQuery(workspace, id, Number(days), version));
  const m = metrics.data;
  const numbers = stepNumbers(draft.definition.steps);

  const review = async () => {
    setReviewing(true);
    try {
      const thread = await startWorkspaceThread(workspace);
      void askOperator(
        workspace,
        thread.id,
        `Review the recent runs of the procedure "${draft.name}" (${id}): call list_procedure_anomalies, read the ` +
          "traces of the runs that went wrong, and propose fixes to its working copy with regression tests made from " +
          "the failing conversations.",
      );
      toast.success("Operator is reviewing the runs. Its proposals wait for you on the Operator tab.");
      void navigate({ to: "/d/agent", search: { tab: "operator" } });
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Operator could not start.");
    } finally {
      setReviewing(false);
    }
  };

  return (
    <div className="mx-auto flex w-full max-w-[760px] flex-col gap-8 px-6 pt-8 pb-16">
      <Section
        title="Scorecard"
        description="Counted the way the dimmi agent report counts, so the numbers add up."
        loading={metrics.isPending}
        action={
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
        }
      >
        {m ? (
          <>
            <Row label="Conversations" description="That followed this procedure.">
              <span className="text-body-medium tabular-nums">{m.matched_conversations}</span>
            </Row>
            <Row
              label="AI resolution rate"
              description={`${m.ai_confirmed} confirmed and ${m.ai_assumed} assumed of ${m.ai_involved} the AI was in.`}
            >
              <span className="text-body-medium tabular-nums">{percent(m.ai_resolution_rate)}</span>
            </Row>
            <Row label="Handover rate" description={`${m.handed_off} handed to the team.`}>
              <span className="text-body-medium tabular-nums">{percent(m.handover_rate)}</span>
            </Row>
            <Row
              label="CSAT"
              description={`${m.csat.rated} rated, mean ${m.csat.mean === null ? "–" : m.csat.mean.toFixed(1)} of 5.`}
            >
              <span className="text-body-medium tabular-nums">{percent(m.csat.positive_rate)}</span>
            </Row>
          </>
        ) : null}
      </Section>
      {m ? (
        <>
          <Section title="Handovers by reason">
            {m.handover_reasons.length === 0 ? (
              <p className="px-3 py-2.5 text-body-2-regular text-text-secondary">No handovers.</p>
            ) : null}
            {m.handover_reasons.map((h) => (
              <Row key={h.reason} label={h.reason.replaceAll("_", " ")} description={h.kind}>
                <span className="tabular-nums">{h.conversations}</span>
              </Row>
            ))}
          </Section>
          <Section
            title="Guardrails"
            description="How often code stepped in. A step that keeps tripping them is unclear."
            action={
              <Button size="small" variant="secondary" loading={reviewing} onClick={() => void review()}>
                Review recent runs with Operator
              </Button>
            }
          >
            {Object.keys(m.guardrails).length === 0 ? (
              <p className="px-3 py-2.5 text-body-2-regular text-text-secondary">The guardrails never stepped in.</p>
            ) : null}
            {Object.entries(m.guardrails).map(([type, count]) => (
              <Row key={type} label={guardWords[type] ?? type}>
                <span className="tabular-nums">{count}</span>
              </Row>
            ))}
          </Section>
          <Section
            title="Steps"
            description="Conversations that reached each step, finished it, and which side each rule took."
          >
            {m.steps.map((s) => (
              <Row
                key={s.step_id}
                label={`Step ${numbers.get(s.step_id) ?? s.step_id}`}
                description={
                  s.took_then + s.took_else > 0
                    ? `Then ${s.took_then} · else ${s.took_else}`
                    : `${s.done} of ${s.reached} finished`
                }
              >
                <span className="tabular-nums">{percent(s.completion_rate)}</span>
              </Row>
            ))}
          </Section>
          <Section title="Actions">
            {m.actions.length === 0 ? (
              <p className="px-3 py-2.5 text-body-2-regular text-text-secondary">No actions ran.</p>
            ) : null}
            {m.actions.map((a) => (
              <Row
                key={a.action_id}
                label={a.name || a.action_id}
                description={`${a.calls} calls · ${a.failed} failed · ${Math.round(a.mean_latency_ms)} ms on average`}
              >
                <span className="tabular-nums">{percent(a.error_rate)}</span>
              </Row>
            ))}
          </Section>
          <Section title="Recent runs">
            {m.runs.slice(0, 20).map((r) => (
              <Row
                key={r.run_id}
                label={
                  r.conversation_id ? (
                    <Link
                      to="/d/inbox/$view/$conversation"
                      params={{ view: "all", conversation: r.conversation_id }}
                      className="underline"
                    >
                      Conversation #{r.conversation_number}
                    </Link>
                  ) : (
                    `Run ${r.run_id}`
                  )
                }
                description={[
                  r.outcome ?? "running",
                  r.handoff_reason ? `handed over: ${r.handoff_reason.replaceAll("_", " ")}` : "",
                  r.procedure_trace?.guard_events?.length
                    ? `${r.procedure_trace.guard_events.length} guardrail events`
                    : "",
                  r.rating ? `rated ${r.rating}` : "",
                ]
                  .filter(Boolean)
                  .join(" · ")}
              />
            ))}
          </Section>
        </>
      ) : null}
    </div>
  );
}
