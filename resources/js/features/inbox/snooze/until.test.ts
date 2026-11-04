import { CalendarDate } from "@internationalized/date";
import { describe, expect, it } from "vitest";

import { untilLabel, untilOf } from "./until";

describe("snooze moments", () => {
  it("turns a picked day and time into the moment in the reader's zone", () => {
    expect(untilOf(new CalendarDate(2026, 11, 2), "09:30", "America/New_York")).toBe("2026-11-02T14:30:00.000Z");
    expect(untilOf(new CalendarDate(2026, 11, 2), "9am", "America/New_York")).toBeNull();
  });

  it("names the weekday and time a snooze ends", () => {
    expect(untilLabel("2026-11-02T14:30:00.000Z", "en-US")).toMatch(/Mon/);
  });
});
