import { allowsStep, isTrigger, tintOf, triggerGroups, triggers, triggersIn } from "./triggers";

describe("triggers", () => {
  it("offers every trigger that runs in exactly one group", () => {
    const offered = triggerGroups.flatMap((group) => triggersIn(group.id));
    const running = Object.entries(triggers)
      .filter(([, trigger]) => trigger.runs)
      .map(([kind]) => kind);

    expect(offered.sort()).toEqual(running.sort());
  });

  it("offers the quiet triggers", () => {
    expect(triggersIn("during", "stops replying")).toEqual(["conversation.customer_quiet"]);
    expect(triggersIn("team", "waits")).toEqual(["conversation.teammate_quiet"]);
  });

  it("narrows a group to the triggers matching every searched word", () => {
    expect(triggersIn("team", "STATUS")).toEqual(["conversation.status_changed"]);
    expect(triggersIn("start", "closed conversation")).toEqual(["conversation.customer_reopened"]);
    expect(triggersIn("start", "invoice")).toEqual([]);
  });

  it("leaves out triggers that do not run yet", () => {
    expect(triggersIn("during", "rates")).toEqual([]);
    expect(triggersIn("sla")).toEqual([]);
  });

  it("offers dimmi's triggers and reusable workflows", () => {
    expect(triggersIn("dimmi").sort()).toEqual(["dimmi.handed_over", "dimmi.resolved"]);
    expect(triggersIn("reuse")).toEqual(["reusable"]);
  });

  it("colours a trigger's mark by its group", () => {
    expect(tintOf("conversation.first_message")).toBe("blue");
    expect(tintOf("dimmi.handed_over")).toBe("lime");
  });

  it("lets only triggers that may speak use customer steps", () => {
    expect(allowsStep("conversation.first_message", "buttons")).toBe(true);
    expect(allowsStep("message.customer", "message")).toBe(false);
    expect(allowsStep("message.customer", "tag")).toBe(true);
    expect(allowsStep("conversation.first_message", "dimmi")).toBe(true);
    expect(allowsStep("note.created", "dimmi")).toBe(false);
  });

  it("only accepts the triggers it knows", () => {
    expect(isTrigger("note.created")).toBe(true);
    expect(isTrigger("toString")).toBe(false);
    expect(isTrigger(undefined)).toBe(false);
  });
});
