import { describe, expect, it } from "vitest";

import { withTry } from "./use-operator-tries";

describe("Operator's tries", () => {
  it("keeps one try per case: trying the same case again takes its place", () => {
    let tries = withTry([], { try_id: "try_1", kind: "workflow", moment: "started", message: "Where is my parcel?" });
    tries = withTry(tries, { try_id: "try_1", moment: "finished", result: { passed: false } });
    tries = withTry(tries, { try_id: "try_2", kind: "workflow", moment: "started", message: "Can I get a refund?" });
    tries = withTry(tries, { try_id: "try_3", kind: "workflow", moment: "started", message: " where is my  parcel? " });

    expect(tries.map((t) => t.id)).toEqual(["try_3", "try_2"]);
    expect(tries[0]?.done).toBe(false);
  });
});
