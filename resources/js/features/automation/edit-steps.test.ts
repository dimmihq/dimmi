import { freshStep, withoutStep, withStep } from "./edit-steps";
import { findStep, findWorkflow } from "./workflows";

const steps = findWorkflow("first-message")?.steps ?? [];

describe("withStep", () => {
  it("adds a step at the top level", () => {
    const next = withStep(steps, null, freshStep("close", "x1"));

    expect(next.at(-1)?.id).toBe("x1");
  });

  it("adds a step to the end of a nested path", () => {
    const next = withStep(steps, "p5", freshStep("tag", "x2"));

    expect(findStep(next, "x2")?.kind).toBe("tag");
    expect(findStep(steps, "x2")).toBeUndefined();
  });
});

describe("withoutStep", () => {
  it("removes a nested step and leaves the rest", () => {
    const next = withoutStep(steps, "s6");

    expect(findStep(next, "s6")).toBeUndefined();
    expect(findStep(next, "s7")).toBeDefined();
  });
});

describe("freshStep", () => {
  it("gives Dimmi its three outcomes", () => {
    expect(freshStep("dimmi", "d").paths?.map((path) => path.label)).toEqual(["Resolved", "Hands over", "No answer"]);
  });

  it("gives a plain step no paths", () => {
    expect(freshStep("wait", "w").paths).toBeUndefined();
  });

  it("describes the settings a new step starts with, or asks for text", () => {
    expect(freshStep("tag", "t").summary).toBe("Tag VIP");
    expect(freshStep("message", "m").summary).toBe("Not set up yet");
  });
});
