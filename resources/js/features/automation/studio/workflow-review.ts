/*
 * A change Operator suggests to a workflow, read against the working
 * copy step by step: which steps it adds, changes and removes, and
 * whether it changes what starts the workflow. The builder draws the
 * suggested copy with those marks, so the teammate reviews it on the
 * canvas itself and accepts or declines it from there. Accepting saves
 * the suggested copy as the working copy, and the builder adopts it.
 *
 * It shows as soon as Operator suggests it: the Operator chat refreshes
 * the workspace's pending changes on the step that made it.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";

import { toast } from "@/components/base/toast/toast";
import { useSuggestions } from "@/features/operator/suggestions";
import type { OperatorChangeset, OperatorOperation } from "@/lib/api/operator";
import { type Spec, type SpecStep, type Workflow, workflowQuery } from "@/lib/api/workflows";

export interface WorkflowRequest {
  changeset: OperatorChangeset;
  edit: OperatorOperation;
}

export interface WorkflowReview {
  proposed: Spec;
  added: Set<string>;
  changed: Set<string>;
  removed: Set<string>;
  // head is true when the trigger's settings, channels, audience, schedule or re-entry change.
  head: boolean;
}

/*
 * stableJSON writes a value with its object keys sorted, so two copies
 * that differ only in key order compare equal.
 */
export function stableJSON(value: unknown): string {
  if (Array.isArray(value)) {
    return `[${value.map(stableJSON).join(",")}]`;
  }
  if (value && typeof value === "object") {
    const entries = Object.entries(value as Record<string, unknown>)
      .filter(([, v]) => v !== undefined)
      .sort(([a], [b]) => a.localeCompare(b));
    return `{${entries.map(([k, v]) => `${JSON.stringify(k)}:${stableJSON(v)}`).join(",")}}`;
  }
  return JSON.stringify(value ?? null);
}

/*
 * stepsById lists every step of a tree by id, each without the steps of
 * its paths, so a step counts as changed only for its own settings,
 * kind and paths.
 */
export function stepsById(steps: SpecStep[], out = new Map<string, string>()): Map<string, string> {
  for (const step of steps) {
    const paths = (step.paths ?? []).map(({ steps: inner, ...path }) => path);
    out.set(step.id, stableJSON({ kind: step.kind, settings: step.settings ?? {}, paths }));
    for (const path of step.paths ?? []) {
      stepsById(path.steps ?? [], out);
    }
  }
  return out;
}

const headOf = (spec: Spec) =>
  stableJSON({
    trigger_settings: spec.trigger_settings ?? {},
    channels: spec.channels ?? [],
    audience: spec.audience ?? null,
    schedule: spec.schedule,
    reentry: spec.reentry,
    exclusive: spec.exclusive,
  });

/*
 * reviewOf reads a suggested copy against the working copy.
 */
export function reviewOf(current: Spec, proposed: Spec): WorkflowReview {
  const before = stepsById(current.steps ?? []);
  const after = stepsById(proposed.steps ?? []);
  const added = new Set([...after.keys()].filter((id) => !before.has(id)));
  const removed = new Set([...before.keys()].filter((id) => !after.has(id)));
  const changed = new Set([...after.keys()].filter((id) => before.has(id) && before.get(id) !== after.get(id)));
  return { proposed, added, changed, removed, head: headOf(current) !== headOf(proposed) };
}

/*
 * requestFor is the oldest pending suggestion on a workflow.
 */
export function requestFor(changesets: OperatorChangeset[], workflow: string): WorkflowRequest | null {
  const mine = changesets
    .filter((c) => c.operations.some((o) => o.workflow_id === workflow && o.kind === "workflow.edit_draft"))
    .sort((a, b) => a.created_at.localeCompare(b.created_at));
  const changeset = mine[0];
  const edit = changeset?.operations.find((o) => o.workflow_id === workflow && o.kind === "workflow.edit_draft");
  return changeset && edit ? { changeset, edit } : null;
}

/*
 * proposedOf is a suggestion's copy, or nothing when it cannot be read.
 */
export function proposedOf(edit: OperatorOperation | undefined): Spec | null {
  if (!edit?.text) {
    return null;
  }
  try {
    return JSON.parse(edit.text) as Spec;
  } catch {
    return null;
  }
}

/*
 * useWorkflowRequest is the suggestion waiting on a workflow, read
 * against the working copy the builder shows, and how to accept or
 * decline it. Accepting reads the saved workflow again and hands it to
 * onAccepted, which the builder passes its draft's adopt as.
 */
export function useWorkflowRequest(
  workspace: string,
  workflow: string,
  current: Spec | undefined,
  onAccepted?: (next: Workflow) => void,
) {
  const queryClient = useQueryClient();
  const { suggestions, decide } = useSuggestions(workspace, (op) => op.workflow_id === workflow);
  const request = requestFor(suggestions, workflow);
  const proposed = proposedOf(request?.edit);
  const deciding = useMutation({
    mutationFn: async (accepting: boolean) => {
      if (!request) {
        return;
      }
      await decide(request.changeset.id, accepting ? "applications" : "declines");
      if (accepting) {
        const fresh = await queryClient.fetchQuery({ ...workflowQuery(workspace, workflow), staleTime: 0 });
        onAccepted?.(fresh);
      }
    },
    onSuccess: (_, accepting) =>
      toast.success(accepting ? "Changes accepted into the working copy. Publish when ready." : "Suggestion declined."),
    onError: (error) => toast.error(error.message),
  });

  return {
    request,
    proposed,
    review: proposed && current ? reviewOf(current, proposed) : null,
    accept: () => deciding.mutate(true),
    decline: () => deciding.mutate(false),
    deciding: deciding.isPending,
  };
}
