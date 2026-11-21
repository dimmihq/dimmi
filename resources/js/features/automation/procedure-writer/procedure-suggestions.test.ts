import type { Action } from "@/lib/api/ai-actions";

import { actionHint, fieldsOf, findChips, labelOf, noSources } from "./procedure-suggestions";

const lookup = {
  id: "aac_1",
  name: "lookup_order",
  title: "Look up order",
  status: "published",
  side_effect: "read",
  confirmation: "none",
  approval: "none",
  last_test_response: { status: "shipped", shipping: { eta: "Monday" } },
} as unknown as Action;
const refund = {
  ...lookup,
  id: "aac_2",
  name: "refund_order",
  title: "Refund order",
  side_effect: "write",
  confirmation: "customer",
  approval: "teammate",
  last_test_response: undefined,
} as unknown as Action;
const sources = {
  ...noSources,
  actions: [lookup, refund],
  teams: [{ id: "team_1", name: "Billing" }],
  procedures: [{ id: "prc_1", name: "Verify identity" }],
};

describe("procedure chip suggestions", () => {
  it("groups chips and marks the first of each group", () => {
    const found = findChips(sources, "");
    expect(found[0]).toMatchObject({ group: "Actions", kind: "action", ref: "aac_1", first: true });
    expect(found[1]).toMatchObject({ group: "Actions", first: false });
    expect(found.find((c) => c.kind === "handoff")).toMatchObject({ group: "Inbox actions", label: "Billing" });
    expect(found.find((c) => c.kind === "sub_procedure")?.first).toBe(true);
  });

  it("filters by what was typed", () => {
    expect(findChips(sources, "refund").map((c) => c.ref)).toEqual(["aac_2"]);
  });

  it("says what an action does and who agrees", () => {
    expect(actionHint(lookup)).toBe("Read-only lookup");
    expect(actionHint(refund)).toBe("Changes the account · customer confirms · teammate approves");
  });

  it("offers the fields earlier actions returned to rules", () => {
    const fields = fieldsOf(sources).map((f) => f.field);
    expect(fields).toContain("action.lookup_order.status");
    expect(fields).toContain("action.lookup_order.shipping.eta");
    expect(labelOf(sources, { kind: "attribute", ref: "action.lookup_order.status" })).toBe("Look up order → status");
    expect(labelOf(sources, { kind: "action", ref: "aac_gone" })).toBe("");
  });
});
