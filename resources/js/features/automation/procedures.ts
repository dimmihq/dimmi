/*
 * Changes to a procedure while it is edited: steps added, removed,
 * moved, and rewritten, nested then and else steps included, and the
 * handover list. Every change makes a new definition and leaves the old
 * one as it was. Steps are found by id wherever they sit.
 */
import type { ProcedureDefinition, ProcedureStep, ProcedureStepKind } from "@/lib/api/procedures";

import { newId } from "./workflows";

/*
 * Where is the place of a new step: the top level, or one side of a
 * condition or branch.
 */
export type Where = { parent: string; side: "then_steps" | "else_steps" } | null;

/*
 * mapSteps rebuilds a list of steps, nested ones included, with fn
 * applied to each list on the way back up.
 */
function mapLists(steps: ProcedureStep[], fn: (list: ProcedureStep[]) => ProcedureStep[]): ProcedureStep[] {
  return fn(
    steps.map((step) => {
      const out = { ...step };
      if (step.then_steps) {
        out.then_steps = mapLists(step.then_steps, fn);
      }
      if (step.else_steps) {
        out.else_steps = mapLists(step.else_steps, fn);
      }
      return out;
    }),
  );
}

/*
 * walk visits every step in reading order with how deep it sits.
 */
export function walk(steps: ProcedureStep[], fn: (step: ProcedureStep, depth: number) => void, depth = 0): void {
  for (const step of steps) {
    fn(step, depth);
    walk(step.then_steps ?? [], fn, depth + 1);
    walk(step.else_steps ?? [], fn, depth + 1);
  }
}

export function newStep(kind: ProcedureStepKind): ProcedureStep {
  const step: ProcedureStep = { id: newId("s_"), kind, text: "" };
  if (kind === "branch") {
    step.condition = { mode: "all", rules: [{ field: "", operator: "eq", value: "" }] };
    step.then_steps = [];
    step.else_steps = [];
  }
  return step;
}

export function withNewStep(d: ProcedureDefinition, kind: ProcedureStepKind, where: Where = null): ProcedureDefinition {
  const step = newStep(kind);
  if (!where) {
    return { ...d, steps: [...d.steps, step] };
  }

  return withStepChange(d, where.parent, (parent) => ({ [where.side]: [...(parent[where.side] ?? []), step] }));
}

export function withStepChange(
  d: ProcedureDefinition,
  id: string,
  change: Partial<ProcedureStep> | ((step: ProcedureStep) => Partial<ProcedureStep>),
): ProcedureDefinition {
  return {
    ...d,
    steps: mapLists(d.steps, (list) =>
      list.map((step) =>
        step.id === id ? { ...step, ...(typeof change === "function" ? change(step) : change) } : step,
      ),
    ),
  };
}

export function withoutStepById(d: ProcedureDefinition, id: string): ProcedureDefinition {
  return { ...d, steps: mapLists(d.steps, (list) => list.filter((step) => step.id !== id)) };
}

/*
 * withStepMoved moves a step one place up or down among its siblings,
 * staying put at either end.
 */
export function withStepMoved(d: ProcedureDefinition, id: string, by: -1 | 1): ProcedureDefinition {
  let moved = false;
  const steps = mapLists(d.steps, (list) => {
    const at = list.findIndex((step) => step.id === id);
    const to = at + by;
    if (at < 0 || to < 0 || to >= list.length) {
      return list;
    }
    const out = [...list];
    const [step] = out.splice(at, 1);
    if (step) {
      out.splice(to, 0, step);
      moved = true;
    }
    return out;
  });

  return moved ? { ...d, steps } : d;
}

export function withHandover(d: ProcedureDefinition, index: number, text: string | null): ProcedureDefinition {
  if (text === null) {
    return { ...d, handover: d.handover.filter((_, at) => at !== index) };
  }
  if (index >= d.handover.length) {
    return { ...d, handover: [...d.handover, text] };
  }

  return { ...d, handover: d.handover.map((line, at) => (at === index ? text : line)) };
}

/*
 * stepNumbers numbers every step as dimmi reads it: 2, then 2.1 for the
 * first step under step 2.
 */
export function stepNumbers(steps: ProcedureStep[], prefix = ""): Map<string, string> {
  const out = new Map<string, string>();
  steps.forEach((step, index) => {
    const label = `${prefix}${index + 1}`;
    out.set(step.id, label);
    for (const [id, nested] of stepNumbers([...(step.then_steps ?? []), ...(step.else_steps ?? [])], `${label}.`)) {
      out.set(id, nested);
    }
  });
  return out;
}

/*
 * actionsOf lists the actions a step's chips and legacy action name.
 */
export function actionsOf(step: ProcedureStep): string[] {
  const out = new Set<string>();
  if (step.action_id) {
    out.add(step.action_id);
  }
  for (const binding of step.bindings ?? []) {
    if (binding.kind === "action") {
      out.add(binding.ref);
    }
  }
  return [...out];
}

/*
 * earlierActions lists the actions of every step before the one given,
 * in reading order, for its prerequisites.
 */
export function earlierActions(d: ProcedureDefinition, id: string): string[] {
  const out: string[] = [];
  let reached = false;
  walk(d.steps, (step) => {
    if (step.id === id) {
      reached = true;
    }
    if (!reached) {
      for (const action of actionsOf(step)) {
        if (!out.includes(action)) {
          out.push(action);
        }
      }
    }
  });
  return out;
}

/*
 * StepChange is how one step differs between two versions.
 */
export interface StepChange {
  id: string;
  label: string;
  change: "added" | "removed" | "changed" | "same";
  before?: ProcedureStep;
  after?: ProcedureStep;
}

function flat(steps: ProcedureStep[]): Map<string, ProcedureStep> {
  const out = new Map<string, ProcedureStep>();
  walk(steps, (step) => {
    out.set(step.id, { ...step, then_steps: undefined, else_steps: undefined });
  });
  return out;
}

/*
 * diffSteps compares two versions step by step, in the newer one's
 * order with removed steps last.
 */
export function diffSteps(before: ProcedureDefinition, after: ProcedureDefinition): StepChange[] {
  const old = flat(before.steps);
  const now = flat(after.steps);
  const labels = stepNumbers(after.steps);
  const oldLabels = stepNumbers(before.steps);
  const out: StepChange[] = [];
  for (const [id, step] of now) {
    const was = old.get(id);
    const label = labels.get(id) ?? "";
    if (!was) {
      out.push({ id, label, change: "added", after: step });
    } else {
      const same = JSON.stringify(was) === JSON.stringify(step);
      out.push({ id, label, change: same ? "same" : "changed", before: was, after: step });
    }
  }
  for (const [id, step] of old) {
    if (!now.has(id)) {
      out.push({ id, label: oldLabels.get(id) ?? "", change: "removed", before: step });
    }
  }
  return out;
}
