import { describe, expect, it } from "vitest";

import { fitOperator, operatorsFor, readTyped, typeOfValue } from "./rules";

describe("typed rules", () => {
  it("offers only comparisons that fit the field", () => {
    expect(operatorsFor("boolean").map(([id]) => id)).toEqual(["eq", "neq", "exists", "not_exists"]);
    expect(operatorsFor("number").map(([id]) => id)).toContain("gte");
    expect(operatorsFor("text").map(([id]) => id)).not.toContain("gte");
    expect(fitOperator("boolean", "gte")).toBe("eq");
    expect(fitOperator("number", "gte")).toBe("gte");
  });

  it("reads a value by the field's type", () => {
    expect(readTyped("boolean", "eq", "true")).toBe(true);
    expect(readTyped("boolean", "eq", "false")).toBe(false);
    expect(readTyped("number", "gte", " 5000 ")).toBe(5000);
    expect(readTyped("text", "eq", "02134")).toBe("02134");
    expect(readTyped("choice", "in", "growth, pro")).toEqual(["growth", "pro"]);
  });

  it("guesses a field's type from a value it held", () => {
    expect(typeOfValue(true)).toBe("boolean");
    expect(typeOfValue(12)).toBe("number");
    expect(typeOfValue("shipped")).toBe("text");
  });
});
