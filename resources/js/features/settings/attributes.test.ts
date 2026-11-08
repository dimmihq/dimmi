import { describe, expect, it } from "vitest";

import type { AttributeDefinition } from "@/lib/api/attributes";

import { ago, definitionLine } from "./attributes";

const now = new Date("2026-11-08T12:00:00Z");

const plan: AttributeDefinition = {
  id: "atd_1",
  of: "contact",
  key: "plan",
  label: "Plan",
  description: null,
  data_type: "list",
  options: ["pro", "team"],
  origin: "observed",
  status: "active",
  client_writable: false,
  last_value_at: "2026-11-08T09:00:00Z",
};

describe("definitionLine", () => {
  it("names the key, type, list values and last value", () => {
    expect(definitionLine(plan, now)).toBe("plan · List · pro, team · last value 3 hours ago · seen in data");
    expect(
      definitionLine({ ...plan, data_type: "date", options: [], last_value_at: null, status: "archived" }, now),
    ).toBe("plan · Date · no value yet · archived");
  });

  it("says roughly how long ago", () => {
    expect(ago("2026-11-08T11:59:30Z", now)).toBe("just now");
    expect(ago("2026-11-07T12:00:00Z", now)).toBe("yesterday");
    expect(ago("2026-11-01T12:00:00Z", now)).toBe("7 days ago");
  });
});
