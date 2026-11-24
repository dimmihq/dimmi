/*
 * The workflow's Preview: the draft read back in words. What still
 * stops it from being published comes first, and picking a problem
 * opens the step it is about. Then what starts it and what each step
 * does, path by path, the way the cards on the canvas read.
 */
import { RiAlertLine, RiErrorWarningLine } from "@remixicon/react";
import type { ReactNode } from "react";

import { ScrollArea } from "@/features/shell/page";
import type { Issue, Report } from "@/lib/api/workflows";
import { cx } from "@/utils/cx";

import { KindMark } from "../parts";
import { stepLook } from "../step-kinds";
import { type TriggerKind, tintOf, triggers } from "../triggers";
import { pathName, type Step, type StepKind } from "../workflows";

function Block({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section aria-label={title} className="flex flex-col gap-2">
      <h3 className="text-body-2-medium text-text-secondary">{title}</h3>
      {children}
    </section>
  );
}

function IssueLine({ issue, error, onShow }: { issue: Issue; error: boolean; onShow: (issue: Issue) => void }) {
  const Icon = error ? RiErrorWarningLine : RiAlertLine;

  return (
    <li>
      <button
        type="button"
        onClick={() => onShow(issue)}
        className="-mx-2 flex w-[calc(100%+1rem)] cursor-pointer items-start gap-2 rounded-lg px-2 py-1.5 text-start outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <Icon
          aria-hidden
          className={cx("mt-0.5 size-4 shrink-0", error ? "text-text-error-primary" : "text-foreground-icon-secondary")}
        />
        <span className="text-body-2-regular text-text-primary">{issue.message}</span>
      </button>
    </li>
  );
}

function StepLines({ steps, summarize }: { steps: Step[]; summarize: (step: Step) => string }) {
  if (steps.length === 0) {
    return <p className="text-body-2-regular text-text-tertiary">Nothing happens on this path.</p>;
  }

  return (
    <ol className="flex flex-col gap-2">
      {steps.map((step) => {
        const look = stepLook[step.kind as StepKind];

        return (
          <li key={step.id} className="flex flex-col gap-2">
            <span className="flex items-start gap-2.5">
              {look ? <KindMark icon={look.icon} tint={look.tint} size="sm" /> : null}
              <span className="min-w-0 pt-0.5 text-body-2-regular text-text-primary">{summarize(step)}</span>
            </span>
            {(step.paths ?? []).map((path) => (
              <div key={path.id} className="ms-3 flex flex-col gap-2 border-s border-separator-border ps-4">
                <span className="text-caption-1-semibold text-text-tertiary">{pathName(path) || "Path"}</span>
                <StepLines steps={path.steps} summarize={summarize} />
              </div>
            ))}
          </li>
        );
      })}
    </ol>
  );
}

export function WorkflowPreview({
  trigger,
  steps,
  report,
  summarize,
  onShow,
}: {
  trigger: TriggerKind;
  steps: Step[];
  report: Report;
  summarize: (step: Step) => string;
  onShow: (issue: Issue) => void;
}) {
  const look = triggers[trigger];

  return (
    <ScrollArea className="px-4 pb-6">
      <div className="flex min-w-0 flex-col gap-6 pt-2">
        {report.errors.length > 0 ? (
          <Block title="To fix before publishing">
            <ul className="flex flex-col">
              {report.errors.map((issue) => (
                <IssueLine
                  key={`${issue.code}-${issue.step_id ?? ""}-${issue.message}`}
                  issue={issue}
                  error
                  onShow={onShow}
                />
              ))}
            </ul>
          </Block>
        ) : null}
        {report.warnings.length > 0 ? (
          <Block title="Worth a look">
            <ul className="flex flex-col">
              {report.warnings.map((issue) => (
                <IssueLine
                  key={`${issue.code}-${issue.step_id ?? ""}-${issue.message}`}
                  issue={issue}
                  error={false}
                  onShow={onShow}
                />
              ))}
            </ul>
          </Block>
        ) : null}
        {report.errors.length === 0 && report.warnings.length === 0 ? (
          <p className="text-body-2-regular text-text-secondary">Nothing to fix.</p>
        ) : null}
        <Block title="What starts it">
          <span className="flex items-start gap-2.5">
            {look ? <KindMark icon={look.icon} tint={tintOf(trigger)} size="sm" /> : null}
            <span className="min-w-0 pt-0.5 text-body-2-regular text-text-primary">{look?.name ?? trigger}</span>
          </span>
        </Block>
        <Block title="What it does">
          <StepLines steps={steps} summarize={summarize} />
        </Block>
      </div>
    </ScrollArea>
  );
}
