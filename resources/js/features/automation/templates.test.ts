import { describe, expect, it } from "vitest";

import type { Spec } from "@/lib/api/workflows";

import { workflowTemplates } from "./templates";
import { allowsStep } from "./triggers";
import type { StepKind } from "./workflows";

const blank = (trigger: string): Spec => ({
  schema: 1,
  trigger,
  schedule: { mode: "any" },
  reentry: { mode: "every_time" },
  exclusive: false,
  steps: [],
});

describe("workflowTemplates", () => {
  it("builds only steps the template's trigger allows", () => {
    for (const template of workflowTemplates) {
      const spec = template.build(blank(template.trigger));
      expect(spec.trigger, template.id).toBe(template.trigger);
      expect(spec.steps.length, template.id).toBeGreaterThan(0);
      for (const step of spec.steps) {
        expect(allowsStep(template.trigger, step.kind as StepKind), `${template.id}: ${step.kind}`).toBe(true);
      }
    }
  });

  it("gives every template its own id", () => {
    const ids = workflowTemplates.map((t) => t.id);
    expect(new Set(ids).size).toBe(ids.length);
  });
});
