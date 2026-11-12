import { describe, expect, it } from "vitest";

import type { ReportMetric } from "@/lib/api/reports";

import { deltaOf, formatDuration, formatValue } from "./format";
import { drillFilters, validateReportSearch } from "./search";

function metric(changes: Partial<ReportMetric>): ReportMetric {
  return {
    key: "replies",
    label: "Replies",
    unit: "count",
    value: 10,
    previous: 8,
    p90: null,
    mean: null,
    sample: 10,
    drill: null,
    ...changes,
  };
}

describe("report formatting", () => {
  it("writes durations in the largest units that matter", () => {
    expect(formatDuration(45)).toBe("45s");
    expect(formatDuration(12 * 60)).toBe("12m");
    expect(formatDuration(3 * 3600 + 5 * 60)).toBe("3h 5m");
    expect(formatDuration(2 * 86_400 + 4 * 3600)).toBe("2d 4h");
  });

  it("writes costs, rates and scores", () => {
    expect(formatValue("micros", 120_000)).toBe("$0.12");
    expect(formatValue("percent", 87)).toBe("87%");
    expect(formatValue("score", 4)).toBe("4.0");
    expect(formatValue("count", null)).toBe("—");
  });

  it("colours a change by whether it is good", () => {
    expect(deltaOf(metric({}))).toEqual({ text: "+25%", tone: "lime" });
    expect(deltaOf(metric({ unit: "seconds", value: 60, previous: 30 }))).toEqual({ text: "+100%", tone: "rose" });
    expect(deltaOf(metric({ key: "reopen_rate", unit: "percent", value: 5, previous: 8 }))).toEqual({
      text: "-3 pts",
      tone: "lime",
    });
    expect(deltaOf(metric({ previous: null }))).toBeNull();
  });
});

describe("report search", () => {
  it("keeps only what a report reads", () => {
    expect(validateReportSearch({ range: "7d", from: "2026-01-01", compare: true, inbox: "inb_1", junk: 1 })).toEqual({
      range: "7d",
      compare: true,
      inbox: "inb_1",
    });
    expect(validateReportSearch({ from: "nope", drill_from: "2026-01-01" })).toEqual({});
  });

  it("narrows the open list to the day and row picked", () => {
    const filters = drillFilters({
      from: "2026-01-01",
      to: "2026-01-31",
      compare: true,
      member: "mem_1",
      drill: "replied",
      drill_from: "2026-01-05",
      drill_to: "2026-01-05",
      drill_by: "member=mem_2;channel=email",
    });
    expect(filters).toEqual({ from: "2026-01-05", to: "2026-01-05", member: "mem_2", channel: "email" });
  });
});
