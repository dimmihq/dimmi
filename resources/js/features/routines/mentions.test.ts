import { describe, expect, it } from "vitest";

import { findMentions, type Mentionables, nameOf, nothingToMention, token } from "./mentions";

const known: Mentionables = {
  ...nothingToMention,
  handoff: [
    { id: "team_1", name: "Billing" },
    { id: "team_2", name: "Support" },
  ],
  teammate: [{ id: "mem_1", name: "Bill Okafor" }],
  tag: [{ id: "tag_1", name: "refund" }],
};

describe("findMentions", () => {
  it("lists every kind that has something, each under its heading once", () => {
    const found = findMentions(known, "");

    expect(found.map((m) => [m.group, m.label, m.first])).toEqual([
      ["Teams", "Billing", true],
      ["Teams", "Support", false],
      ["Teammates", "Bill Okafor", true],
      ["Tags", "refund", true],
    ]);
  });

  it("narrows by the words typed, whatever their case", () => {
    expect(findMentions(known, "BILL").map((m) => m.label)).toEqual(["Billing", "Bill Okafor"]);
    expect(findMentions(known, "nothing like it")).toEqual([]);
  });

  it("shows at most five of a kind", () => {
    const many = { ...nothingToMention, tag: Array.from({ length: 9 }, (_, i) => ({ id: `tag_${i}`, name: `t${i}` })) };

    expect(findMentions(many, "")).toHaveLength(5);
  });
});

describe("nameOf", () => {
  it("names what a mention points at now", () => {
    expect(nameOf(known, "handoff", "team_1")).toBe("Billing");
  });

  it("says nothing for a target that is gone or a kind it does not know", () => {
    expect(nameOf(known, "handoff", "team_9")).toBe("");
    expect(nameOf(known, "weather", "x")).toBe("");
  });
});

describe("token", () => {
  it("writes a mention the way the server reads it", () => {
    expect(token("teammate", "mem_1")).toBe("[[teammate:mem_1]]");
  });
});
