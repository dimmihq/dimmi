/*
 * Per-step numbers for the workflow inspector and canvas, from
 * useWorkflowStepCounts. StepCounts is the inspector's Results card,
 * laid out like a series step's: reached, here now, each path, ended
 * here and failed. StepCountsBadge is the one line a canvas card can
 * carry.
 */
import { Row, Section } from "@/features/settings/layout";
import type { SpecStep, WorkflowStepMetrics } from "@/lib/api/workflows";
import { formatNumber } from "@/lib/i18n/format";
import { cx } from "@/utils/cx";

import { pathName } from "../workflows";
import { rate } from "./workflow-numbers";

function Value({ children }: { children: string }) {
  return <span className="text-body-2-medium text-text-primary tabular-nums">{children}</span>;
}

/*
 * StepCounts is a step's Results card: reached, here now, each path it
 * was left by, how many ended here, and how many failed.
 */
export function StepCounts({ step, counts }: { step: SpecStep; counts?: WorkflowStepMetrics }) {
  const reached = counts?.reached ?? 0;
  const labels = new Map((step.paths ?? []).map((p) => [p.id, pathName(p) || "A path"]));

  return (
    <Section title="Results" description="Live runs of the last 30 days. Test runs never count.">
      <Row label="Reached this step">
        <Value>{formatNumber(reached)}</Value>
      </Row>
      <Row label="Here now">
        <Value>{formatNumber(counts?.waiting ?? 0)}</Value>
      </Row>
      {Object.entries(counts?.paths ?? {}).map(([path, n]) => (
        <Row key={path} label={labels.get(path) ?? "A removed path"}>
          <Value>{`${formatNumber(n)} · ${rate(n, reached)}`}</Value>
        </Row>
      ))}
      {(counts?.dropped ?? 0) > 0 ? (
        <Row label="Ended here" description="The customer left, a teammate stepped in, or the run expired.">
          <Value>{`${formatNumber(counts?.dropped ?? 0)} · ${rate(counts?.dropped ?? 0, reached)}`}</Value>
        </Row>
      ) : null}
      {(counts?.failed ?? 0) > 0 ? (
        <Row label="Failed">
          <span className="text-body-2-medium text-text-error-primary tabular-nums">
            {formatNumber(counts?.failed ?? 0)}
          </span>
        </Row>
      ) : null}
    </Section>
  );
}

/*
 * StepCountsBadge is the line a canvas card carries: how many reached
 * the step, how many wait there, and failures when there are any.
 */
export function StepCountsBadge({ counts, className }: { counts?: WorkflowStepMetrics; className?: string }) {
  if (!counts || counts.reached === 0) {
    return null;
  }
  const parts = [`${formatNumber(counts.reached)} reached`];
  if (counts.waiting > 0) {
    parts.push(`${formatNumber(counts.waiting)} here now`);
  }

  return (
    <span
      className={cx(
        "inline-flex max-w-full items-center gap-1.5 truncate rounded-md bg-background-secondary-default px-1.5 py-0.5 text-caption-1-medium text-text-secondary tabular-nums",
        className,
      )}
    >
      {parts.join(" · ")}
      {counts.failed > 0 ? (
        <span className="text-text-error-primary">{`· ${formatNumber(counts.failed)} failed`}</span>
      ) : null}
    </span>
  );
}
