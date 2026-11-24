import { describe, expect, it } from "vitest";

import type { ReportMetric } from "@/lib/api/reports";

import { changeOf, initialsOf, told, waited } from "./figures";

function metric(unit: ReportMetric["unit"], value: number | null, previous: number | null): ReportMetric {
  return { key: "k", label: "K", unit, value, previous, p90: null, mean: null, sample: 0, drill: null };
}

describe("home figures", () => {
  it("takes two letters of a name for its avatar", () => {
    expect(initialsOf("Priya Raman")).toBe("PR");
    expect(initialsOf("lena")).toBe("L");
    expect(initialsOf("Ana  Maria de Souza")).toBe("AM");
  });

  it("says how long a customer has waited", () => {
    const now = new Date("2026-03-10T12:00:00Z").getTime();

    expect(waited("2026-03-08T08:00:00Z", now)).toBe("2d 4h");
    expect(waited("2026-03-10T13:00:00Z", now)).toBe("0s");
  });

  it("says how a count moved by its share", () => {
    expect(changeOf(metric("count", 108, 100))).toBe("up 8%");
    expect(changeOf(metric("count", 75, 100))).toBe("down 25%");
    expect(changeOf(metric("count", 12, 0))).toBeUndefined();
  });

  it("says where a rate came from", () => {
    expect(changeOf(metric("percent", 61, 57))).toBe("up from 57%");
    expect(changeOf(metric("percent", 50, 57))).toBe("down from 57%");
  });

  it("says whether a time got faster or slower", () => {
    expect(changeOf(metric("seconds", 600, 780))).toBe("3m faster");
    expect(changeOf(metric("seconds", 780, 600))).toBe("3m slower");
  });

  it("shows a routine's report only to the people it tells", () => {
    const delivery = { in_app_member_ids: ["mem_a"], email_member_ids: ["mem_b"], slack_destination_ids: [] };

    expect(told({ delivery }, "mem_a")).toBe(true);
    expect(told({ delivery }, "mem_b")).toBe(true);
    expect(told({ delivery }, "mem_c")).toBe(false);
    expect(told({ delivery }, "")).toBe(false);
  });

  it("says nothing with nothing to compare, and no change when it held", () => {
    expect(changeOf(metric("count", 5, null))).toBeUndefined();
    expect(changeOf(metric("count", null, 5))).toBeUndefined();
    expect(changeOf(undefined)).toBeUndefined();
    expect(changeOf(metric("percent", 92, 92))).toBe("no change");
  });
});
