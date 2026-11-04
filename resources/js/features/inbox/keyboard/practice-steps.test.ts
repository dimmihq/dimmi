import { beforeEach, describe, expect, it } from "vitest";

import { advance, practiceSteps, savedStep, saveStep } from "./practice-steps";

describe("practice steps", () => {
  beforeEach(() => window.localStorage.clear());

  it("moves on only with the key asked for", () => {
    expect(advance(0, "x")).toBe(0);
    expect(advance(0, "J")).toBe(1);
    expect(advance(3, "?")).toBe(4);
  });

  it("remembers progress and ignores what it cannot read", () => {
    saveStep(2);
    expect(savedStep()).toBe(2);
    window.localStorage.setItem("dimmi:shortcuts:practice", String(practiceSteps.length + 3));
    expect(savedStep()).toBe(0);
  });
});
