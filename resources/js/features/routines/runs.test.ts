import { describe, expect, it } from "vitest";

import { tookOf } from "./runs";

describe("tookOf", () => {
  it("says how long a finished run took", () => {
    expect(tookOf({ started_at: "2026-11-23T09:00:00Z", completed_at: "2026-11-23T09:00:42Z" })).toBe("took 42 s");
    expect(tookOf({ started_at: "2026-11-23T09:00:00Z", completed_at: "2026-11-23T09:01:12Z" })).toBe(
      "took 1 min 12 s",
    );
    expect(tookOf({ started_at: "2026-11-23T09:00:00Z", completed_at: "2026-11-23T09:02:00Z" })).toBe("took 2 min");
  });

  it("says nothing while a run has not finished", () => {
    expect(tookOf({ started_at: "2026-11-23T09:00:00Z", completed_at: null })).toBe("");
  });
});
