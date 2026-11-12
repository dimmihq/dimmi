/*
 * Changes to a workflow's steps while it is being built. Steps live in a
 * tree (paths hold steps, steps hold paths), so each change rebuilds the
 * branch it touches and leaves the rest as it was.
 */
import { newId, type Path, type Step, type StepKind } from "./workflows";

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

const starts: Partial<Record<StepKind, Step["settings"]>> = {
  message: { text: "" },
  buttons: { text: "" },
  collect: { text: "What's the best email to reach you?", field: "contact.email", type: "email" },
  wait: { for: "1h" },
  snooze: { for: "1d" },
  assign: { mode: "team" },
  unassign: { what: "both" },
  tag: { tag_ids: [] },
  untag: { tag_ids: [] },
  priority: { value: "high" },
  note: { text: "" },
  update_contact: { set: [{ field: "contact.name", value: "" }] },
  http_request: { method: "GET", url: "" },
};

/*
 * freshStep is a new step of a kind with the settings it starts with
 * and the paths it needs: two buttons for reply buttons, one condition
 * and everyone else for a branch, two categories and none matched for
 * sorting by topic, and for dimmi a path for each way its turn ends. A
 * call to your system starts with no paths; its failure path is opt-in.
 */
export function freshStep(kind: StepKind): Step {
  const step: Step = { id: newId("s_"), kind, settings: { ...starts[kind] } };
  if (kind === "buttons") {
    step.paths = ["Option 1", "Option 2"].map((label) => ({ id: newId("p_"), label, steps: [] }));
  }
  if (kind === "dimmi") {
    step.paths = ["resolved", "handed_over", "quiet"].map((outcome) => ({ id: newId("p_"), outcome, steps: [] }));
  }
  if (kind === "classify") {
    step.paths = [
      { id: newId("p_"), label: "Category 1", steps: [] },
      { id: newId("p_"), label: "Category 2", steps: [] },
      { id: newId("p_"), outcome: "none", steps: [] },
    ];
  }
  if (kind === "branch") {
    step.paths = [
      { id: newId("p_"), label: "Path 1", steps: [] },
      { id: newId("p_"), outcome: "else", steps: [] },
    ];
  }

  return step;
}
