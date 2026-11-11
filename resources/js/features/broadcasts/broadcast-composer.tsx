/*
 * Writing a broadcast, one step at a time: set it up, write it, decide
 * how it is delivered, then review and send. The steps sit in the middle
 * of the header; any of them can be opened at any time, since nothing is lost by
 * jumping. The current one wears a pill that glides to the next, and
 * each step slides in from the side the person is heading, on BoardUI's
 * motion. Back and Continue sit at the foot of each step; the last step
 * sends from the header.
 */
import { RiArrowLeftLine, RiArrowRightLine, RiCheckLine } from "@remixicon/react";
import { motion, useReducedMotion } from "motion/react";
import type { ReactNode } from "react";

import { Button } from "@/components/base/buttons/button";
import { Swap } from "@/features/inbox/motion";
import type { Broadcast, BroadcastEstimate } from "@/lib/api/broadcasts";
import { cx } from "@/utils/cx";

import { DeliveryStep } from "./delivery-step";
import { type Check, type Draft, type StepId, stepNames, steps } from "./draft";
import { type Names, ReviewStep } from "./review-step";
import { SetupStep } from "./setup-step";
import { WriteStep } from "./write-step";

/*
 * StepButton is one step in the row: its number, or a tick once its
 * checks pass, and its name. The current one wears the gliding pill.
 */
function StepButton({
  id,
  number,
  current,
  done,
  onStep,
}: {
  id: StepId;
  number: number;
  current: boolean;
  done: boolean;
  onStep: (step: StepId) => void;
}) {
  const still = useReducedMotion();
  const ticked = done && !current;

  return (
    <button
      type="button"
      aria-current={current ? "step" : undefined}
      onClick={() => onStep(id)}
      className={cx(
        "relative flex h-8 shrink-0 items-center gap-2 rounded-full px-3 outline-none transition-colors duration-150",
        "focus-visible:ring-2 focus-visible:ring-border-focus-ring",
        current ? "text-text-primary" : "text-text-secondary hover:text-text-primary",
      )}
    >
      {current ? (
        <motion.span
          layoutId="broadcast-step"
          transition={still ? { duration: 0 } : { type: "spring", stiffness: 520, damping: 42, mass: 0.7 }}
          className="absolute inset-0 rounded-full bg-background-primary-default shadow-xs ring-1 ring-border-button-default ring-inset"
        />
      ) : null}
      <span
        className={cx(
          "relative flex size-5 items-center justify-center rounded-full text-caption-1-medium tabular-nums",
          ticked ? "bg-status-lime-background text-status-lime-text" : "bg-background-tertiary-default",
        )}
      >
        {ticked ? <RiCheckLine aria-hidden className="size-3.5" /> : `${number}`}
      </span>
      <span className={cx("relative", current ? "text-body-2-medium" : "text-body-2-regular")}>{stepNames[id]}</span>
    </button>
  );
}

/*
 * Stepper is the row of steps. A step whose checks all pass wears a
 * tick.
 */
export function Stepper({ checks, step, onStep }: { checks: Check[]; step: StepId; onStep: (step: StepId) => void }) {
  return (
    <nav aria-label="Steps" className="flex max-w-full items-center gap-1 overflow-x-auto [scrollbar-width:none]">
      {steps.map((id, index) => (
        <StepButton
          key={id}
          id={id}
          number={index + 1}
          current={id === step}
          done={id !== "review" && checks.filter((check) => check.step === id).every((check) => check.done)}
          onStep={onStep}
        />
      ))}
    </nav>
  );
}

function Foot({ step, onStep }: { step: StepId; onStep: (step: StepId) => void }) {
  const at = steps.indexOf(step);
  const back = steps[at - 1];
  const next = steps[at + 1];

  return (
    <div className="flex items-center justify-between gap-3 pt-2">
      {back ? (
        <Button variant="secondary" size="small" leadingIcon={RiArrowLeftLine} onClick={() => onStep(back)}>
          {stepNames[back]}
        </Button>
      ) : (
        <span />
      )}
      {next ? (
        <Button size="small" trailingIcon={RiArrowRightLine} onClick={() => onStep(next)}>
          Continue to {stepNames[next].toLowerCase()}
        </Button>
      ) : null}
    </div>
  );
}

/*
 * ComposerProps are what every step reads: the stored broadcast, the
 * draft being written, the names of its ids, who it reaches now, what
 * still stands before sending and why the server last refused to send.
 */
export interface ComposerProps {
  workspace: string;
  broadcast: Broadcast;
  draft: Draft;
  names: Names;
  estimate: BroadcastEstimate | undefined;
  checks: Check[];
  refusal: string | null;
  step: StepId;
  forward: boolean;
  onStep: (step: StepId) => void;
  onChange: (change: Partial<Draft>) => void;
}

export function BroadcastComposer(props: ComposerProps) {
  const { workspace, broadcast, draft, names, step, forward, onStep, onChange } = props;
  const shared = { workspace, broadcast, draft, onChange };
  const views: Record<StepId, ReactNode> = {
    setup: <SetupStep {...shared} />,
    write: (
      <WriteStep
        workspace={workspace}
        channel={broadcast.channel}
        draft={draft}
        sender={names.sender}
        onChange={onChange}
      />
    ),
    delivery: <DeliveryStep {...shared} />,
    review: (
      <ReviewStep
        broadcast={broadcast}
        draft={draft}
        names={names}
        estimate={props.estimate}
        checks={props.checks}
        refusal={props.refusal}
        onStep={onStep}
      />
    ),
  };

  return (
    <div
      className={cx(
        "mx-auto flex w-full flex-col pb-16",
        step === "write" ? "max-w-[42.5rem] px-6 pt-10 md:pt-14" : "max-w-[680px] px-4 pt-4 sm:px-6",
      )}
    >
      <Swap view={step} forward={forward} className="flex flex-col gap-8">
        {views[step]}
        <Foot step={step} onStep={onStep} />
      </Swap>
    </div>
  );
}
