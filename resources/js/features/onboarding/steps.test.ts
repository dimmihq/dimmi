import { describe, expect, it } from "vitest";

import { nextStep, position, previousStep, stepFrom, steps } from "./steps";

describe("onboarding steps", () => {
  it("walks forward to the summary and back to the start", () => {
    expect(nextStep("workspace")).toBe("logo");
    expect(nextStep("test_conversations")).toBe("done");
    expect(previousStep("logo")).toBe("workspace");
    expect(previousStep("workspace")).toBe("workspace");
  });

  it("numbers the steps from one and reads an unknown step as the first", () => {
    expect(position("workspace")).toBe(1);
    expect(position("done")).toBe(steps.length);
    expect(stepFrom("tiers")).toBe("workspace");
    expect(stepFrom("tags")).toBe("tags");
  });
});
