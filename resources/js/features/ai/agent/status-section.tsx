/*
 * Whether the agent answers customers. On dimmi's built-in provider the
 * agent comes with a model already picked, so the switch works from the
 * start. When something still stands in the way, the line under the
 * switch names it and links to where it is fixed.
 */
import { RiArrowRightSLine } from "@remixicon/react";
import { Link } from "@tanstack/react-router";

import { Switch } from "@/components/base/switch/switch";
import type { AgentView } from "@/lib/api/ai";

import { type AgentChange, readiness, type Step } from "../format";

const fixLook =
  "flex shrink-0 items-center gap-0.5 rounded-md text-body-2-medium text-text-primary outline-none hover:underline focus-visible:ring-2 focus-visible:ring-border-focus-ring";

/*
 * blockedLines say in a few words what keeps the agent from answering.
 */
const blockedLines: Record<Step["id"], string> = {
  provider: "Add a provider before it can answer.",
  model: "Choose the model it answers with.",
  channels: "Pick at least one channel for it to answer on.",
};

/*
 * StepFix is where the open step is fixed: the providers page for a
 * connection, the matching section of this page for the rest.
 */
function StepFix({ step, onJump }: { step: Step; onJump: (step: Step["id"]) => void }) {
  if (step.id === "provider") {
    return (
      <Link to="/d/settings/$section" params={{ section: "ai-providers" }} className={fixLook}>
        Add a provider
        <RiArrowRightSLine aria-hidden className="size-4" />
      </Link>
    );
  }

  return (
    <button type="button" onClick={() => onJump(step.id)} className={fixLook}>
      {step.id === "model" ? "Choose" : "Pick channels"}
      <RiArrowRightSLine aria-hidden className="size-4" />
    </button>
  );
}

/*
 * StatusSection is the switch, with what still blocks it when anything
 * does.
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
  const blocker = readiness(view).find((step) => !step.done);
  const { agent } = view;
  const line = agent.enabled
    ? `${agent.name} replies to new customer messages on the channels below.`
    : blocker
      ? blockedLines[blocker.id]
      : "Ready to answer. Switch it on when you are.";

  return (
    <section aria-label="Status" className="flex flex-col gap-2">
      <div className="flex min-h-[72px] items-center justify-between gap-4 rounded-2xl bg-background-secondary-default px-3 py-3">
        <div className="flex min-w-0 flex-col gap-0.5">
          <span className="text-headline-medium text-text-primary">Answer customers</span>
          <span className="text-body-2-regular text-text-secondary">{line}</span>
        </div>
        <div className="flex shrink-0 items-center gap-4">
          {blocker && !agent.enabled && canWrite ? <StepFix step={blocker} onJump={onJump} /> : null}
          <Switch
            aria-label="Answer customers"
            isSelected={agent.enabled}
            isDisabled={!canWrite || (blocker !== undefined && !agent.enabled)}
            onChange={(enabled) => save({ enabled })}
          />
        </div>
      </div>
    </section>
  );
}
