import { describe, expect, it } from "vitest";

import { joinLayout, layoutClasses, roundWidth, splitLayout } from "./image-layout";

describe("image layout", () => {
  it("reads the width and alignment from the address", () => {
    expect(splitLayout("/plans.png#width=60&align=center")).toEqual({
      src: "/plans.png",
      width: 60,
      align: "center",
    });
    expect(splitLayout("/plans.png")).toEqual({ src: "/plans.png", width: null, align: null });
  });

  it("leaves any other fragment in the address", () => {
    expect(splitLayout("/a.png#top").src).toBe("/a.png#top");
    expect(splitLayout("/a.png#align=left").src).toBe("/a.png#align=left");
    expect(splitLayout("/a.png#width=wide").src).toBe("/a.png#width=wide");
  });

  it("writes only what differs from the default", () => {
    expect(joinLayout("/a.png", { width: 60, align: "end" })).toBe("/a.png#width=60&align=end");
    expect(joinLayout("/a.png", { width: 100, align: null })).toBe("/a.png#width=100");
    expect(joinLayout("/a.png", { width: null, align: null })).toBe("/a.png");
    expect(joinLayout("/a.png", { width: null, align: "center" })).toBe("/a.png#align=center");
  });

  it("rounds and bounds widths the way the help center does", () => {
    expect(roundWidth(62)).toBe(60);
    expect(roundWidth(3)).toBe(20);
    expect(roundWidth(400)).toBe(100);
    expect(layoutClasses({ width: 45, align: "center" })).toBe("image-width-45 image-align-center");
  });
});
