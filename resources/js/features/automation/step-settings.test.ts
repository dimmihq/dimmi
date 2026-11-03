import { withEdited } from "./edit-steps";
import { fieldsOf, unsetSummary, withField, withPathLabel } from "./step-settings";
import { findStep, findWorkflow, type Step } from "./workflows";

const steps = findWorkflow("first-message")?.steps ?? [];

describe("fieldsOf", () => {
  it("reads the settings of an example step back from its sentence", () => {
    expect(fieldsOf({ kind: "assign", summary: "Assign to the Billing team" })).toEqual({ assignee: "Billing team" });
    expect(fieldsOf({ kind: "wait", summary: "Wait 1 day" })).toEqual({ wait: "1 day", stop: "on" });
  });

  it("starts a new message with nothing written", () => {
    expect(fieldsOf({ kind: "message", summary: unsetSummary })).toEqual({ text: "" });
  });
});

describe("withField", () => {
  const message: Step = { id: "m", kind: "message", summary: unsetSummary };

  it("puts what is typed on the card", () => {
    expect(withField(message, "text", "Hi there").summary).toBe("Hi there");
  });

  it("says the step is not set up once its text is cleared", () => {
    expect(withField(withField(message, "text", "Hi"), "text", "  ").summary).toBe(unsetSummary);
  });

  it("describes a picked option in the card's words", () => {
    const assign: Step = { id: "a", kind: "assign", summary: "Assign to Sam Okafor" };

    expect(withField(assign, "assignee", "Support team").summary).toBe("Assign to the Support team");
    expect(withField(assign, "assignee", "Lea Martin").summary).toBe("Assign to Lea Martin");
  });

  it("keeps the other settings when one changes", () => {
    const wait = withField({ id: "w", kind: "wait", summary: "Wait 1 hour" }, "stop", "off");

    expect(wait.summary).toBe("Wait 1 hour, even if they reply");
    expect(withField(wait, "wait", "1 day").summary).toBe("Wait 1 day, even if they reply");
  });

  it("leaves the sentence of a kind whose settings do not change it", () => {
    const branch: Step = { id: "b", kind: "branch", summary: "Company plan" };

    expect(withField(branch, "anything", "x").summary).toBe("Company plan");
  });
});

describe("withPathLabel", () => {
  it("renames the path a reply button leads down", () => {
    const edited = withEdited(steps, "s1", (step) => withPathLabel(step, "p1", "Invoices"));

    expect(findStep(edited, "s1")?.paths?.[0]?.label).toBe("Invoices");
    expect(findStep(steps, "s1")?.paths?.[0]?.label).toBe("Billing");
  });
});

describe("withEdited", () => {
  it("edits a step deep in the tree and leaves the rest", () => {
    const edited = withEdited(steps, "s6", (step) => withField(step, "saveAs", "Name"));

    expect(findStep(edited, "s6")?.summary).toBe('Ask "What\'s the best email to reach you?" → Name');
    expect(findStep(edited, "s7")).toEqual(findStep(steps, "s7"));
  });
});
