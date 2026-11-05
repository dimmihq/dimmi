import { describe, expect, it } from "vitest";

import { contrast, tagColors, testing } from "./tag-colors";

const rgb = (value: string) => {
  const [r, g, b] = value.match(/\d+/g)?.slice(0, 3).map(Number) ?? [];
  return [r, g, b] as [number, number, number];
};

const picks = [
  "#2563eb",
  "#16a34a",
  "#dc2626",
  "#ca8a04",
  "#ffff00",
  "#00ff00",
  "#ffffff",
  "#000000",
  "#0891b2",
  "#777777",
];

describe("tagColors", () => {
  it("keeps the text readable on its tint in light mode, whatever the color", () => {
    for (const pick of picks) {
      const colors = tagColors(pick);
      expect(colors).not.toBeNull();
      const { background, text } = colors?.light ?? { background: "", text: "" };
      expect(contrast(rgb(text), rgb(background)), pick).toBeGreaterThanOrEqual(4.5);
    }
  });

  it("keeps the text readable on its tint over the dark surface", () => {
    for (const pick of picks) {
      const colors = tagColors(pick);
      const color = testing.parse(pick) ?? [0, 0, 0];
      const tint = testing.mix(color, [0, 0, 0], 0.8);
      const shown = testing.mix(testing.darkSurface, tint, 0.6);
      expect(contrast(rgb(colors?.dark.text ?? ""), shown), pick).toBeGreaterThanOrEqual(4.5);
    }
  });

  it("gives no tint for a missing or unreadable color", () => {
    expect(tagColors(null)).toBeNull();
    expect(tagColors("blue")).toBeNull();
  });
});
