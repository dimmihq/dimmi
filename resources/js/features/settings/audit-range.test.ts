import { CalendarDate, today } from "@internationalized/date";

import { oldestDay, presetRanges, rangeFilters } from "./audit-range";

describe("audit range", () => {
  const now = new Date("2026-11-04T12:30:00Z");

  it("offers the last 24 hours, 7, 30 and 90 days", () => {
    const presets = presetRanges("UTC");

    expect(presets.map((p) => p.label)).toEqual(["Last 24 hours", "Last 7 days", "Last 30 days", "Last 90 days"]);
    expect(presets[1]?.range.end.compare(today("UTC"))).toBe(0);
    expect(presets[1]?.range.start.compare(today("UTC").subtract({ days: 7 }))).toBe(0);
  });

  it("runs a preset up to now, to the minute", () => {
    const range = { start: new CalendarDate(2026, 11, 3), end: new CalendarDate(2026, 11, 4) };

    expect(rangeFilters(range, "24h", "UTC", now)).toEqual({ from: "2026-11-03T12:30:00.000Z" });
  });

  it("runs picked days from the start of the first to the end of the last, in the reader's zone", () => {
    const range = { start: new CalendarDate(2026, 11, 1), end: new CalendarDate(2026, 11, 2) };

    expect(rangeFilters(range, null, "Europe/Paris", now)).toEqual({
      from: "2026-10-31T23:00:00.000Z",
      to: "2026-11-02T23:00:00.000Z",
    });
  });

  it("leaves time out when no range is picked", () => {
    expect(rangeFilters(null, null, "UTC", now)).toEqual({});
  });

  it("never offers days older than the server keeps", () => {
    expect(oldestDay(0, "UTC")).toBeUndefined();
    expect(oldestDay(365, "UTC")?.compare(today("UTC").subtract({ days: 365 }))).toBe(0);
  });
});
