import { describe, expect, it } from "vitest";

import type { TreeNode } from "@/lib/api/knowledge-sources";

import { inheritedOf, nextSelection } from "./tree-selection";

const leaf = (id: string): TreeNode => ({ id, title: id, kind: "file", children: [] });
const nodes: TreeNode[] = [
  {
    id: "docs",
    title: "docs",
    kind: "folder",
    children: [leaf("a"), { id: "guides", title: "g", kind: "folder", children: [leaf("b")] }],
  },
  leaf("readme"),
];

describe("tree selection", () => {
  it("ticks everything under a ticked item without storing it", () => {
    expect([...inheritedOf(nodes, ["docs"])].sort()).toEqual(["a", "b", "guides"]);
    expect(inheritedOf(nodes, []).size).toBe(0);
  });

  it("stores a new tick and drops ticks the new one covers", () => {
    expect(nextSelection(nodes, ["a"], new Set(["a", "readme"]))).toEqual(["a", "readme"]);
    expect(nextSelection(nodes, ["a", "b"], new Set(["a", "b", "docs"]))).toEqual(["docs"]);
  });

  it("unticks what is no longer shown", () => {
    expect(nextSelection(nodes, ["docs", "readme"], new Set(["readme", "a", "b", "guides"]))).toEqual(["readme"]);
  });
});
