import { findStep, findWorkflow, workflows } from "./workflows";

describe("findStep", () => {
  it("finds a step at the top of a workflow", () => {
    const workflow = findWorkflow("nudge");

    expect(findStep(workflow?.steps ?? [], "n2")?.kind).toBe("message");
  });

  it("finds a step down nested paths", () => {
    const workflow = findWorkflow("first-message");

    expect(findStep(workflow?.steps ?? [], "s6")?.summary).toContain("email");
  });

  it("returns nothing for an unknown step", () => {
    expect(findStep(workflows[0]?.steps ?? [], "missing")).toBeUndefined();
  });
});
