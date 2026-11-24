/*
 * The first steps of a new workspace, at the foot of the sidebar. The
 * sidebar keeps one line: a ring that fills as steps get done, and the
 * count. Pressing it opens the steps beside the sidebar, where there is
 * room to say why each one matters. Each step checks itself off from
 * what the workspace has done and leads to the page that sets it up;
 * while the list is open it keeps checking, so a step done in another
 * tab ticks here on its own. It shows to people who can change settings,
 * and goes away once every step is done.
 */
import { RiArrowRightSLine, RiCheckLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useState } from "react";

import { Popover, PopoverTrigger } from "@/components/base/popover/popover";
import { ProgressCircle, ProgressTrack } from "@/components/base/progress/progress";
import { callerQuery } from "@/lib/api/workspace";
import { cx } from "@/utils/cx";

import { type Step, useSteps } from "./setup-steps";

/*
 * Tick is a step's mark: an empty ring, or a filled one whose check pops
 * in when the step gets done.
 */
function Tick({ done }: { done: boolean }) {
  return (
    <span
      aria-hidden
      className={cx(
        "flex size-5 shrink-0 items-center justify-center rounded-full border transition-colors duration-300",
        done ? "border-transparent bg-accent-500" : "border-border-button-default",
      )}
    >
      <RiCheckLine
        className={cx(
          "size-3.5 text-white transition-[scale,opacity] duration-300 ease-out motion-reduce:transition-none",
          done ? "scale-100 opacity-100" : "scale-50 opacity-0",
        )}
      />
    </span>
  );
}

/*
 * Steps is the list in the popover. Rows come in one after another, and
 * a done one is struck through.
 */
function Steps({ steps, onOpen }: { steps: Step[]; onOpen: () => void }) {
  return (
    <ul className="flex flex-col">
      {steps.map((step, index) => (
        <li key={step.label} className="motion-safe:animate-page-in" style={{ animationDelay: `${index * 45}ms` }}>
          <Link
            to={step.href}
            onClick={onOpen}
            className={cx(
              "flex items-center gap-3 rounded-xl px-2.5 py-2 no-underline outline-none transition-colors duration-150",
              "hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring",
            )}
          >
            <Tick done={step.done} />
            <span className="flex min-w-0 flex-1 flex-col">
              <span
                className={cx(
                  "text-body-medium transition-colors duration-300",
                  step.done ? "text-text-tertiary line-through" : "text-text-primary",
                )}
              >
                {step.label}
                <span className="sr-only">{step.done ? " (done)" : ""}</span>
              </span>
              <span className="text-body-2-regular text-text-secondary">{step.why}</span>
            </span>
            {step.done ? null : (
              <RiArrowRightSLine
                aria-hidden
                className="size-5 shrink-0 text-foreground-icon-tertiary rtl:-scale-x-100"
              />
            )}
          </Link>
        </li>
      ))}
    </ul>
  );
}

export function GetStarted({ workspace }: { workspace: string }) {
  const [open, setOpen] = useState(false);
  const { data: caller } = useQuery({ ...callerQuery(workspace), enabled: workspace !== "" });
  const steps = useSteps(workspace, caller?.member?.permissions?.includes("settings:read") ?? false, open);

  if (!steps) {
    return null;
  }
  const done = steps.filter((step) => step.done).length;
  if (done === steps.length) {
    return null;
  }
  const share = (done / steps.length) * 100;

  return (
    <PopoverTrigger isOpen={open} onOpenChange={setOpen}>
      <button
        type="button"
        className={cx(
          "flex w-full cursor-pointer items-center gap-2 rounded-xl bg-background-tertiary-default px-3 py-2 outline-none",
          "transition-colors duration-150 hover:bg-background-secondary-hover",
          "focus-visible:ring-2 focus-visible:ring-border-focus-ring",
        )}
      >
        <ProgressCircle size="sm" value={share} aria-label="Setup progress" />
        <span className="flex-1 text-start text-body-medium text-text-primary">Get started</span>
        <span className="text-caption-1-medium text-text-tertiary tabular-nums">
          {done}/{steps.length}
        </span>
        <RiArrowRightSLine
          aria-hidden
          className={cx(
            "size-4 text-foreground-icon-tertiary transition-transform duration-200 rtl:-scale-x-100",
            open && "translate-x-0.5",
          )}
        />
      </button>
      <Popover
        placement="right bottom"
        offset={12}
        aria-label="Get started"
        className="w-[380px]"
        dialogClassName="p-1.5"
      >
        <div className="flex flex-col gap-2 px-2.5 pt-2 pb-1">
          <div className="flex items-baseline justify-between gap-4">
            <span className="text-headline-medium text-text-primary">Get started</span>
            <span className="text-body-2-regular text-text-secondary tabular-nums">
              {done} of {steps.length} done
            </span>
          </div>
          <ProgressTrack percentage={share} size="sm" />
        </div>
        <Steps steps={steps} onOpen={() => setOpen(false)} />
      </Popover>
    </PopoverTrigger>
  );
}
