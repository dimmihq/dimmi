/*
 * Captures 4K-quality WebP stills and high-resolution looping MP4 videos
 * of Agent Studio in light and dark mode from a running Stowline demo
 * instance (http://localhost:8091 by default).
 *
 *   node site/shots/capture-agent-studio.ts
 */
import { execFileSync } from "node:child_process";
import { mkdirSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

import { chromium, type Page } from "@playwright/test";

const base = process.env.SHOTS_URL ?? "http://localhost:8091";
const email = process.env.SHOTS_EMAIL ?? "maya@stowline.test";
const password = process.env.SHOTS_PASSWORD ?? "stowline-demo-2026";
const out = fileURLToPath(new URL("../public/shots/", import.meta.url));
const videoTmp = join(tmpdir(), "dimmi-agent-studio-videos");

const VIEWPORT = { width: 1680, height: 1000 };
const OPERATOR_VIEWPORT = { width: 1680, height: 720 };
const VIDEO_SIZE = { width: 1680, height: 1000 };
const OPERATOR_VIDEO_SIZE = { width: 1680, height: 720 };

type Clip = { x: number; y: number; width: number; height: number };

async function signIn(page: Page, scheme: "light" | "dark") {
  await page.goto(`${base}/login`);
  await page.locator('input[type="email"]').fill(email);
  await page.locator('input[type="password"]').fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.waitForURL(/\/d\//);
  await page
    .getByRole("button", { name: scheme === "dark" ? "Use dark mode" : "Use light mode" })
    .click()
    .catch(() => undefined);
  await page.waitForTimeout(300);
}

async function openSky(page: Page) {
  await page.goto(`${base}/d/stowline/agents`);
  await page.getByText("Sky", { exact: true }).first().click();
  await page.getByText("Guidance", { exact: true }).first().waitFor();
  await page.evaluate(() => document.fonts.ready);
  await page.waitForTimeout(700);
}

async function ensureTestingOpen(page: Page, open: boolean) {
  const hide = page.getByRole("button", { name: "Hide testing", exact: true });
  const show = page.getByRole("button", { name: "Show testing", exact: true });
  const isOpen = await hide.isVisible().catch(() => false);
  if (open && !isOpen) {
    await show.click();
    await page.waitForTimeout(400);
  } else if (!open && isOpen) {
    await hide.click();
    await page.waitForTimeout(400);
  }
}

async function ensureOperatorOpen(page: Page, open: boolean) {
  const close = page.getByRole("button", { name: /close operator/i });
  const isOpen = await close.isVisible().catch(() => false);
  if (open && !isOpen) {
    await page.getByRole("button", { name: "Operator", exact: true }).click();
    await page.waitForTimeout(500);
  } else if (!open && isOpen) {
    await close.click();
    await page.waitForTimeout(400);
  }
}

async function openDiagnosisThread(page: Page) {
  const changeset = page.getByText("Tighten USPS refund window check").first();
  if (await changeset.isVisible().catch(() => false)) {
    return;
  }
  const pastChats = page.getByRole("button", { name: "Past chats", exact: true });
  await pastChats.waitFor();
  await pastChats.click();
  await page.getByText("Why Sky fails carrier adjustment disputes").first().waitFor();
  await page.getByText("Why Sky fails carrier adjustment disputes").first().click();
  await changeset.waitFor();
  // Dismiss the Past chats popover cleanly if React Aria kept it open.
  await page.keyboard.press("Escape");
  await page.waitForTimeout(350);
  const internalBadge = page.getByText("Internal", { exact: true }).first();
  if (await internalBadge.isVisible().catch(() => false)) {
    await internalBadge.click();
  }
  await page.waitForTimeout(350);
}

async function panels(page: Page) {
  return page.evaluate(() =>
    [...document.querySelectorAll("div, section, aside, main")]
      .filter((el) => {
        const r = el.getBoundingClientRect();
        return (
          r.height > window.innerHeight * 0.75 &&
          r.width > 200 &&
          r.width < 1300 &&
          Number.parseFloat(getComputedStyle(el).borderTopLeftRadius) >= 12
        );
      })
      .map((el) => {
        const r = el.getBoundingClientRect();
        return { x: r.x, y: r.y, width: r.width, height: r.height };
      })
      .sort((a, b) => a.x - b.x),
  );
}

async function columnBox(page: Page, text: string, above: number, height: number): Promise<Clip> {
  const label = page.getByText(text, { exact: true }).first();
  const box = await label.evaluate((el) => {
    let node: Element = el;
    while (node.parentElement && node.getBoundingClientRect().width < 580) {
      node = node.parentElement;
    }
    const col = node.getBoundingClientRect();
    return { x: col.x, width: col.width, top: el.getBoundingClientRect().top };
  });
  const x = Math.max(0, Math.round(box.x - 16));
  const y = Math.max(0, Math.round(box.top - above));
  const width = Math.min(VIEWPORT.width - x, Math.round(box.width + 32));
  const clampedH = Math.min(height, VIEWPORT.height - y);
  return { x, y, width, height: clampedH };
}

async function modalBox(page: Page, inset = 4): Promise<Clip> {
  const box = await page.getByRole("dialog").first().boundingBox();
  if (!box) {
    throw new Error("Compare dialog not visible");
  }
  const x = Math.max(0, Math.round(box.x + inset));
  const y = Math.max(0, Math.round(box.y + inset));
  const width = Math.max(100, Math.round(box.width - inset * 2));
  const height = Math.max(100, Math.round(box.height - inset * 2));
  return { x, y, width, height };
}

async function capture4K(page: Page, name: string, scheme: "light" | "dark", clip?: Clip) {
  await page.mouse.move(2, VIEWPORT.height - 2);
  await page.evaluate(() => document.fonts.ready);
  await page.waitForTimeout(700);
  // True 4K density: ~3840px wide for full window (2.2857x), 2.5x for cropped panels.
  const scale = clip ? 2.5 : 3840 / VIEWPORT.width;
  const cdp = await page.context().newCDPSession(page);
  const { data } = await cdp.send("Page.captureScreenshot", {
    format: "webp",
    quality: 94,
    clip: clip ? { ...clip, scale } : { x: 0, y: 0, width: VIEWPORT.width, height: VIEWPORT.height, scale },
  });
  const file = join(out, `${name}-${scheme}.webp`);
  writeFileSync(file, Buffer.from(data, "base64"));
  const w = clip ? Math.round(clip.width) : VIEWPORT.width;
  const h = clip ? Math.round(clip.height) : VIEWPORT.height;
  process.stdout.write(
    `shot ${name}-${scheme}.webp (${w}x${h} CSS @ ${scale.toFixed(2)}x -> ${Math.round(w * scale)}x${Math.round(h * scale)}px)\n`,
  );
  return { width: w, height: h };
}

function encodeMp4(
  rawWebm: string,
  destMp4: string,
  trimStartSec: number,
  crop?: Clip,
  viewport: { width: number; height: number } = VIEWPORT,
  videoSize: { width: number; height: number } = VIDEO_SIZE,
) {
  const filters: string[] = [];
  if (crop) {
    const sx = videoSize.width / viewport.width;
    const sy = videoSize.height / viewport.height;
    const cw = Math.floor((crop.width * sx) / 2) * 2;
    const ch = Math.floor((crop.height * sy) / 2) * 2;
    const cx = Math.max(0, Math.floor((crop.x * sx) / 2) * 2);
    const cy = Math.max(0, Math.floor((crop.y * sy) / 2) * 2);
    filters.push(`crop=${cw}:${ch}:${cx}:${cy}`);
  } else {
    filters.push("scale=trunc(iw/2)*2:trunc(ih/2)*2");
  }
  const args = [
    "-y",
    "-ss",
    trimStartSec.toFixed(2),
    "-i",
    rawWebm,
    "-vf",
    filters.join(","),
    "-c:v",
    "libx264",
    "-crf",
    "17",
    "-preset",
    "fast",
    "-pix_fmt",
    "yuv420p",
    "-movflags",
    "+faststart",
    "-an",
    destMp4,
  ];
  execFileSync("ffmpeg", args, { stdio: "pipe" });
  process.stdout.write(`video ${destMp4}\n`);
}

async function smoothClick(page: Page, locator: ReturnType<Page["getByText"]>) {
  const box = await locator.boundingBox();
  if (!box) {
    throw new Error("Target not visible for smoothClick");
  }
  await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2, { steps: 18 });
  await page.waitForTimeout(180);
  await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
}

async function captureStills(browser: Awaited<ReturnType<typeof chromium.launch>>) {
  const dims: Record<string, { width: number; height: number }> = {};

  for (const scheme of ["light", "dark"] as const) {
    const context = await browser.newContext({
      viewport: VIEWPORT,
      deviceScaleFactor: 2,
      colorScheme: scheme,
      reducedMotion: "reduce",
    });
    const page = await context.newPage();
    await signIn(page, scheme);

    // 1. Full Agent Studio hero: Build + Simulations list (showing Passing, Flaky, Differs from live)
    await openSky(page);
    await ensureOperatorOpen(page, false);
    await ensureTestingOpen(page, true);
    await page.getByRole("tab", { name: /Simulations/ }).click();
    await page.getByText("Disputes a carrier adjustment", { exact: false }).first().waitFor();
    dims["agent-studio-hero"] = await capture4K(page, "agent-studio-hero", scheme);

    // 2. Guidance & Scope close-up (hide testing so Build sits wide & clean)
    await ensureTestingOpen(page, false);
    await page.getByText("Everything", { exact: true }).first().scrollIntoViewIfNeeded();
    await page.waitForTimeout(350);
    dims["agent-studio-guidance"] = await capture4K(
      page,
      "agent-studio-guidance",
      scheme,
      await columnBox(page, "Guidance", 20, 670),
    );

    // 2b. Fresh agent-voice capture (Identity + Guidance)
    dims["agent-voice"] = await capture4K(page, "agent-voice", scheme, await columnBox(page, "Guidance", 240, 486));

    // 3. History diff (Version 1 vs Working copy)
    await page.getByRole("tab", { name: "History", exact: true }).click();
    await page.getByText("Compare", { exact: true }).first().waitFor();
    await page.waitForTimeout(400);
    dims["agent-studio-history"] = await capture4K(
      page,
      "agent-studio-history",
      scheme,
      await columnBox(page, "Compare", 24, 680),
    );

    // 4. Simulations panel opened on Flaky simulation ("Voided USPS label") showing dots + checks + judge reasons
    await page.getByRole("tab", { name: "Build", exact: true }).click();
    await ensureTestingOpen(page, true);
    await page.getByRole("tab", { name: /Simulations/ }).click();
    await page.getByText("Voided USPS label, asks when the money comes back").first().click();
    await page.getByText("Checks", { exact: true }).first().waitFor();
    await page.waitForTimeout(500);
    const simPanels = await panels(page);
    const side = simPanels[simPanels.length - 1];
    const sideClip: Clip = {
      x: Math.max(0, Math.round(side.x - 4)),
      y: 0,
      width: Math.min(VIEWPORT.width - Math.max(0, Math.round(side.x - 4)), Math.round(side.width + 8)),
      height: 940,
    };
    dims["agent-studio-simulations"] = await capture4K(page, "agent-studio-simulations", scheme, sideClip);

    // 5. Draft vs. Live Compare modal on "Disputes a carrier adjustment and wants the money today"
    await page
      .getByRole("button", { name: "Back" })
      .first()
      .click()
      .catch(async () => {
        await page.locator("button:has(svg)").filter({ visible: true }).first().click();
      });
    // Re-open Sky Simulations list cleanly to click Disputes a carrier adjustment
    await openSky(page);
    await ensureOperatorOpen(page, false);
    await ensureTestingOpen(page, true);
    await page.getByRole("tab", { name: /Simulations/ }).click();
    await page.getByText("Disputes a carrier adjustment and wants the money today").first().click();
    await page.getByRole("button", { name: "Compare", exact: true }).waitFor();
    await page.getByRole("button", { name: "Compare", exact: true }).click();
    await page.getByRole("dialog").first().waitFor();
    await page.waitForTimeout(600);
    dims["agent-studio-compare"] = await capture4K(page, "agent-studio-compare", scheme, await modalBox(page, 4));
    await page.keyboard.press("Escape");
    await page.waitForTimeout(300);

    // 6. Operator panel inside Agent Studio on a compact 720px height so header + diagnosis + changeset card fill edge to edge
    await page.setViewportSize(OPERATOR_VIEWPORT);
    await ensureTestingOpen(page, false);
    await ensureOperatorOpen(page, true);
    await openDiagnosisThread(page);
    await page.waitForTimeout(500);
    const opPanels = await panels(page);
    const op = opPanels[opPanels.length - 1];
    const opClip: Clip = {
      x: Math.max(0, Math.round(op.x + 2)),
      y: Math.max(0, Math.round(op.y + 2)),
      width: Math.max(100, Math.round(op.width - 4)),
      height: Math.min(OPERATOR_VIEWPORT.height - Math.max(0, Math.round(op.y + 2)), 660),
    };
    dims["agent-studio-operator"] = await capture4K(page, "agent-studio-operator", scheme, opClip);

    await context.close();
  }

  return dims;
}

async function captureVideos(browser: Awaited<ReturnType<typeof chromium.launch>>) {
  rmSync(videoTmp, { recursive: true, force: true });
  mkdirSync(videoTmp, { recursive: true });

  for (const scheme of ["light", "dark"] as const) {
    // Video 1: Full Agent Studio tour (hero bleed)
    {
      const context = await browser.newContext({
        viewport: VIEWPORT,
        deviceScaleFactor: 2,
        colorScheme: scheme,
        recordVideo: { dir: videoTmp, size: VIDEO_SIZE },
      });
      const page = await context.newPage();
      const t0 = Date.now();
      await signIn(page, scheme);
      await openSky(page);
      await ensureOperatorOpen(page, false);
      await ensureTestingOpen(page, true);
      await page.getByRole("tab", { name: /Simulations/ }).click();
      await page.getByText("Disputes a carrier adjustment").first().waitFor();
      await page.waitForTimeout(400);
      const trimStart = (Date.now() - t0) / 1000;

      // Hold initial state (Build + Simulations list with Passing/Flaky/Differs from live)
      await page.waitForTimeout(1600);
      // Click into Disputes simulation
      await smoothClick(page, page.getByText("Disputes a carrier adjustment and wants the money today").first());
      await page.getByRole("button", { name: "Compare", exact: true }).waitFor();
      await page.waitForTimeout(1400);
      // Click Compare to open side-by-side Draft vs Live
      await smoothClick(page, page.getByRole("button", { name: "Compare", exact: true }));
      await page.getByRole("dialog").first().waitFor();
      await page.waitForTimeout(2600);
      // Close dialog and open Operator panel
      await page.keyboard.press("Escape");
      await page.waitForTimeout(500);
      await smoothClick(page, page.getByRole("button", { name: "Operator", exact: true }));
      await openDiagnosisThread(page);
      await page.waitForTimeout(2400);

      const rawPath = await page.video()?.path();
      await context.close();
      if (rawPath) {
        encodeMp4(rawPath, join(out, `agent-studio-tour-${scheme}.mp4`), trimStart);
      }
    }

    // Video 2: Compare Draft vs. Live walkthrough (cropped to modal)
    {
      const context = await browser.newContext({
        viewport: VIEWPORT,
        deviceScaleFactor: 2,
        colorScheme: scheme,
        recordVideo: { dir: videoTmp, size: VIDEO_SIZE },
      });
      const page = await context.newPage();
      const t0 = Date.now();
      await signIn(page, scheme);
      await openSky(page);
      await ensureOperatorOpen(page, false);
      await ensureTestingOpen(page, true);
      await page.getByRole("tab", { name: /Simulations/ }).click();
      await page.getByText("Disputes a carrier adjustment and wants the money today").first().click();
      await page.getByRole("button", { name: "Compare", exact: true }).click();
      await page.getByRole("dialog").first().waitFor();
      await page.waitForTimeout(500);
      const clip = await modalBox(page, 4);
      const trimStart = (Date.now() - t0) / 1000;

      // Pause on Draft (Passing) vs Live (Failing)
      await page.waitForTimeout(1600);
      // Gently glide across Draft passing checks, then over to Live failing check
      await page.mouse.move(clip.x + clip.width * 0.28, clip.y + clip.height * 0.72, { steps: 22 });
      await page.waitForTimeout(1800);
      await page.mouse.move(clip.x + clip.width * 0.75, clip.y + clip.height * 0.76, { steps: 24 });
      await page.waitForTimeout(2400);

      const rawPath = await page.video()?.path();
      await context.close();
      if (rawPath) {
        encodeMp4(rawPath, join(out, `agent-studio-compare-${scheme}.mp4`), trimStart, clip);
      }
    }

    // Video 3: Operator diagnosing & proposing a fix inside Agent Studio
    {
      const context = await browser.newContext({
        viewport: OPERATOR_VIEWPORT,
        deviceScaleFactor: 2,
        colorScheme: scheme,
        recordVideo: { dir: videoTmp, size: OPERATOR_VIDEO_SIZE },
      });
      const page = await context.newPage();
      const t0 = Date.now();
      await signIn(page, scheme);
      await openSky(page);
      await ensureTestingOpen(page, false);
      await ensureOperatorOpen(page, true);
      await openDiagnosisThread(page);
      await page.waitForTimeout(500);
      const opPanels = await panels(page);
      const op = opPanels[opPanels.length - 1];
      const clip: Clip = {
        x: Math.max(0, Math.round(op.x + 2)),
        y: Math.max(0, Math.round(op.y + 2)),
        width: Math.max(100, Math.round(op.width - 4)),
        height: Math.min(OPERATOR_VIEWPORT.height - Math.max(0, Math.round(op.y + 2)), 660),
      };
      const trimStart = (Date.now() - t0) / 1000;

      await page.waitForTimeout(1400);
      // Expand the tool steps, hold so viewers read what Operator checked, then glide to Apply
      const stepsBtn = page.getByText(/Worked through/i).first();
      if (await stepsBtn.isVisible().catch(() => false)) {
        await smoothClick(page, stepsBtn);
        await page.waitForTimeout(2200);
      }
      const applyBtn = page.getByRole("button", { name: "Apply", exact: true }).first();
      if (await applyBtn.isVisible().catch(() => false)) {
        const b = await applyBtn.boundingBox();
        if (b) {
          await page.mouse.move(b.x + b.width / 2, b.y + b.height / 2, { steps: 22 });
        }
      }
      await page.waitForTimeout(2400);

      const rawPath = await page.video()?.path();
      await context.close();
      if (rawPath) {
        encodeMp4(
          rawPath,
          join(out, `agent-studio-operator-${scheme}.mp4`),
          trimStart,
          clip,
          OPERATOR_VIEWPORT,
          OPERATOR_VIDEO_SIZE,
        );
      }
    }
  }

  rmSync(videoTmp, { recursive: true, force: true });
}

mkdirSync(out, { recursive: true });
const browser = await chromium.launch();
try {
  const dims = await captureStills(browser);
  await captureVideos(browser);
  process.stdout.write(`DIMS=${JSON.stringify(dims)}\n`);
} finally {
  await browser.close();
}
