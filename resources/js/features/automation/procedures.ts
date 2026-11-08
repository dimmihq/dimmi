/*
 * Changes to a procedure while it is edited: steps added, removed,
 * moved, and rewritten, and the handover list. Every change makes a new
 * definition and leaves the old one as it was.
 */
import type { ProcedureDefinition, ProcedureStep, ProcedureStepKind } from "@/lib/api/procedures";

import { newId } from "./workflows";

export function withNewStep(d: ProcedureDefinition, kind: ProcedureStepKind): ProcedureDefinition {
  return { ...d, steps: [...d.steps, { id: newId("s_"), kind, text: "" }] };
}

export function withStepChange(
  d: ProcedureDefinition,
  id: string,
  change: Partial<ProcedureStep>,
): ProcedureDefinition {
  return { ...d, steps: d.steps.map((step) => (step.id === id ? { ...step, ...change } : step)) };
}

export function withoutStepById(d: ProcedureDefinition, id: string): ProcedureDefinition {
  return { ...d, steps: d.steps.filter((step) => step.id !== id) };
}

/*
 * withStepMoved moves a step one place up or down, staying put at either
 * end.
 */
export function withStepMoved(d: ProcedureDefinition, id: string, by: -1 | 1): ProcedureDefinition {
  const at = d.steps.findIndex((step) => step.id === id);
  const to = at + by;
  if (at < 0 || to < 0 || to >= d.steps.length) {
    return d;
  }
  const steps = [...d.steps];
  const [moved] = steps.splice(at, 1);
  if (moved) {
    steps.splice(to, 0, moved);
  }

  return { ...d, steps };
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
