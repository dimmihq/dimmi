import { reportParams } from "@/lib/api/reports";

import { drillFilters, reportFilters, validateReportSearch } from "./search";

describe("the AI attribute filter in the address", () => {
  it("keeps a value only beside its attribute and drops malformed keys", () => {
    expect(validateReportSearch({ ai_attribute: "reason", ai_attribute_value: "billing" })).toEqual({
      ai_attribute: "reason",
      ai_attribute_value: "billing",
    });
    expect(validateReportSearch({ ai_attribute_value: "billing" })).toEqual({});
    expect(validateReportSearch({ ai_attribute: "Not a key", ai_breakdown: "DROP" })).toEqual({});
    expect(validateReportSearch({ ai_breakdown: "urgent" })).toEqual({ ai_breakdown: "urgent" });
  });

  it("asks the report for the filter and the split, and the open list for the filter only", () => {
    const search = { from: "2026-11-01", to: "2026-11-07", ai_attribute: "reason", ai_breakdown: "urgent" };
    const params = reportParams(reportFilters(search));
    expect(params.get("filter[ai_attribute]")).toBe("reason");
    expect(params.get("ai_breakdown")).toBe("urgent");

    const drill = drillFilters({ ...search, drill_by: "ai_attribute=reason;ai_attribute_value=bug" });
    expect(drill.ai_breakdown).toBeUndefined();
    expect(drill.ai_attribute_value).toBe("bug");
  });
});
