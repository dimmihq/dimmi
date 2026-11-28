import { existsSync, statSync } from "node:fs";
import { join, resolve } from "node:path";

import { describe, expect, it } from "vitest";

import { findRoute, type Route, routes } from "./routes";
import { headElements, OG_IMAGE_URL, ogImageUrl, SITE_URL, syncHead } from "./seo";

const PUBLIC_DIR = resolve(process.cwd(), "site/public");

function metaValue(elements: ReturnType<typeof headElements>, key: "property" | "name", target: string): string {
  for (const el of elements) {
    if (el.type === "meta" && key in el.props && el.props[key as keyof typeof el.props] === target) {
      return el.props.content ?? "";
    }
  }
  return "";
}

describe("ogImageUrl and SEO head metadata", () => {
  it("uses route.ogImage for og:image and twitter:image when set", () => {
    const pricing = findRoute("/pricing");
    expect(pricing?.ogImage).toBe("/og/pricing.png");
    expect(ogImageUrl(pricing)).toBe(`${SITE_URL}/og/pricing.png`);

    const elements = headElements(pricing);
    expect(metaValue(elements, "property", "og:image")).toBe(`${SITE_URL}/og/pricing.png`);
    expect(metaValue(elements, "name", "twitter:image")).toBe(`${SITE_URL}/og/pricing.png`);
  });

  it("falls back to /og.png when route or ogImage is omitted", () => {
    expect(ogImageUrl(undefined)).toBe(OG_IMAGE_URL);

    const routeWithoutImage: Route = {
      path: "/example",
      title: "Example | dimmi",
      description: "Example route without a custom OG card.",
      load: async () => () => null,
    };

    const elements = headElements(routeWithoutImage);
    expect(metaValue(elements, "property", "og:image")).toBe(OG_IMAGE_URL);
    expect(metaValue(elements, "name", "twitter:image")).toBe(OG_IMAGE_URL);
  });

  it("preserves absolute ogImage URLs as-is", () => {
    const routeWithAbsolute: Route = {
      path: "/external",
      title: "External | dimmi",
      description: "Route with an absolute OG image URL.",
      ogImage: "https://cdn.trydimmi.com/og/custom.png",
      load: async () => () => null,
    };

    expect(ogImageUrl(routeWithAbsolute)).toBe("https://cdn.trydimmi.com/og/custom.png");
  });

  it("keeps document.head in sync on client navigation via syncHead()", () => {
    document.head.innerHTML = "";
    const pricing = findRoute("/pricing");
    syncHead(pricing);

    expect(document.head.querySelector('meta[property="og:image"]')?.getAttribute("content")).toBe(
      `${SITE_URL}/og/pricing.png`,
    );
    expect(document.head.querySelector('meta[name="twitter:image"]')?.getAttribute("content")).toBe(
      `${SITE_URL}/og/pricing.png`,
    );

    syncHead(undefined);
    expect(document.head.querySelector('meta[property="og:image"]')?.getAttribute("content")).toBe(OG_IMAGE_URL);
    expect(document.head.querySelector('meta[name="twitter:image"]')?.getAttribute("content")).toBe(OG_IMAGE_URL);
  });

  it("ships an existing non-empty PNG in site/public for every route ogImage", () => {
    for (const route of routes) {
      expect(route.ogImage, `missing ogImage on ${route.path}`).toBeTruthy();
      const rel = (route.ogImage ?? "").replace(/^\/+/, "");
      const filePath = join(PUBLIC_DIR, rel);
      expect(existsSync(filePath), `missing file ${filePath} for ${route.path}`).toBe(true);
      expect(statSync(filePath).size).toBeGreaterThan(0);
    }
  });
});
