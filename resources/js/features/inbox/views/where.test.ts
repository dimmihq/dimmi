import { describe, expect, it } from "vitest";

import { decodeWhere, encodeWhere, hasConditions } from "./where";

const definition = {
  op: "and" as const,
  groups: [{ op: "and" as const, conditions: [{ field: "subject", op: "contains", value: "café, refund" }] }],
};

describe("where", () => {
  it("round-trips a definition, commas and accents included", () => {
    const encoded = encodeWhere(definition);

    expect(encoded).not.toMatch(/[,+/=]/);
    expect(decodeWhere(encoded)).toEqual(definition);
  });

  it("ignores anything that is not a definition", () => {
    expect(decodeWhere(undefined)).toBeNull();
    expect(decodeWhere("not base64!")).toBeNull();
    expect(decodeWhere(encodeWhere({ op: "and", groups: [] }).slice(0, 3))).toBeNull();
  });

  it("tells an empty definition apart", () => {
    expect(hasConditions(definition)).toBe(true);
    expect(hasConditions({ op: "and", groups: [{ op: "and", conditions: [] }] })).toBe(false);
  });
});
