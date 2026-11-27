/*
 * Captures the help center theme shots: a real help center wearing the
 * Retro 95 theme from site/shots/themes/retro, published with the dimmi
 * CLI. Its home page is saved 3840 pixels wide (4K) as WebP into
 * site/public/shots/theme-retro-home.webp.
 *
 *   dimmi theme publish site/shots/themes/retro
 *   node site/shots/capture-themes.ts
 *
 * THEME_SHOTS_URL is the help center's address, by default the Builder
 * Check help center of a local dimmi at http://localhost:8080.
 */
import { execFileSync } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

import { chromium } from "@playwright/test";

const base = process.env.THEME_SHOTS_URL ?? "http://localhost:8080/hc/builder-check";
const out = fileURLToPath(new URL("../public/shots/", import.meta.url));

/* The window's CSS size; the file is 2.4 times it, 3840 pixels wide. */
const width = 1600;
const height = 1000;

const pages: {
  name: string;
  path: string;
  scale: number;
  clip?: { x: number; y: number; width: number; height: number };
}[] = [{ name: "home", path: "", scale: 2.4 }];

const browser = await chromium.launch();
const work = mkdtempSync(join(tmpdir(), "dimmi-theme-shots-"));

try {
  for (const shot of pages) {
    const context = await browser.newContext({ viewport: { width, height }, deviceScaleFactor: shot.scale });
    const page = await context.newPage();
    await page.goto(base + shot.path, { waitUntil: "load" });
    // The messenger may keep a connection open; idle is best effort.
    await page.waitForLoadState("networkidle", { timeout: 5000 }).catch(() => undefined);
    // Hold the scrolling banner at its start and the blinking text on, so
    // every capture matches.
    await page.addStyleTag({
      content: "*{animation:none!important}.marquee span{padding-left:24px!important}",
    });
    await page.evaluate(() => document.fonts.ready);
    const png = join(work, `${shot.name}.png`);
    await page.screenshot({ path: png, clip: shot.clip });
    await context.close();
    const webp = `${out}theme-retro-${shot.name}.webp`;
    execFileSync("cwebp", ["-quiet", "-q", "90", "-m", "6", png, "-o", webp]);
    process.stdout.write(`saved ${webp}\n`);
  }
} finally {
  await browser.close();
  rmSync(work, { recursive: true, force: true });
}
