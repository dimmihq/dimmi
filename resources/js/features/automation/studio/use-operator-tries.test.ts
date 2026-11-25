import { describe, expect, it } from "vitest";

import { withTry } from "./use-operator-tries";

describe("Operator's tries", () => {
  it("keeps one try per case: trying the same case again takes its place", () => {
    const parcel = { kind: "workflow" as const, moment: "started" as const, situation: "Ships weekly." };
    let tries = withTry([], { ...parcel, try_id: "try_1", message: "Where is my parcel?" });
    tries = withTry(tries, { try_id: "try_1", moment: "finished", result: { passed: false } });
    tries = withTry(tries, { ...parcel, try_id: "try_2", message: "Can I get a refund?" });
    tries = withTry(tries, { ...parcel, try_id: "try_3", message: " where is my  parcel? " });

    expect(tries.map((t) => t.id)).toEqual(["try_3", "try_2"]);
    expect(tries[0]?.done).toBe(false);
  });

  it("keeps the same message from different customers apart", () => {
    const ask = { kind: "workflow" as const, moment: "started" as const, message: "I need help with my label" };
    let tries = withTry([], { ...ask, try_id: "try_1", situation: "An enterprise customer." });
    tries = withTry(tries, { ...ask, try_id: "try_2", situation: "A customer on the free plan." });

    expect(tries).toHaveLength(2);
  });
});
