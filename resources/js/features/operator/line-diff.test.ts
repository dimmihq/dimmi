import { describe, expect, it } from "vitest";

import { changeCount, diffLines, folded } from "./line-diff";

describe("diffLines", () => {
  it("finds lines added, removed and kept", () => {
    const d = diffLines("# Refunds\nWithin 14 days.\nContact us.", "# Refunds\nWithin 30 days.\nContact us.");
    expect(d).toEqual([
      { kind: "same", text: "# Refunds" },
      { kind: "del", text: "Within 14 days." },
      { kind: "add", text: "Within 30 days." },
      { kind: "same", text: "Contact us." },
    ]);
    expect(changeCount(d)).toEqual({ added: 1, removed: 1 });
  });

  it("treats a new text as all added", () => {
    expect(diffLines("", "one\ntwo").filter((l) => l.kind === "add")).toHaveLength(2);
  });
});

describe("folded", () => {
  it("folds long runs of unchanged lines", () => {
    const before = Array.from({ length: 20 }, (_, i) => `line ${i}`).join("\n");
    const after = before.replace("line 10", "line ten");
    const f = folded(diffLines(before, after));
    expect(f[0]).toEqual({ kind: "fold", text: "8 unchanged lines" });
    expect(f.filter((l) => l.kind === "same")).toHaveLength(4);
    expect(f.at(-1)).toEqual({ kind: "fold", text: "7 unchanged lines" });
  });
});
