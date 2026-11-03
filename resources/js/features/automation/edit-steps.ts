/*
 * Changes to a workflow's steps while it is being built. Steps live in a
 * tree (paths hold steps, steps hold paths), so each change rebuilds the
 * branch it touches and leaves the rest as it was.
 */
import { describe, fieldsOf, unsetSummary } from "./step-settings";
import type { Path, Step, StepKind } from "./workflows";

/*
 * withStep adds a step to the end of a path, or to the top level when
 * path is null.
 */
export function withStep(steps: Step[], path: string | null, step: Step): Step[] {
  if (path === null) {
    return [...steps, step];
  }

  return steps.map((current) => ({
    ...current,
    paths: current.paths?.map(
      (inner): Path =>
        inner.id === path
          ? { ...inner, steps: [...inner.steps, step] }
          : { ...inner, steps: withStep(inner.steps, path, step) },
    ),
  }));
}

/*
 * withoutStep removes a step, and every path below it, from anywhere in
 * the tree.
 */
export function withoutStep(steps: Step[], id: string): Step[] {
  return steps
    .filter((step) => step.id !== id)
    .map((step) => ({
      ...step,
      paths: step.paths?.map((path): Path => ({ ...path, steps: withoutStep(path.steps, id) })),
    }));
}

/*
 * withEdited replaces one step, wherever it is in the tree, with what
 * edit makes of it.
 */
export function withEdited(steps: Step[], id: string, edit: (step: Step) => Step): Step[] {
  return steps.map((step) => {
    const current = step.id === id ? edit(step) : step;

    return {
      ...current,
      paths: current.paths?.map((path): Path => ({ ...path, steps: withEdited(path.steps, id, edit) })),
    };
  });
}

/*
 * freshStep is a new step of a kind with the paths that kind needs. Its
 * card describes the settings it starts with, or says what to fill in
 * when it starts with nothing written.
 */
export function freshStep(kind: StepKind, id: string): Step {
  const paths: Partial<Record<StepKind, string[]>> = {
    buttons: ["Option 1", "Option 2"],
    branch: ["If…", "Everyone else"],
    dimmi: ["Resolved", "Hands over", "No answer"],
  };
  const labels = paths[kind];
  const start = describe[kind]?.(fieldsOf({ kind, summary: unsetSummary })) ?? unsetSummary;

  return {
    id,
    kind,
    summary: start,
    paths: labels?.map((label, index) => ({ id: `${id}-p${index}`, label, steps: [] })),
  };
}
