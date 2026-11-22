import type { ProcedureDefinition } from "@/lib/api/procedures";

import {
  diffSteps,
  earlierActions,
  stepNumbers,
  withHandover,
  withNewStep,
  withoutStepById,
  withStepChange,
  withStepMoved,
  withStepReordered,
} from "./procedures";

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

describe("nested procedure steps", () => {
  const branch: ProcedureDefinition = {
    description: "Refunds",
    steps: [
      {
        id: "s_1",
        kind: "instruction",
        text: "Look it up with [[action:aac_lookup]].",
        bindings: [{ kind: "action", ref: "aac_lookup" }],
      },
      {
        id: "s_2",
        kind: "branch",
        text: "",
        condition: { mode: "all", rules: [{ field: "action.lookup.days", operator: "lte", value: 30 }] },
        then_steps: [
          { id: "s_3", kind: "instruction", text: "Refund", bindings: [{ kind: "action", ref: "aac_refund" }] },
          { id: "s_4", kind: "instruction", text: "Confirm" },
        ],
        else_steps: [],
      },
    ],
    handover: [],
  };

  it("adds, changes, moves and removes steps on either side of a branch", () => {
    const added = withNewStep(branch, "instruction", { parent: "s_2", side: "else_steps" });
    expect(added.steps[1]?.else_steps).toHaveLength(1);
    expect(withStepChange(branch, "s_4", { text: "Tell them" }).steps[1]?.then_steps?.[1]?.text).toBe("Tell them");
    expect(withStepMoved(branch, "s_4", -1).steps[1]?.then_steps?.map((s) => s.id)).toEqual(["s_4", "s_3"]);
    expect(withStepReordered(branch, "s_3", "s_4").steps[1]?.then_steps?.map((s) => s.id)).toEqual(["s_4", "s_3"]);
    expect(withStepReordered(branch, "s_1", "s_3")).toBe(branch);
    expect(withoutStepById(branch, "s_3").steps[1]?.then_steps?.map((s) => s.id)).toEqual(["s_4"]);
  });

  it("numbers nested steps and lists the actions before a step", () => {
    const numbers = stepNumbers(branch.steps);
    expect(numbers.get("s_3")).toBe("2.1");
    expect(numbers.get("s_4")).toBe("2.2");
    expect(earlierActions(branch, "s_3")).toEqual(["aac_lookup"]);
    expect(earlierActions(branch, "s_4")).toEqual(["aac_lookup", "aac_refund"]);
  });

  it("compares two versions step by step", () => {
    const after = withoutStepById(withStepChange(branch, "s_3", { text: "Refund it" }), "s_4");
    const changes = diffSteps(branch, withNewStep(after, "instruction"));
    expect(changes.find((c) => c.id === "s_3")?.change).toBe("changed");
    expect(changes.find((c) => c.id === "s_1")?.change).toBe("same");
    expect(changes.find((c) => c.id === "s_4")?.change).toBe("removed");
    expect(changes.filter((c) => c.change === "added")).toHaveLength(1);
  });
});
