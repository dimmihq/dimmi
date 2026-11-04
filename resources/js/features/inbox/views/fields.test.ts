import { describe, expect, it } from "vitest";

import { blankDefinition, defaultValue, opFor, warned } from "./fields";

describe("view editor fields", () => {
  it("starts a view with one open-status condition", () => {
    expect(blankDefinition().groups[0].conditions[0]).toEqual({ field: "status", op: "in", value: ["open"] });
  });

  it("finds an operator, falling back to the field's first", () => {
    expect(opFor({ field: "assignee", op: "not_exists" })?.label).toBe("is nobody");
    expect(opFor({ field: "assignee", op: "gte" })?.op).toBe("in");
    expect(opFor({ field: "nowhere", op: "eq" })).toBeUndefined();
  });

  it("gives each kind of value a starting value", () => {
    expect(defaultValue("member")).toEqual(["me"]);
    expect(defaultValue("flag")).toBe(true);
    expect(defaultValue(undefined)).toBeUndefined();
  });

  it("matches a warning to its condition", () => {
    expect(warned([{ path: "groups.1.conditions.0" }], 1, 0)).toBe(true);
    expect(warned([{ path: "groups.1.conditions.0" }], 0, 1)).toBe(false);
  });
});
