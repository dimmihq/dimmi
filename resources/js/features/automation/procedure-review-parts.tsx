/*
 * How a requested change reads inside the procedure editor: a reworded
 * step keeps its own words struck through with the requested words in
 * green beneath, a removed step is struck through in red, and new steps
 * stand in green where they would go. A note above the steps says why,
 * and which tests come with the request.
 */
import { RiSparkling2Line } from "@remixicon/react";

import { Button } from "@/components/base/buttons/button";

import type { ProcedureStep } from "@/lib/api/procedures";
import type { ProcedureRequest, Review } from "./procedure-review";
import type { ChipSources } from "./procedure-writer/procedure-suggestions";
import type { StepChange } from "./procedures";
import { kindWords, stepTitle } from "./step-words";

/*
 * Requested is the requested words of a reworded step, under its own.
 */
export function Requested({ change, sources }: { change: StepChange | undefined; sources: ChipSources }) {
  if (change?.change !== "changed" || !change.after) {
    return null;
  }
  const before = change.before ? stepTitle(change.before, sources) : "";
  const after = stepTitle(change.after, sources);
  const retyped = change.before?.kind !== change.after.kind;
  return (
    <div className="flex flex-col gap-1">
      {before !== after ? (
        <span className="rounded-e-md px-2 py-0.5 text-body-2-regular border-s-2 border-status-rose-text bg-status-rose-background/40 text-text-tertiary line-through decoration-status-rose-text/60">
          {before}
        </span>
      ) : null}
      <span className="rounded-e-md px-2 py-0.5 text-body-2-regular text-text-primary border-s-2 border-status-lime-text bg-status-lime-background/40">
        {retyped ? `${kindWords[change.after.kind] ?? change.after.kind}: ` : ""}
        {after}
      </span>
    </div>
  );
}

function AddedStep({ step, sources }: { step: ProcedureStep; sources: ChipSources }) {
  const children = [...(step.then_steps ?? []), ...(step.else_steps ?? [])];
  return (
    <li className="my-1 flex flex-col gap-1 rounded-e-xl px-3 py-2.5 border-s-2 border-status-lime-text bg-status-lime-background/40">
      <span className="flex items-center gap-2">
        <span className="text-caption-1-medium text-status-lime-text">New {kindWords[step.kind] ?? step.kind}</span>
      </span>
      <span className="text-body-2-regular text-text-primary">{stepTitle(step, sources)}</span>
      {children.length > 0 ? (
        <ul className="flex flex-col gap-1 border-s-2 border-status-lime-text/40 ps-3">
          {children.map((c) => (
            <li key={c.id} className="text-body-2-regular text-text-primary">
              {stepTitle(c, sources)}
            </li>
          ))}
        </ul>
      ) : null}
    </li>
  );
}

/*
 * AddedSteps is the new steps a request puts after a step, or at the
 * start of a list.
 */
export function AddedSteps({ review, at, sources }: { review: Review | null; at: string; sources: ChipSources }) {
  const steps = review?.added.get(at) ?? [];
  if (steps.length === 0) {
    return null;
  }
  return (
    <>
      {steps.map((s) => (
        <AddedStep key={s.id} step={s} sources={sources} />
      ))}
    </>
  );
}

/*
 * RequestedText is a requested rewording of a plain field, such as when
 * dimmi uses the procedure.
 */
export function RequestedText({ before, after }: { before: string; after: string | undefined }) {
  if (after === undefined || after === before) {
    return null;
  }
  return (
    <div className="flex flex-col gap-1 pt-2">
      {before ? (
        <span className="rounded-e-md px-2 py-0.5 text-body-2-regular border-s-2 border-status-rose-text bg-status-rose-background/40 text-text-tertiary line-through decoration-status-rose-text/60">
          {before}
        </span>
      ) : null}
      <span className="rounded-e-md px-2 py-0.5 text-body-2-regular text-text-primary border-s-2 border-status-lime-text bg-status-lime-background/40">
        {after}
      </span>
    </div>
  );
}

/*
 * RequestNote says, above the steps, why Operator requests the change and
 * which tests come with it.
 */
export function RequestNote({ request }: { request: ProcedureRequest | null }) {
  if (!request) {
    return null;
  }
  return (
    <div className="mx-auto flex w-full max-w-[760px] items-start gap-2.5 px-6 pt-6">
      <RiSparkling2Line aria-hidden className="mt-0.5 size-4 shrink-0 text-foreground-icon-secondary" />
      <div className="flex min-w-0 flex-col gap-1 text-body-2-regular text-text-secondary">
        <p>
          <span className="text-body-2-medium text-text-primary">Operator requests a change: </span>
          {request.changeset.reason || request.changeset.title}
        </p>
        {request.tests.length > 0 ? (
          <p className="text-text-tertiary">
            Accepting also adds {request.tests.length === 1 ? "the test" : "the tests"}{" "}
            {request.tests.map((t) => `“${t.label.replace(/^Add the test “(.*)” to.*$/, "$1")}”`).join(", ")}.
          </p>
        ) : null}
      </div>
    </div>
  );
}

/*
 * RequestButtons are the header's Decline and Accept changes.
 */
export function RequestButtons({
  request,
  readOnly,
  busy,
  onDecide,
}: {
  request: ProcedureRequest | null;
  readOnly: boolean;
  busy: boolean;
  onDecide: (accepting: boolean) => void;
}) {
  if (!request || readOnly) {
    return null;
  }
  return (
    <>
      <Button size="small" variant="secondary" onClick={() => onDecide(false)} disabled={busy}>
        Decline
      </Button>
      <Button size="small" onClick={() => onDecide(true)} loading={busy}>
        Accept changes
      </Button>
    </>
  );
}
