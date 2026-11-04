import { describe, expect, it } from "vitest";

import { waitedFor } from "./conversations";

describe("waitedFor", () => {
  const now = Date.parse("2026-11-04T12:00:00Z");

  it("reads in minutes, then hours, then days", () => {
    expect(waitedFor("2026-11-04T11:59:40Z", now)).toBe("1 m");
    expect(waitedFor("2026-11-04T11:15:00Z", now)).toBe("45 m");
    expect(waitedFor("2026-11-04T09:00:00Z", now)).toBe("3 h");
    expect(waitedFor("2026-11-01T12:00:00Z", now)).toBe("3 d");
  });
});
