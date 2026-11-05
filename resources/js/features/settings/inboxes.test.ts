import { assignmentModes, keyOf, poolLine, queueOrders, reopenLabel } from "./inboxes";

describe("reopenLabel", () => {
  it("reads never, one day and many days", () => {
    expect(reopenLabel(0)).toBe("Never reopen");
    expect(reopenLabel(1)).toBe("1 day");
    expect(reopenLabel(30)).toBe("30 days");
  });
});

describe("poolLine", () => {
  it("counts what a teammate holds and says why they are passed over", () => {
    expect(poolLine(1, null)).toBe("1 open");
    expect(poolLine(3, "away")).toBe("3 open · Away");
    expect(poolLine(2, "at_channel_capacity")).toBe("2 open · At channel capacity");
  });
});

describe("keyOf", () => {
  it("finds the key of a label and nothing for an unknown one", () => {
    expect(keyOf(assignmentModes, "Round robin")).toBe("round_robin");
    expect(keyOf(queueOrders, "Highest priority first")).toBe("priority_then_oldest");
    expect(keyOf(assignmentModes, "Lottery")).toBeUndefined();
  });
});
