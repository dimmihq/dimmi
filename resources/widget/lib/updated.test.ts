import { describe, expect, it } from "vitest";
import type { Content } from "../content";
import { defaults } from "../options";
import { Pages } from "../views/pages";
import { reading, updatedAgo } from "./map";

const now = Date.parse("2026-11-26T12:00:00Z");
const back = (days: number) => new Date(now - days * 86_400_000).toISOString();

describe("when an article last changed", () => {
  it("reads like a help center: today, yesterday, then whole days, weeks, months and years", () => {
    reading.locale = "en";
    expect(updatedAgo(back(0), now)).toBe("today");
    expect(updatedAgo(back(1), now)).toBe("yesterday");
    expect(updatedAgo(back(3), now)).toBe("3 days ago");
    expect(updatedAgo(back(9), now)).toBe("last week");
    expect(updatedAgo(back(16), now)).toBe("2 weeks ago");
    expect(updatedAgo(back(70), now)).toBe("2 months ago");
    expect(updatedAgo(back(800), now)).toBe("2 years ago");
  });

  it("speaks the visitor's language", () => {
    reading.locale = "de";
    expect(updatedAgo(back(16), now)).toBe("vor 2 Wochen");
    reading.locale = "en";
  });

  it("is nothing for a date it cannot read", () => {
    expect(updatedAgo("soon", now)).toBe("");
  });
});

describe("an article's header", () => {
  const page = (author?: { name: string; avatar_url: string | null }) => {
    const content = {
      collections: [],
      articles: [{ slug: "invite", title: "Invite your team", summary: "", updated: back(16), author, body: [] }],
      changelog: [],
    } as unknown as Content;
    const root = document.createElement("div");
    root.innerHTML = String(Pages({ options: defaults, content }));
    return root.querySelector(".page-meta");
  };

  it("names who wrote it, with their photo, and when it changed", () => {
    const meta = page({ name: "Maya Kerr", avatar_url: "https://files.example.com/maya.png" });
    expect(meta?.querySelector("img")?.getAttribute("src")).toBe("https://files.example.com/maya.png");
    expect(meta?.textContent).toContain("Written by Maya Kerr");
    expect(meta?.textContent).toMatch(/Updated \S/);
  });

  it("draws initials for an author without a photo", () => {
    expect(page({ name: "Maya Kerr", avatar_url: null })?.querySelector(".face")?.textContent).toBe("MK");
  });

  it("shows only when it changed when no teammate wrote it", () => {
    const meta = page();
    expect(meta?.querySelector(".face")).toBeNull();
    expect(meta?.textContent).not.toContain("Written by");
    expect(meta?.textContent).toMatch(/^Updated /);
  });
});
