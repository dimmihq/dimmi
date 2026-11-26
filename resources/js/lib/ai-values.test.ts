import type { ConversationAttribute } from "@/lib/api/conversation-attributes";

import { aiFilterSummary, aiValueLabel, aiValueOptions, defaultBreakdown, splitsReports } from "./ai-values";

function attribute(over: Partial<ConversationAttribute>): ConversationAttribute {
  return {
    id: "aia_1",
    object: "conversation_attribute",
    key: "reason",
    label: "Reason",
    description: "",
    data_type: "choice",
    options: ["billing", "bug"],
    filled_by: "both",
    required_to_close: false,
    team_ids: [],
    status: "active",
    created_at: "2026-11-01T00:00:00Z",
    updated_at: "2026-11-01T00:00:00Z",
    ...over,
  };
}

describe("aiValueLabel", () => {
  it("writes each type for a reader", () => {
    expect(aiValueLabel({ data_type: "boolean" }, true)).toBe("Yes");
    expect(aiValueLabel({ data_type: "boolean" }, "false")).toBe("No");
    expect(aiValueLabel({ data_type: "choice" }, "billing")).toBe("billing");
    expect(aiValueLabel({ data_type: "text" }, "ord_12")).toBe("ord_12");
    expect(aiValueLabel({ data_type: "number" }, 3)).toBe((3).toLocaleString());
    expect(aiValueLabel({ data_type: "date" }, "2026-11-24")).toContain("2026");
  });

  it("reads nothing found as null", () => {
    expect(aiValueLabel({ data_type: "text" }, null)).toBeNull();
    expect(aiValueLabel({ data_type: "text" }, undefined)).toBeNull();
    expect(aiValueLabel({ data_type: "text" }, "")).toBeNull();
  });
});

describe("aiValueOptions", () => {
  it("offers a choice's options, yes and no, or nothing", () => {
    expect(aiValueOptions(attribute({})).map((o) => o.id)).toEqual(["billing", "bug"]);
    expect(aiValueOptions(attribute({ data_type: "boolean", options: [] }))).toEqual([
      { id: "true", label: "Yes" },
      { id: "false", label: "No" },
    ]);
    expect(aiValueOptions(attribute({ data_type: "text", options: [] }))).toEqual([]);
  });
});

describe("defaultBreakdown", () => {
  it("picks the first active choice or yes-or-no attribute", () => {
    const list = [
      attribute({ key: "order", data_type: "text", options: [] }),
      attribute({ key: "old", status: "archived" }),
      attribute({ key: "urgent", data_type: "boolean", options: [] }),
      attribute({ key: "reason" }),
    ];
    expect(defaultBreakdown(list)?.key).toBe("urgent");
    expect(defaultBreakdown([list[0]])).toBeUndefined();
    expect(splitsReports(list[0])).toBe(false);
  });
});

describe("aiFilterSummary", () => {
  it("names the attribute and the value picked", () => {
    const list = [attribute({}), attribute({ key: "urgent", label: "Urgent", data_type: "boolean", options: [] })];
    expect(aiFilterSummary(list, "reason", "billing")).toEqual({ attribute: "Reason", value: "billing" });
    expect(aiFilterSummary(list, "urgent", "true")).toEqual({ attribute: "Urgent", value: "Yes" });
    expect(aiFilterSummary(list, "reason", undefined)).toEqual({ attribute: "Reason", value: "found" });
    expect(aiFilterSummary([], "gone", "x")).toEqual({ attribute: "gone", value: "x" });
  });
});
