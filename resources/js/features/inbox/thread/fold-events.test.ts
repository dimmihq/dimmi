import { describe, expect, it } from "vitest";

import type { ThreadEntry } from "../conversations";
import { foldEvents, foldedText } from "./fold-events";

const said = (id: string): ThreadEntry => ({ kind: "customer", id, author: "Ann", at: "", text: "Hi" });
const event = (id: string): ThreadEntry => ({ kind: "event", id, at: "", text: "Ada closed the conversation" });

describe("foldEvents", () => {
  it("keeps every entry while events show", () => {
    const entries = [said("a"), event("b")];
    expect(foldEvents(entries, false)).toBe(entries);
  });

  it("folds each run of events into one line with its count", () => {
    const folded = foldEvents([event("a"), event("b"), said("c"), event("d"), event("e"), event("f")], true);
    expect(folded).toEqual([
      { kind: "folded", id: "folded-a", count: 2 },
      said("c"),
      { kind: "folded", id: "folded-d", count: 3 },
    ]);
  });

  it("says one event or how many", () => {
    expect(foldedText(1)).toBe("1 event");
    expect(foldedText(3)).toBe("3 events");
  });
});
