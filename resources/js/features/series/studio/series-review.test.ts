import { describe, expect, it } from "vitest";

import type { SeriesSpec } from "@/lib/api/series";

import { reviewOf } from "./series-review";

const spec: SeriesSpec = {
  entry: { reentry: { kind: "once" } },
  exit: {},
  nodes: [
    { id: "entry", kind: "entry" },
    { id: "hello", kind: "email", settings: { subject: "Hi", body: "Hello" } },
    { id: "wait", kind: "wait", settings: { mode: "duration", amount: 2, unit: "days" } },
  ],
  edges: [
    { from: "entry", path: "next", to: "hello" },
    { from: "hello", path: "next", to: "wait" },
  ],
};

describe("series review", () => {
  it("marks added, changed and removed steps and the paths joined and cut", () => {
    const proposed: SeriesSpec = {
      ...spec,
      goal: { event: "paid", days: 14 },
      nodes: [
        spec.nodes[0],
        { id: "hello", kind: "email", settings: { subject: "Welcome", body: "Hello" } },
        { id: "tag-1", kind: "tag", settings: { tag_ids: ["tag_1"] } },
      ],
      edges: [
        { from: "entry", path: "next", to: "hello" },
        { from: "hello", path: "next", to: "tag-1" },
      ],
    };

    const review = reviewOf(spec, proposed);

    expect([...review.added]).toEqual(["tag-1"]);
    expect([...review.changed]).toEqual(["hello"]);
    expect([...review.removed]).toEqual(["wait"]);
    expect([...review.joined]).toEqual(["hello:next"]);
    expect([...review.cut]).toEqual(["hello:next"]);
    expect(review.head).toBe(true);
  });
});
