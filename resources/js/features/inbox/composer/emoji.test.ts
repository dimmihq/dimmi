import { describe, expect, it } from "vitest";

import { type Emoji, findEmoji, loadEmoji } from "./emoji";

const list: Emoji[] = [
  { unicode: "😀", label: "grinning face", tags: ["face", "grin"] },
  { unicode: "👍", label: "thumbs up", tags: ["+1", "good", "yes"] },
  { unicode: "🎉", label: "party popper", tags: ["celebration", "tada"] },
  { unicode: "🙂", label: "slightly smiling face", tags: ["face", "smile"] },
];

describe("findEmoji", () => {
  it("puts names that start with the words before names and tags that contain them", () => {
    expect(findEmoji(list, "fa", 5).map((e) => e.unicode)).toEqual(["😀", "🙂"]);
    expect(findEmoji(list, "smil", 5).map((e) => e.unicode)).toEqual(["🙂"]);
  });

  it("finds by tag, such as +1", () => {
    expect(findEmoji(list, "+1", 5).map((e) => e.unicode)).toEqual(["👍"]);
    expect(findEmoji(list, "Tada", 5).map((e) => e.unicode)).toEqual(["🎉"]);
  });

  it("gives the first ones for no words and stops at the limit", () => {
    expect(findEmoji(list, " ", 2)).toHaveLength(2);
    expect(findEmoji(list, "zzz", 5)).toEqual([]);
  });
});

describe("loadEmoji", () => {
  it("loads the emoji once, without skin tone components", async () => {
    const first = await loadEmoji();
    expect(await loadEmoji()).toBe(first);
    expect(first.find((e) => e.label === "thumbs up")?.unicode).toBe("👍");
    expect(first.some((e) => e.label.startsWith("light skin tone"))).toBe(false);
  });
});
