import { describe, expect, it } from "vitest";

import type { AudienceField } from "@/lib/api/audiences";

import {
  fieldGroup,
  fieldName,
  sentence,
  shapeOf,
  summary,
  toForm,
  toRules,
  withField,
  withOp,
  withWindow,
} from "./rules";

const plan = { field: "contact.custom.plan", op: "eq", value: "pro" };
const seen = { field: "contact.last_seen_at", op: "within_last_days", value: 30 };

describe("toForm and toRules", () => {
  it("round-trips conditions and groups matched all, any or none", () => {
    const rules = {
      visitors: "identified" as const,
      rules: { all: [plan, { any: [seen, plan] }, { not: { any: [plan] } }] },
    };
    const form = toForm(rules);

    expect(form?.items.map((i) => (i.kind === "group" ? i.match : "condition"))).toEqual(["condition", "any", "none"]);
    expect(form && toRules(form)).toEqual(rules);
  });

  it("reads a lone condition and no rule at all", () => {
    expect(toForm({ rules: plan })?.items).toHaveLength(1);
    expect(toForm(undefined)).toEqual({ visitors: "everyone", match: "all", items: [] });
    expect(toRules({ visitors: "leads", match: "any", items: [] })).toEqual({ visitors: "leads" });
  });

  it("leaves alone a tree deeper than the builder shows", () => {
    expect(toForm({ rules: { all: [{ any: [{ all: [plan] }] }] } })).toBeNull();
  });

  it("drops empty groups when writing", () => {
    expect(
      toRules({
        visitors: "everyone",
        match: "all",
        items: [{ kind: "group", id: "g", match: "any", conditions: [] }],
      }),
    ).toEqual({ visitors: "everyone" });
  });
});

describe("fields and operators", () => {
  const number: AudienceField = {
    key: "contact.custom.seats",
    type: "number",
    where: "server",
    trust: "claimed",
    ops: ["eq", "gt"],
    custom: true,
  };

  it("names and groups fields, custom attributes in words", () => {
    expect(fieldName("contact.custom.trial_ends_at")).toBe("Trial ends at");
    expect(fieldName("company.custom.plan")).toBe("Company plan");
    expect(fieldGroup("page.path")).toBe("Page");
    expect(fieldGroup("audience")).toBe("Tags and audiences");
  });

  it("starts a condition from the field's first operator and keeps fitting values", () => {
    expect(withField(number)).toEqual({ field: "contact.custom.seats", op: "eq", value: 0 });
    expect(withOp({ field: "x", op: "gt", value: 5 }, "lte", "number").value).toBe(5);
    expect(withOp({ field: "x", op: "gt", value: 5 }, "exists", "number")).toEqual({ field: "x", op: "exists" });
    expect(shapeOf("within_last_days")).toBe("days");
    expect(shapeOf("eq", "number")).toBe("number");
  });

  it("reads an audience as one line", () => {
    expect(summary({ visitors: "identified", rules: { any: [plan, seen] } })).toBe(
      "Signed-in users where Plan is pro or Last seen is within the last 30 days",
    );
    expect(summary({ rules: { not: { any: [plan] } } })).toBe("Everyone where none of: Plan is pro");
    expect(summary({ rules: { field: "audience", op: "in", value: ["audn_1"] } }, { audn_1: "VIP" })).toBe(
      "Everyone where Saved audience is any of VIP",
    );
  });
});

describe("event conditions", () => {
  it("names event fields and reads a window", () => {
    const count = { field: "event.report.exported.count", op: "gte", value: 3, within_days: 7 };

    expect(fieldName("event.report.exported.last_at")).toBe("report.exported: last time");
    expect(fieldGroup("event.x.count")).toBe("Your events");
    expect(fieldGroup("contact.custom.plan")).toBe("Your person attributes");
    expect(fieldGroup("company.custom.seats")).toBe("Your company attributes");
    expect(fieldGroup("contact.email")).toBe("Person");
    expect(sentence(count)).toBe("report.exported: times is at least 3 in the last 7 days");
    expect(withWindow(count, undefined)).toEqual({ field: "event.report.exported.count", op: "gte", value: 3 });
    expect(withOp(count, "lt", "number").within_days).toBe(7);
  });
});
