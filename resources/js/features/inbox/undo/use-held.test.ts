import { describe, expect, it } from "vitest";

import { secondsLeft } from "./use-held";

describe("seconds left", () => {
  it("counts whole seconds up to the moment, and none past it", () => {
    const now = Date.parse("2026-11-04T10:00:00Z");

    expect(secondsLeft("2026-11-04T10:00:05Z", now)).toBe(5);
    expect(secondsLeft("2026-11-04T10:00:04.200Z", now)).toBe(5);
    expect(secondsLeft("2026-11-04T09:59:59Z", now)).toBe(0);
  });
});
