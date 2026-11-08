import type { ProcedureDefinition } from "@/lib/api/procedures";

import { withHandover, withNewStep, withoutStepById, withStepChange, withStepMoved } from "./procedures";

const d: ProcedureDefinition = {
  description: "Sign-in trouble",
  steps: [
    { id: "s_a", kind: "instruction", text: "Ask for the email." },
    { id: "s_b", kind: "action", text: "Look up the account.", action_id: "aac_1" },
  ],
  handover: ["Lost the mailbox"],
};

describe("procedure edits", () => {
  it("adds a step of a kind at the end with a fresh id", () => {
    const next = withNewStep(d, "condition");

    expect(next.steps.at(-1)).toMatchObject({ kind: "condition", text: "" });
    expect(next.steps.at(-1)?.id).toMatch(/^s_/);
    expect(d.steps).toHaveLength(2);
  });

  it("rewrites, removes and moves steps", () => {
    expect(withStepChange(d, "s_a", { text: "Ask nicely." }).steps[0]?.text).toBe("Ask nicely.");
    expect(withoutStepById(d, "s_a").steps.map((s) => s.id)).toEqual(["s_b"]);
    expect(withStepMoved(d, "s_b", -1).steps.map((s) => s.id)).toEqual(["s_b", "s_a"]);
    expect(withStepMoved(d, "s_a", -1)).toBe(d);
  });

  it("adds, edits and removes handovers", () => {
    expect(withHandover(d, 1, "Two codes failed").handover).toEqual(["Lost the mailbox", "Two codes failed"]);
    expect(withHandover(d, 0, "Changed").handover).toEqual(["Changed"]);
    expect(withHandover(d, 0, null).handover).toEqual([]);
  });
});
