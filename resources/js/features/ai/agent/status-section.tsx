/*
 * Whether the agent answers customers, and what still stands in the way.
 * The switch turns on only once every step is done; each open step links
 * to where it is fixed, so setting the agent up is a short checklist
 * rather than a hunt through the page.
 */
import { RiArrowRightSLine, RiCheckLine } from "@remixicon/react";
import { Link } from "@tanstack/react-router";

import { Switch } from "@/components/base/switch/switch";
import type { AgentView } from "@/lib/api/ai";
import { cx } from "@/utils/cx";

import { type AgentChange, readiness, type Step } from "../format";

/*
 * StepFix is where an open step is fixed: the providers page for a
 * connection, the matching section of this page for the rest.
 */
function StepFix({ step, onJump }: { step: Step; onJump: (step: Step["id"]) => void }) {
  const look =
    "flex items-center gap-0.5 rounded-md text-body-2-medium text-text-primary outline-none hover:underline focus-visible:ring-2 focus-visible:ring-border-focus-ring";

  if (step.id === "provider") {
    return (
      <Link to="/d/settings/$section" params={{ section: "ai-providers" }} className={look}>
        Add a provider
        <RiArrowRightSLine aria-hidden className="size-4" />
      </Link>
    );
  }

  return (
    <button type="button" onClick={() => onJump(step.id)} className={look}>
      {step.id === "model" ? "Choose" : "Pick channels"}
      <RiArrowRightSLine aria-hidden className="size-4" />
    </button>
  );
}

function StepRow({ step, onJump }: { step: Step; onJump: (step: Step["id"]) => void }) {
  return (
    <li className="flex min-h-[52px] items-center gap-3 border-separator-border border-b py-2.5 pe-2.5 last:border-b-0">
      <span
        aria-hidden
        className={cx(
          "flex size-5 shrink-0 items-center justify-center rounded-full",
          step.done
            ? "bg-notification-success-background text-notification-success-foreground"
            : "border border-border-button-default border-dashed",
        )}
      >
        {step.done ? <RiCheckLine className="size-3.5" /> : null}
      </span>
      <span className="flex min-w-0 flex-1 flex-col">
        <span className={cx("text-body-regular", step.done ? "text-text-secondary" : "text-text-primary")}>
          {step.label}
          <span className="sr-only">{step.done ? ", done" : ", to do"}</span>
        </span>
        {step.done ? null : <span className="text-body-2-regular text-text-secondary">{step.hint}</span>}
      </span>
      {step.done ? null : <StepFix step={step} onJump={onJump} />}
    </li>
  );
}

/*
 * StatusSection is the switch and the checklist under it.
 */
export function StatusSection({
  view,
  canWrite,
  save,
  onJump,
}: {
  view: AgentView;
  canWrite: boolean;
  save: (change: AgentChange) => void;
  onJump: (step: Step["id"]) => void;
}) {
  const steps = readiness(view);
  const open = steps.filter((step) => !step.done).length;
  const ready = open === 0;
  const { agent } = view;
  const line = agent.enabled
    ? `${agent.name} replies to new customer messages on the channels below.`
    : ready
      ? "Everything is in place. Switch it on when you are ready."
      : `${open === 1 ? "One step" : `${open} steps`} left before it can answer.`;

  return (
    <section aria-label="Status" className="flex flex-col gap-2">
      <div className="flex flex-col rounded-2xl bg-background-secondary-default ps-3">
        <div className="flex min-h-[72px] items-center justify-between gap-4 border-separator-border border-b py-3 pe-3">
          <div className="flex min-w-0 flex-col gap-0.5">
            <span className="text-headline-medium text-text-primary">Answer customers</span>
            <span className="text-body-2-regular text-text-secondary">{line}</span>
          </div>
          <Switch
            aria-label="Answer customers"
            isSelected={agent.enabled}
            isDisabled={!canWrite || (!ready && !agent.enabled)}
            onChange={(enabled) => save({ enabled })}
          />
        </div>
        <ul aria-label="Setup" className="flex flex-col">
          {steps.map((step) => (
            <StepRow key={step.id} step={step} onJump={onJump} />
          ))}
        </ul>
      </div>
    </section>
  );
}
