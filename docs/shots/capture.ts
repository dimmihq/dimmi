/*
 * Captures the docs' product screenshots from a running dimmi, the same
 * way the marketing site's capture does (site/shots/capture.ts): it signs
 * in to the Lumen demo workspace that `marketing:demo` seeds, opens each
 * screen listed in shots.ts and saves it at twice the pixel density in
 * light and dark mode as WebP into docs/public/shots. Pages show them with
 * <Screenshot name="…" alt="…" />.
 *
 *   ./tmp/dimmi-shots marketing:demo --owner-email maya@lumen.test
 *   SHOTS_URL=http://localhost:8085 node docs/shots/capture.ts [--only messenger]
 *
 * SHOTS_URL points at the app (default http://localhost:8080). The demo
 * owner signs in with the password the demo command prints.
 */
import { mkdirSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import { chromium, type Page } from "@playwright/test";

import { type Annotation, type Clip, type Shot, shots, VIEWPORT } from "./shots.ts";

const base = process.env.SHOTS_URL ?? "http://localhost:8080";
const email = process.env.SHOTS_EMAIL ?? "maya@lumen.test";
const password = process.env.SHOTS_PASSWORD ?? "dimmi-demo-team-2026";
const out = fileURLToPath(new URL("../public/shots/", import.meta.url));
const only = process.argv.includes("--only") ? process.argv[process.argv.indexOf("--only") + 1] : undefined;

// The crop of a shot: the whole window, a box given in CSS pixels, a
// custom locator/callback, a section found by its heading, or the open dialog.
async function clipFor(page: Page, shot: Shot): Promise<Clip | undefined> {
  const crop = shot.crop;
  if (!crop) {
    return undefined;
  }
  if (typeof crop === "function") {
    return crop(page);
  }
  if ("x" in crop) {
    return crop;
  }
  const target =
    "section" in crop
      ? page.getByRole("heading", { name: crop.section, exact: true }).first().locator("xpath=ancestor::section[1]")
      : page.getByRole("dialog").first();
  await target.scrollIntoViewIfNeeded();
  await page.waitForTimeout(300);
  const box = await target.boundingBox();
  if (!box) {
    throw new Error(`${shot.name}: nothing to crop to`);
  }
  const pad = crop.pad ?? 16;
  const x = Math.max(0, box.x - pad);
  const y = Math.max(0, box.y - pad);
  return {
    x,
    y,
    width: Math.min(VIEWPORT.width - x, box.width + pad * 2),
    height: Math.min(VIEWPORT.height - y, box.height + pad * 2),
  };
}

function anchorPoint(box: Clip, item: Annotation): { x: number; y: number } {
  const anchor = item.anchor ?? "top-left";
  const ax = anchor.includes("right") ? box.x + box.width : box.x;
  const ay = anchor.includes("bottom")
    ? box.y + box.height
    : anchor.includes("middle")
      ? box.y + box.height / 2
      : box.y;
  return { x: ax + (item.dx ?? 0), y: ay + (item.dy ?? 0) };
}

// Draws numbered callout pins anchored to real DOM elements so docs shots
// guide the reader directly to each part explained below the picture.
async function annotate(page: Page, annotations: Annotation[] | undefined, scheme: "light" | "dark") {
  if (!annotations || annotations.length === 0) {
    return;
  }
  const resolved: { label: string; x: number; y: number }[] = [];
  for (const item of annotations) {
    const box = await item.target(page).boundingBox();
    if (!box) {
      throw new Error(`annotation ${item.label}: target not visible`);
    }
    resolved.push({ label: item.label, ...anchorPoint(box, item) });
  }
  await page.evaluate(
    ({ pins, dark }) => {
      const host = document.createElement("div");
      host.setAttribute("data-docs-annotations", "true");
      Object.assign(host.style, { position: "fixed", inset: "0", pointerEvents: "none", zIndex: "2147483647" });
      for (const pin of pins) {
        const badge = document.createElement("div");
        badge.textContent = pin.label;
        Object.assign(badge.style, {
          position: "fixed",
          left: `${Math.round(pin.x)}px`,
          top: `${Math.round(pin.y)}px`,
          transform: "translate(-50%, -50%)",
          width: "24px",
          height: "24px",
          borderRadius: "9999px",
          background: "#059669",
          color: "#ffffff",
          border: dark ? "2px solid #0b0f0e" : "2px solid #ffffff",
          boxShadow: dark ? "0 4px 12px rgba(0,0,0,0.55)" : "0 4px 12px rgba(5,150,105,0.32)",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          fontFamily: "Inter, ui-sans-serif, system-ui, sans-serif",
          fontSize: "12px",
          fontWeight: "700",
          lineHeight: "1",
        });
        host.appendChild(badge);
      }
      document.body.appendChild(host);
    },
    { pins: resolved, dark: scheme === "dark" },
  );
}

async function clearAnnotations(page: Page) {
  await page.evaluate(() => {
    for (const el of document.querySelectorAll("[data-docs-annotations]")) {
      el.remove();
    }
  });
}

// Pictures show dimmi Cloud's address, not the local server's: snippets
// and URLs on screen name the app's own origin.
async function showProductionHost(page: Page) {
  await page.evaluate((origin) => {
    const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
    for (let node = walker.nextNode(); node; node = walker.nextNode()) {
      if (node.nodeValue?.includes(origin)) {
        node.nodeValue = node.nodeValue.replaceAll(origin, "https://trydimmi.com");
      }
    }
  }, new URL(base).origin);
}

// WebP straight from the browser, at 2x, as the site's capture does.
async function shoot(page: Page, file: string, clip: Clip) {
  const cdp = await page.context().newCDPSession(page);
  const { data } = await cdp.send("Page.captureScreenshot", {
    format: "webp",
    quality: 92,
    clip: { ...clip, scale: 2 },
  });
  writeFileSync(file, Buffer.from(data, "base64"));
}

async function signIn(page: Page, scheme: "light" | "dark") {
  await page.goto(`${base}/login`);
  await page.locator('input[type="email"]').fill(email);
  await page.locator('input[type="password"]').fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.waitForURL(/\/d\//);
  await page
    .getByRole("button", { name: scheme === "dark" ? "Use dark mode" : "Use light mode" })
    .first()
    .click()
    .catch(() => undefined);
}

mkdirSync(out, { recursive: true });
const browser = await chromium.launch();

for (const scheme of ["light", "dark"] as const) {
  const context = await browser.newContext({
    viewport: VIEWPORT,
    deviceScaleFactor: 2,
    colorScheme: scheme,
    reducedMotion: "reduce",
  });
  const page = await context.newPage();
  await signIn(page, scheme);

  for (const shot of shots.filter((s) => !only || s.name.startsWith(only))) {
    await page.goto(`${base}${shot.path}`);
    if (shot.waitFor) {
      await page.getByText(shot.waitFor, { exact: true }).first().waitFor();
    }
    await shot.prepare?.(page);
    await showProductionHost(page);
    // Let transitions and lazy content settle.
    await page.waitForTimeout(600);
    await annotate(page, shot.annotations, scheme);
    const clip = (await clipFor(page, shot)) ?? { x: 0, y: 0, ...VIEWPORT };
    await shoot(page, `${out}${shot.name}-${scheme}.webp`, clip);
    await clearAnnotations(page);
    process.stdout.write(`saved ${shot.name}-${scheme}.webp\n`);
    await shot.cleanup?.(page);
  }

  await context.close();
}

await browser.close();
