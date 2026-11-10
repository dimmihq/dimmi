/*
 * The last step of a broadcast: everything decided, in one place, each
 * with a way back to the step that set it, then what still stands
 * between the draft and sending it. The test send counts as done once
 * one went out from here. When the server refuses to send, its reason
 * stands on top, since it knows things the checklist cannot, such as an
 * address whose domain is not verified yet. A scheduled broadcast shows
 * the same summary without the ways back: it must be unscheduled to
 * change.
 */
import { RiCheckboxBlankCircleLine, RiCheckboxCircleFill, RiErrorWarningLine } from "@remixicon/react";

import { Button } from "@/components/base/buttons/button";
import { useWorkspace } from "@/features/customers/names";
import { Row, Section } from "@/features/settings/layout";
import type { Broadcast, BroadcastEstimate } from "@/lib/api/broadcasts";
import { formatNumber } from "@/lib/i18n/format";
import { cx } from "@/utils/cx";

import type { Check, Draft, StepId } from "./draft";
import { channelLook, reachNoun } from "./looks";
import { sendTimeOf } from "./schedule";
import { useWorkspaceZone } from "./sources";
import { filled } from "./writer/variables";

function Edit({ step, onStep }: { step: StepId; onStep?: (step: StepId) => void }) {
  if (!onStep) {
    return null;
  }

  return (
    <Button variant="ghost" size="xs" onClick={() => onStep(step)}>
      Edit
    </Button>
  );
}

function Checklist({ checks, onStep }: { checks: Check[]; onStep?: (step: StepId) => void }) {
  return (
    <Section title="Before you send">
      <ul className="flex flex-col py-1.5 pe-2.5">
        {checks.map((check) => (
          <li key={check.label} className="flex min-h-9 items-center gap-2.5">
            {check.done ? (
              <RiCheckboxCircleFill aria-hidden className="size-4 text-status-lime-text" />
            ) : (
              <RiCheckboxBlankCircleLine aria-hidden className="size-4 text-foreground-icon-quaternary" />
            )}
            <span className={cx("flex-1 text-body-regular", check.done ? "text-text-secondary" : "text-text-primary")}>
              {check.label}
            </span>
            <span className="sr-only">{check.done ? "done" : "to do"}</span>
            {check.done || check.step === "review" ? null : <Edit step={check.step} onStep={onStep} />}
          </li>
        ))}
      </ul>
    </Section>
  );
}

/*
 * Refusal is why the server would not send, in its own words.
 */
export function Refusal({ message }: { message: string }) {
  return (
    <div
      role="alert"
      className="flex items-start gap-3 rounded-2xl border border-border-error-default bg-background-primary-default px-4 py-3"
    >
      <RiErrorWarningLine aria-hidden className="mt-0.5 size-4 shrink-0 text-text-error-primary" />
      <p className="text-body-regular text-text-primary">{message}</p>
    </div>
  );
}

function whenOf(draft: Draft, workspaceZone: string | undefined): string {
  if (draft.schedule_kind === "now" || !draft.scheduled_at) {
    return "Right away";
  }

  return sendTimeOf(draft.schedule_kind, draft.scheduled_at, workspaceZone);
}

/*
 * Names are what the review shows for the draft's ids.
 */
export interface Names {
  audience: string;
  sender: string;
}

export function ReviewStep({
  broadcast,
  draft,
  names,
  estimate,
  checks,
  refusal,
  onStep,
}: {
  broadcast: Broadcast;
  draft: Draft;
  names: Names;
  estimate: BroadcastEstimate | undefined;
  checks: Check[];
  refusal: string | null;
  onStep?: (step: StepId) => void;
}) {
  const zone = useWorkspaceZone(useWorkspace());
  const channel = channelLook[broadcast.channel].name;
  const reach = estimate
    ? `${formatNumber(estimate.reach)}${estimate.capped ? "+" : ""} ${reachNoun(broadcast.channel, estimate.reach)}`
    : `Counting ${reachNoun(broadcast.channel, 2)}…`;

  return (
    <div className="flex flex-col gap-8">
      {refusal ? <Refusal message={refusal} /> : null}
      <Section title="Summary">
        <Row label={draft.name || "Untitled"} description={`${channel} to ${names.audience}`}>
          <Edit step="setup" onStep={onStep} />
        </Row>
        <Row label={reach} description="Counted again when it sends, without unsubscribed and bounced people." />
        {broadcast.channel === "chat" ? null : (
          <Row label={filled(draft.subject) || "No subject"} description={draft.preview || undefined}>
            <Edit step="write" onStep={onStep} />
          </Row>
        )}
        <Row label={`From ${names.sender}`} description={whenOf(draft, zone)}>
          <Edit step="delivery" onStep={onStep} />
        </Row>
      </Section>
      {onStep ? <Checklist checks={checks} onStep={onStep} /> : null}
    </div>
  );
}
