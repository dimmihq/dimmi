/*
 * A workflow's failure inbox: the live runs of a range that failed, or
 * in which a call, an action, a classify step or the agent step failed,
 * newest first. Each names the step that failed and what it said, the
 * version that ran, and opens its conversation. Narrow it to one step,
 * and load more a page at a time.
 */
import { RiCheckboxCircleLine } from "@remixicon/react";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Action } from "@/features/settings/controls";
import { Section } from "@/features/settings/layout";
import { EmptyState } from "@/features/shell/page";
import { type WorkflowFailure, workflowFailuresPage, workflowQuery } from "@/lib/api/workflows";
import { formatRelative } from "@/lib/i18n/format";

import type { Choices } from "../choices";
import { Pick } from "../pick";
import { daysBack, placeSteps, stepLabel } from "./workflow-numbers";

const dayChoices: Choices = [
  ["7", "Last 7 days"],
  ["30", "Last 30 days"],
  ["90", "Last 90 days"],
];

const everyStep = "_every";
const line = "flex items-start gap-3 border-b border-separator-border py-3 pe-3 last:border-b-0";

const reasons: Record<string, string> = {
  ai_unavailable: "The AI could not be reached",
  definition_error: "The step could not run",
  internal_error: "Something went wrong on our side",
  step_budget: "Ran too many steps",
  host_blocked: "The address is not allowed",
  timeout: "Timed out",
  response_too_large: "The answer was too large",
};

/* codeOf reads the code a failing step kept, "" for none. */
function codeOf(detail: Record<string, unknown>): string {
  for (const key of ["error", "reason"]) {
    if (typeof detail[key] === "string") {
      return detail[key];
    }
  }

  return "";
}

const named = (code: string) => reasons[code] ?? code.replaceAll("_", " ");

/*
 * why says what went wrong, from the step's detail or the run's end.
 */
function why(f: WorkflowFailure): string {
  const detail = f.detail ?? {};
  const code = codeOf(detail);
  const parts = typeof detail.status === "number" ? [`Answered ${detail.status}`] : [];
  const outcome = f.outcome && f.outcome !== "failed" && f.outcome !== "none" ? f.outcome : "";
  const said = code || outcome;
  if (said) {
    parts.push(named(said));
  }
  if (parts.length === 0) {
    parts.push(f.end_reason ? named(f.end_reason) : (f.error?.message ?? "Failed"));
  }

  return parts.join(" · ");
}

function FailureLine({ f, label }: { f: WorkflowFailure; label: string }) {
  return (
    <div className={line}>
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="truncate text-body-2-medium text-text-primary" title={label}>
          {label}
        </span>
        <span className="text-body-2-regular text-text-secondary">{why(f)}</span>
        <span className="text-caption-1-medium text-text-tertiary">
          {[
            formatRelative(f.started_at),
            f.version_number ? `Version ${f.version_number}` : "",
            f.status === "failed" ? "The run stopped" : "The run went on",
          ]
            .filter(Boolean)
            .join(" · ")}
        </span>
      </div>
      {f.conversation_id ? (
        <Link
          to="/d/inbox/$view/$conversation"
          params={{ view: "all", conversation: f.conversation_id }}
          className="shrink-0 rounded-md text-body-2-medium text-text-primary underline underline-offset-2 outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          {f.conversation_number ? `Conversation #${f.conversation_number}` : "Open the conversation"}
        </Link>
      ) : (
        <Chip variant="caption" color="neutral">
          No conversation
        </Chip>
      )}
    </div>
  );
}

export function WorkflowFailures({
  workspace,
  workflow,
  step: initialStep = "",
}: {
  workspace: string;
  workflow: string;
  /* A step to narrow to at first. */
  step?: string;
}) {
  const [days, setDays] = useState("30");
  const [step, setStep] = useState(initialStep);
  const from = daysBack(Number(days));
  const spec = useQuery(workflowQuery(workspace, workflow)).data?.spec;
  const placed = placeSteps(spec?.steps ?? []);
  const labels = new Map(placed.map((p) => [p.step.id, `Step ${p.number} · ${stepLabel(p.step)}`]));
  const pages = useInfiniteQuery({
    queryKey: ["workspace", workspace, "workflows", workflow, "failures", from, step],
    queryFn: ({ pageParam }) =>
      workflowFailuresPage(workspace, workflow, { from, step, before: pageParam || undefined }),
    initialPageParam: "",
    getNextPageParam: (last) => last.next ?? undefined,
  });
  const failures = pages.data?.pages.flatMap((p) => p.items) ?? [];
  const stepChoices: Choices = [
    [everyStep, "Every step"],
    ...placed.map((p) => [p.step.id, labels.get(p.step.id) ?? ""] as const),
  ];
  const labelOf = (f: WorkflowFailure) =>
    f.step_id ? (labels.get(f.step_id) ?? stepLabel(undefined, f.step_kind ?? undefined)) : "The run";

  return (
    <div className="mx-auto flex w-full max-w-[760px] flex-col gap-8 px-6 pt-6 pb-16">
      <div className="flex flex-wrap items-center gap-2">
        <Pick label="Range" choices={dayChoices} value={days} onChange={setDays} />
        <Pick
          label="Step"
          choices={stepChoices}
          value={step || everyStep}
          onChange={(v) => setStep(v === everyStep ? "" : v)}
        />
      </div>
      {!pages.isPending && failures.length === 0 ? (
        <EmptyState
          icon={RiCheckboxCircleLine}
          title="No failures"
          description="Runs where a call, an action, a classify step or the agent step failed show here."
        />
      ) : (
        <Section
          title="Failed runs"
          description="Newest first. Open the conversation to see what the customer saw."
          loading={pages.isPending}
        >
          {failures.map((f) => (
            <FailureLine key={f.id} f={f} label={labelOf(f)} />
          ))}
          {pages.hasNextPage ? (
            <div className="flex justify-center py-3">
              <Action loading={pages.isFetchingNextPage} onPress={() => void pages.fetchNextPage()}>
                Load more
              </Action>
            </div>
          ) : null}
        </Section>
      )}
    </div>
  );
}
