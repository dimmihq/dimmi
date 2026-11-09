import { describe, expect, it } from "vitest";

import { listSummary } from "./tagging";

describe("listSummary", () => {
  it("says how many matched, were tagged and were not found", () => {
    expect(
      listSummary({
        tag_id: "tag_1",
        tag_name: "Event",
        matched: 2,
        tagged: 1,
        unmatched: 4,
        unmatched_sample: ["a@x", "b@x", "c@x", "d@x"],
      }),
    ).toBe("2 people matched, 1 person newly tagged Event, 4 not found (a@x, b@x, c@x, …).");
    expect(
      listSummary({ tag_id: "t", tag_name: "Old", matched: 1, tagged: 1, unmatched: 0, unmatched_sample: [] }),
    ).toBe("1 person matched, 1 person newly tagged Old.");
  });
});
