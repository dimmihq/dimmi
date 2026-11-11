import { describe, expect, it } from "vitest";

import { moved } from "./status-order";

describe("moved", () => {
  const order = ["a", "b", "c", "d"];

  it("lands a row after the target", () => {
    expect(moved(order, new Set(["a"]), "c", "after")).toEqual(["b", "c", "a", "d"]);
  });

  it("lands a row before the target", () => {
    expect(moved(order, new Set(["d"]), "a", "before")).toEqual(["d", "a", "b", "c"]);
  });

  it("keeps the order when the target is unknown", () => {
    expect(moved(order, new Set(["a"]), "z", "before")).toEqual(order);
  });
});
