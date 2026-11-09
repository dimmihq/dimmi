import { describe, expect, it } from "vitest";

import type { Article, Collection } from "@/lib/api/help-centers";

import { allKey, articlesIn, canDrop, childrenOf, depthOf, planMove, treeOptions, unfiledKey } from "./collection-tree";

function collection(id: string, position: number, parent: string | null = null): Collection {
  return {
    id,
    object: "collection",
    help_center_id: "hc",
    parent_id: parent,
    name: id,
    slug: id,
    description: null,
    icon: null,
    position,
  };
}

const tree = [
  collection("basics", 1024),
  collection("billing", 2048),
  collection("invoices", 1024, "billing"),
  collection("refunds", 2048, "billing"),
  collection("api", 3072),
];

describe("collection tree", () => {
  it("lists children in order", () => {
    expect(childrenOf(tree, null).map((c) => c.id)).toEqual(["basics", "billing", "api"]);
    expect(childrenOf(tree, "billing").map((c) => c.id)).toEqual(["invoices", "refunds"]);
  });

  it("keeps the tree three levels deep", () => {
    expect(canDrop(tree, "api", { key: "billing", position: "on" })).toBe(true);
    expect(canDrop(tree, "api", { key: "invoices", position: "on" })).toBe(true);
    expect(canDrop(tree, "billing", { key: "basics", position: "on" })).toBe(true);
    expect(canDrop(tree, "billing", { key: "invoices", position: "before" })).toBe(false);
    expect(canDrop(tree, "billing", { key: "invoices", position: "on" })).toBe(false);
    expect(canDrop(tree, "invoices", { key: "basics", position: "after" })).toBe(true);
    expect(canDrop(tree, "basics", { key: "basics", position: "on" })).toBe(false);

    const deep = [...tree, collection("cards", 1024, "invoices")];
    expect(canDrop(deep, "api", { key: "cards", position: "on" })).toBe(false);
    expect(canDrop(deep, "billing", { key: "basics", position: "on" })).toBe(false);
    expect(canDrop(deep, "api", { key: "cards", position: "after" })).toBe(true);
    expect(depthOf(deep, "cards")).toBe(3);
  });

  it("nests a collection at the end of its new parent", () => {
    expect(planMove(tree, "api", { key: "billing", position: "on" })).toEqual([
      { id: "api", parent_id: "billing", position: 3072 },
    ]);
  });

  it("moves a sub-collection to the top between two others", () => {
    expect(planMove(tree, "refunds", { key: "basics", position: "after" })).toEqual([
      { id: "refunds", parent_id: "", position: 2048 },
      { id: "billing", position: 3072 },
      { id: "api", position: 4096 },
    ]);
  });

  it("reorders siblings without touching their parent", () => {
    expect(planMove(tree, "api", { key: "basics", position: "before" })).toEqual([
      { id: "api", position: 1024 },
      { id: "basics", position: 2048 },
      { id: "billing", position: 3072 },
    ]);
  });

  it("lists every collection in tree order with its depth and path", () => {
    const deep = [...tree, collection("cards", 1024, "invoices")];
    expect(treeOptions(deep).map((o) => `${o.depth}:${o.path}`)).toEqual([
      "0:basics",
      "0:billing",
      "1:billing / invoices",
      "2:billing / invoices / cards",
      "1:billing / refunds",
      "0:api",
    ]);
  });

  it("lists the articles of a pick, with those of the collections inside", () => {
    const articles = [
      { id: "a", collection_id: "billing" },
      { id: "b", collection_id: "refunds" },
      { id: "c", collection_id: "api" },
      { id: "e", collection_id: "cards" },
      { id: "d", collection_id: null },
    ] as Article[];
    const deep = [...tree, collection("cards", 1024, "invoices")];
    const ids = (pick: string) => articlesIn(pick, articles, deep).map((a) => a.id);

    expect(ids(allKey)).toEqual(["a", "b", "c", "e", "d"]);
    expect(ids(unfiledKey)).toEqual(["d"]);
    expect(ids("billing")).toEqual(["a", "b", "e"]);
    expect(ids("refunds")).toEqual(["b"]);
  });
});
