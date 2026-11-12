import { describe, expect, it } from "vitest";

import type { AttributeDefinition } from "@/lib/api/attributes";

import {
  attributeKey,
  fieldChoices,
  initialFields,
  mappingProblem,
  planFrom,
  previewSummary,
  skipField,
} from "./import-plan";

const plan: AttributeDefinition = {
  id: "attr_1",
  of: "contact",
  key: "plan",
  label: "Plan",
  description: null,
  data_type: "string",
  options: [],
  origin: "created",
  status: "active",
  client_writable: false,
  last_value_at: null,
};

describe("import plan", () => {
  it("offers the kind's fields and its active attributes", () => {
    const tier = { ...plan, id: "attr_2", of: "company" as const, key: "tier", label: "Tier" };
    const archived = { ...plan, id: "attr_3", key: "old", status: "archived" as const };
    const people = fieldChoices("contacts", [plan, tier, archived]).map((c) => c.id);
    expect(people).toContain("email");
    expect(people).toContain("attribute.plan");
    expect(people).not.toContain("attribute.tier");
    expect(people).not.toContain("attribute.old");
    expect(fieldChoices("companies", [plan, tier]).map((c) => c.id)).toEqual([
      "name",
      "domain",
      "external_id",
      "tags",
      "attribute.tier",
    ]);
  });

  it("turns headers into attribute keys", () => {
    expect(attributeKey("Seats bought")).toBe("seats_bought");
    expect(attributeKey("  #1 Fan! ")).toBe("field_1_fan");
  });

  it("maps suggested columns and leaves skipped ones out", () => {
    const fields = initialFields(
      ["Email", "Notes", "Name"],
      [
        { column: 0, field: "email" },
        { column: 2, field: "name" },
      ],
    );
    expect(fields).toEqual(["email", skipField, "name"]);
    expect(planFrom(fields)).toEqual([
      { column: 0, field: "email" },
      { column: 2, field: "name" },
    ]);
  });

  it("asks for a key column, single use of each field and named new attributes", () => {
    expect(mappingProblem("contacts", ["name"])).toMatch(/email or external ID/);
    expect(mappingProblem("companies", ["name", "external_id"])).toBeNull();
    expect(mappingProblem("contacts", ["email", "email"])).toMatch(/one column/);
    expect(mappingProblem("contacts", ["email", "attribute."])).toMatch(/Name each/);
  });

  it("words a dry run", () => {
    expect(previewSummary({ rows: 4, created: 1, updated: 1, skipped: 0, failed: 2, errors: [] }, "people")).toBe(
      "4 people: 1 new, 1 updated, 2 with problems.",
    );
  });
});
