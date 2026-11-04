import { beforeEach, describe, expect, it } from "vitest";

import type { InboxAction } from "@/features/inbox/actions/registry";

import { recentFirst, remember } from "./recent";

function move(id: string): InboxAction {
  return { id, label: id, group: "Help", when: () => true, run: () => undefined };
}

describe("recent moves", () => {
  beforeEach(() => window.localStorage.clear());

  it("puts the moves run lately first, newest first", () => {
    remember("b");
    remember("c");

    expect(recentFirst([move("a"), move("b"), move("c")]).map((m) => m.id)).toEqual(["c", "b", "a"]);
  });

  it("keeps the usual order when storage holds nonsense", () => {
    window.localStorage.setItem("dimmi:command-menu:recent", "{oops");

    expect(recentFirst([move("a"), move("b")]).map((m) => m.id)).toEqual(["a", "b"]);
  });
});
