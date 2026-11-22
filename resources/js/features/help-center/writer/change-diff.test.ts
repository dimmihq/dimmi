import { describe, expect, it } from "vitest";

import { changedCount, diffBlocks } from "./change-diff";

describe("diffBlocks", () => {
  it("keeps blocks that did not change", () => {
    expect(diffBlocks(["a", "b"], ["a", "b"])).toEqual([
      { kind: "same", from: 0, to: 0 },
      { kind: "same", from: 1, to: 1 },
    ]);
  });

  it("reads a reworded block as removed then added in its place", () => {
    expect(diffBlocks(["a", "b", "c"], ["a", "B", "c"])).toEqual([
      { kind: "same", from: 0, to: 0 },
      { kind: "del", from: 1 },
      { kind: "add", to: 1 },
      { kind: "same", from: 2, to: 2 },
    ]);
  });

  it("adds blocks at the end and removes them from the start", () => {
    const changes = diffBlocks(["x", "a"], ["a", "y", "z"]);
    expect(changes).toEqual([
      { kind: "del", from: 0 },
      { kind: "same", from: 1, to: 0 },
      { kind: "add", to: 1 },
      { kind: "add", to: 2 },
    ]);
    expect(changedCount(changes)).toEqual({ added: 2, removed: 1 });
  });

  it("reads every block as added for a new article", () => {
    expect(diffBlocks([], ["a", "b"])).toEqual([
      { kind: "add", to: 0 },
      { kind: "add", to: 1 },
    ]);
  });
});
