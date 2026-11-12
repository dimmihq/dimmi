import { describe, expect, it } from "vitest";

import { emptySelection, shown, toggled } from "./selection";

const rows = ["a", "b", "c", "d", "e"];

describe("toggled", () => {
  it("toggles one row and remembers it as the anchor", () => {
    const one = toggled(emptySelection, rows, "b", false);
    expect([...one.ids]).toEqual(["b"]);
    expect(one.anchor).toBe("b");
    expect([...toggled(one, rows, "b", false).ids]).toEqual([]);
  });

  it("selects the range from the anchor on a shift-click, either way", () => {
    const anchor = toggled(emptySelection, rows, "d", false);
    expect([...toggled(anchor, rows, "b", true).ids].sort()).toEqual(["b", "c", "d"]);
  });

  it("clears the range when the clicked row was selected", () => {
    const all = { ids: new Set(rows), anchor: "a" };
    expect([...toggled(all, rows, "c", true).ids].sort()).toEqual(["d", "e"]);
  });

  it("toggles one row when the anchor left the list", () => {
    const gone = { ids: new Set<string>(), anchor: "z" };
    expect([...toggled(gone, rows, "c", true).ids]).toEqual(["c"]);
  });
});

describe("shown", () => {
  it("drops rows that left the list", () => {
    const picked = { ids: new Set(["a", "z"]), anchor: "z" };
    const kept = shown(picked, rows);
    expect([...kept.ids]).toEqual(["a"]);
    expect(kept.anchor).toBeNull();
  });
});
