/*
 * A change Operator requests to a procedure, read against the working
 * copy step by step: which steps it rewords, which it removes, and which
 * new steps it adds after which, so the editor can draw each one where
 * it stands. Tests the request adds come with it.
 */
import { useMutation } from "@tanstack/react-query";

import { toast } from "@/components/base/toast/toast";
import { useSuggestions } from "@/features/operator/suggestions";
import type { OperatorChangeset, OperatorOperation } from "@/lib/api/operator";
import type { ProcedureDefinition, ProcedureStep } from "@/lib/api/procedures";

import { diffSteps, type StepChange } from "./procedures";

export interface ProcedureRequest {
  changeset: OperatorChangeset;
  // edit is the request's new working copy, when it changes the steps.
  edit: OperatorOperation | null;
  // tests are the regression tests the request adds.
  tests: OperatorOperation[];
}

export interface Review {
  proposed: ProcedureDefinition;
  // changes is each step of the working copy the request changes or removes.
  changes: Map<string, StepChange>;
  // added is the new steps, under the step they follow, or `start:<scope>`
  // for the first steps of a list.
  added: Map<string, ProcedureStep[]>;
}

/*
 * requestFor is the oldest pending request on a procedure.
 */
export function requestFor(changesets: OperatorChangeset[], procedure: string): ProcedureRequest | null {
  const mine = changesets
    .filter((c) => c.operations.some((o) => o.procedure_id === procedure))
    .sort((a, b) => a.created_at.localeCompare(b.created_at));
  const changeset = mine[0];
  if (!changeset) {
    return null;
  }
  const ours = changeset.operations.filter((o) => o.procedure_id === procedure);
  return {
    changeset,
    edit: ours.find((o) => o.kind === "procedure.edit_draft") ?? null,
    tests: ours.filter((o) => o.kind === "procedure.create_test_case"),
  };
}

/*
 * proposedOf is a request's working copy over the current one, or
 * nothing when it sends none or cannot be read.
 */
export function proposedOf(current: ProcedureDefinition, edit: OperatorOperation | null): ProcedureDefinition | null {
  if (!edit?.text) {
    return null;
  }
  try {
    return { ...current, ...(JSON.parse(edit.text) as Partial<ProcedureDefinition>) };
  } catch {
    return null;
  }
}

/*
 * placeAdded files each new step under the step before it in its list.
 */
function placeAdded(steps: ProcedureStep[], scope: string, isNew: (id: string) => boolean, out: Review["added"]) {
  let before = `start:${scope}`;
  for (const step of steps) {
    if (isNew(step.id)) {
      out.set(before, [...(out.get(before) ?? []), step]);
      continue;
    }
    before = step.id;
    placeAdded(step.then_steps ?? [], `${step.id}:then_steps`, isNew, out);
    placeAdded(step.else_steps ?? [], `${step.id}:else_steps`, isNew, out);
  }
}

/*
 * reviewOf reads a request against the working copy.
 */
export function reviewOf(current: ProcedureDefinition, proposed: ProcedureDefinition): Review {
  const diff = diffSteps(current, proposed);
  const changes = new Map(diff.filter((c) => c.change === "changed" || c.change === "removed").map((c) => [c.id, c]));
  const fresh = new Set(diff.filter((c) => c.change === "added").map((c) => c.id));
  const added: Review["added"] = new Map();
  placeAdded(proposed.steps, "root", (id) => fresh.has(id), added);
  return { proposed, changes, added };
}

/*
 * useProcedureRequest is the oldest request waiting on a procedure and
 * how to accept or decline it.
 */
export function useProcedureRequest(workspace: string, procedure: string) {
  const { suggestions, decide } = useSuggestions(workspace, (op) => op.procedure_id === procedure);
  const request = requestFor(suggestions, procedure);
  return {
    request,
    accept: () => (request ? decide(request.changeset.id, "applications") : Promise.resolve(null)),
    decline: () => (request ? decide(request.changeset.id, "declines") : Promise.resolve(null)),
  };
}

/*
 * removedStep is the class of a step the request removes.
 */
export const removedStep = "rounded-xl bg-status-rose-background [&_[contenteditable]]:line-through";

/*
 * useRequestReview is the request on a procedure read against its
 * working copy, and how to accept or decline it; accepting loads the
 * working copy again.
 */
export function useRequestReview(
  workspace: string,
  procedure: string,
  definition: ProcedureDefinition,
  onAccepted: () => void,
) {
  const requested = useProcedureRequest(workspace, procedure);
  const request = requested.request;
  const proposed = request ? proposedOf(definition, request.edit) : null;
  const decide = useMutation({
    mutationFn: async (accepting: boolean) => {
      await (accepting ? requested.accept() : requested.decline());
      if (accepting) {
        onAccepted();
      }
    },
    onSuccess: (_, accepting) =>
      toast.success(accepting ? "Changes accepted into the working copy. Publish when ready." : "Request declined."),
    onError: (error) => toast.error(error.message),
  });
  return { request, review: proposed ? reviewOf(definition, proposed) : null, decide };
}
