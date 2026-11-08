import { freshStep, withEdited, withoutStep, withStep } from "./edit-steps";
import { findStep, type Step } from "./workflows";

const steps: Step[] = [
  {
    id: "s_branch01",
    kind: "branch",
    settings: {},
    paths: [
      { id: "p_refund1", label: "Refund", steps: [{ id: "s_tag0001", kind: "tag", settings: { tag_ids: [] } }] },
      { id: "p_else001", outcome: "else", steps: [] },
    ],
  },
];

describe("withStep", () => {
  it("adds a step at the top level", () => {
    const next = withStep(steps, null, { id: "s_close001", kind: "close" });

    expect(next.at(-1)?.id).toBe("s_close001");
  });

  it("adds a step to the end of a nested path", () => {
    const next = withStep(steps, "p_else001", { id: "s_note0001", kind: "note" });

    expect(findStep(next, "s_note0001")?.kind).toBe("note");
    expect(findStep(steps, "s_note0001")).toBeUndefined();
  });
});

describe("withoutStep", () => {
  it("removes a nested step and leaves the rest", () => {
    const next = withoutStep(steps, "s_tag0001");

    expect(findStep(next, "s_tag0001")).toBeUndefined();
    expect(findStep(next, "s_branch01")).toBeDefined();
  });
});

describe("withEdited", () => {
  it("edits a step deep in the tree and leaves the rest", () => {
    const next = withEdited(steps, "s_tag0001", (step) => ({ ...step, settings: { tag_ids: ["tag_1"] } }));

    expect(findStep(next, "s_tag0001")?.settings).toEqual({ tag_ids: ["tag_1"] });
    expect(findStep(steps, "s_tag0001")?.settings).toEqual({ tag_ids: [] });
  });
});

describe("freshStep", () => {
  it("gives reply buttons two buttons", () => {
    expect(freshStep("buttons").paths?.map((path) => path.label)).toEqual(["Option 1", "Option 2"]);
  });

  it("gives a branch one condition and everyone else", () => {
    const branch = freshStep("branch");

    expect(branch.settings).toEqual({});
    expect(branch.paths?.map((path) => path.outcome ?? path.label)).toEqual(["Path 1", "else"]);
  });

  it("gives a plain step no paths and the settings it starts with", () => {
    expect(freshStep("wait").paths).toBeUndefined();
    expect(freshStep("wait").settings).toEqual({ for: "1h" });
    expect(freshStep("wait").id).toMatch(/^s_/);
  });
});
