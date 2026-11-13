import { describe, expect, it } from "vitest";

import type { TicketField, TicketState } from "@/lib/api/tickets";

import {
  addable,
  conditionMet,
  customerFields,
  fieldKey,
  stepOrder,
  typeChanges,
  typeSteps,
  withGroupOrder,
  withSteps,
} from "./form";

function field(key: string, extra: Partial<TicketField> = {}): TicketField {
  return {
    object: "ticket_field",
    id: `tktf_${key}`,
    type_id: "tktt_1",
    key,
    label: key,
    description: "",
    data_type: "short_text",
    options: [],
    currency: null,
    required_create: false,
    required_create_customer: false,
    required_resolve: false,
    visible_customer: true,
    editable_customer: true,
    show_when: null,
    ai_hint: null,
    ai_auto: false,
    position: 0,
    archived_at: null,
    ...extra,
  };
}

function state(id: string, group: TicketState["group"], position: number): TicketState {
  return {
    object: "ticket_state",
    id,
    group,
    name: id,
    customer_label: id,
    notify_customer: true,
    purpose: null,
    position,
    archived_at: null,
  };
}

describe("fieldKey", () => {
  it("makes a key from a label", () => {
    expect(fieldKey("Order number")).toBe("order_number");
    expect(fieldKey("  Refund reason (why?) ")).toBe("refund_reason_why");
    expect(fieldKey("2nd café")).toBe("nd_cafe");
  });
});

describe("conditionMet", () => {
  it("needs a value, or one of the listed ones", () => {
    expect(conditionMet(null, {})).toBe(true);
    expect(conditionMet({ field: "reason" }, {})).toBe(false);
    expect(conditionMet({ field: "reason" }, { reason: "x" })).toBe(true);
    expect(conditionMet({ field: "reason", equals: ["Other"] }, { reason: "Charged twice" })).toBe(false);
    expect(conditionMet({ field: "reason", equals: ["Other"] }, { reason: ["Other", "A"] })).toBe(true);
  });
});

describe("customerFields", () => {
  it("shows live customer fields whose condition holds", () => {
    const fields = [
      field("order"),
      field("internal", { visible_customer: false }),
      field("old", { archived_at: "2026-01-01" }),
      field("explain", { show_when: { field: "reason", equals: ["Other"] } }),
    ];
    expect(customerFields(fields, {}).map((f) => f.key)).toEqual(["order"]);
    expect(customerFields(fields, { reason: "Other" }).map((f) => f.key)).toEqual(["order", "explain"]);
  });
});

describe("typeChanges", () => {
  it("sends only what changed", () => {
    const saved = {
      name: "Refund",
      icon: "refund",
      description: "",
      inbox_id: null,
      team_id: null,
      customer_visible: true,
      company_sharing: false,
      reopen_days: 7,
      auto_resolve_days: null,
      auto_resolve_message: null,
      ai_auto_fill: false,
      state_ids: ["a", "b"],
      default_state_id: "a",
      default_resolved_state_id: "b",
    };
    expect(typeChanges(saved, { ...saved })).toEqual({});
    expect(typeChanges(saved, { ...saved, reopen_days: 3, state_ids: ["a"] })).toEqual({
      reopen_days: 3,
      state_ids: ["a"],
    });
  });
});

describe("withGroupOrder", () => {
  it("reorders one group and keeps the others", () => {
    const states = [state("new", "submitted", 0), state("x", "in_progress", 1), state("y", "in_progress", 2)];
    expect(withGroupOrder(states, "in_progress", ["y", "x"])).toEqual(["new", "y", "x"]);
  });
});

describe("type steps", () => {
  const state = (id: string, group: TicketState["group"], purpose: string | null = null): TicketState => ({
    object: "ticket_state",
    id,
    group,
    name: id,
    customer_label: id,
    notify_customer: true,
    purpose,
    position: 0,
    archived_at: null,
  });
  const states = [
    state("new", "submitted"),
    state("doing", "in_progress"),
    state("found", "in_progress"),
    state("info", "waiting_customer"),
    state("done", "resolved"),
    state("nope", "resolved"),
  ];

  it("keeps the type's order within each group and the groups in theirs", () => {
    const steps = typeSteps(["nope", "doing", "done", "new"], states);

    expect(stepOrder(steps)).toEqual(["new", "doing", "nope", "done"]);
  });

  it("starts in the first submitted state and closes to the first resolved one", () => {
    expect(withSteps(typeSteps(["new", "nope", "done"], states))).toEqual({
      state_ids: ["new", "nope", "done"],
      default_state_id: "new",
      default_resolved_state_id: "nope",
    });
  });

  it("offers the group's states the type does not use yet", () => {
    expect(addable(states, "in_progress", ["doing"]).map((s) => s.id)).toEqual(["found"]);
  });
});
