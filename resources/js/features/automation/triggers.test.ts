import { isTrigger, tintOf, triggerGroups, triggers, triggersIn } from "./triggers";
import { blankWorkflow, workflows } from "./workflows";

describe("triggers", () => {
  it("offers every trigger in exactly one group", () => {
    const offered = triggerGroups.flatMap((group) => triggersIn(group.id));

    expect(offered.sort()).toEqual(Object.keys(triggers).sort());
  });

  it("narrows a group to the triggers matching every searched word", () => {
    expect(triggersIn("during", "stops replying")).toEqual(["customer-quiet"]);
    expect(triggersIn("team", "STATE")).toEqual(["state"]);
    expect(triggersIn("dimmi", "dimmi")).toEqual(["dimmi-handover", "dimmi-closed"]);
    expect(triggersIn("start", "invoice")).toEqual([]);
  });

  it("colours a trigger's mark by its group", () => {
    expect(tintOf("first-message")).toBe("blue");
    expect(tintOf("dimmi-handover")).toBe("lime");
  });

  it("only accepts the triggers it knows", () => {
    expect(isTrigger("page")).toBe(true);
    expect(isTrigger("toString")).toBe(false);
    expect(isTrigger(undefined)).toBe(false);
  });

  it("starts every example workflow from a known trigger", () => {
    expect(workflows.every((workflow) => isTrigger(workflow.trigger))).toBe(true);
  });

  it("makes a new workflow an empty draft with the picked trigger", () => {
    expect(blankWorkflow("rated")).toMatchObject({ trigger: "rated", status: "draft", steps: [] });
  });
});
