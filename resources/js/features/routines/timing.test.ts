import { expect, describe as group, it } from "vitest";

import { cronOf, instantOf, localInput, ordinal, summarize, type Timing, timingOf } from "./timing";

const base: Timing = {
  kind: "weekly",
  hour: 9,
  minute: 0,
  weekday: 1,
  monthDay: 1,
  cron: "",
  runAt: "",
  timezone: "UTC",
};

group("cronOf", () => {
  it("writes the presets", () => {
    expect(cronOf({ ...base, kind: "daily", hour: 8, minute: 30 })).toBe("30 8 * * *");
    expect(cronOf({ ...base, kind: "weekly", weekday: 5, hour: 15 })).toBe("0 15 * * 5");
    expect(cronOf({ ...base, kind: "monthly", monthDay: 0 })).toBe("0 9 L * *");
    expect(cronOf({ ...base, kind: "monthly", monthDay: 14 })).toBe("0 9 14 * *");
    expect(cronOf({ ...base, kind: "once" })).toBe("");
  });
});

group("timingOf", () => {
  it("reads presets back", () => {
    expect(timingOf("weekly", "0 9 * * 1", null, "UTC")).toMatchObject({ kind: "weekly", hour: 9, weekday: 1 });
    expect(timingOf("monthly", "15 7 L * *", null, "UTC")).toMatchObject({ kind: "monthly", monthDay: 0, minute: 15 });
    expect(timingOf("daily", "30 8 * * *", null, "UTC")).toMatchObject({ kind: "daily", hour: 8, minute: 30 });
  });

  it("keeps what the presets cannot show as a custom cron", () => {
    expect(timingOf("weekly", "0 9 * * 1-5", null, "UTC").kind).toBe("cron");
    expect(timingOf("cron", "0 */6 * * *", null, "UTC")).toMatchObject({ kind: "cron", cron: "0 */6 * * *" });
  });
});

group("describe", () => {
  it("says when it runs", () => {
    expect(summarize({ ...base, weekday: 3 })).toBe("Every Wednesday at 09:00");
    expect(summarize({ ...base, kind: "monthly", monthDay: 0 })).toBe("On the last day of each month at 09:00");
    expect(summarize({ ...base, kind: "monthly", monthDay: 22 })).toBe("On the 22nd of each month at 09:00");
  });

  it("writes ordinals", () => {
    expect([1, 2, 3, 4, 11, 12, 13, 21, 22, 23].map(ordinal)).toEqual([
      "1st",
      "2nd",
      "3rd",
      "4th",
      "11th",
      "12th",
      "13th",
      "21st",
      "22nd",
      "23rd",
    ]);
  });
});

group("one-off moments", () => {
  it("round-trips a wall time through the zone", () => {
    const at = instantOf("2026-12-01T10:30", "America/New_York");
    expect(at).toBe("2026-12-01T15:30:00.000Z");
    expect(localInput(at, "America/New_York")).toBe("2026-12-01T10:30");
  });
});
