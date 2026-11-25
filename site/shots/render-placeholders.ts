/*
 * Renders crisp light & dark WebP stills and deterministic looping MP4 videos
 * for all help-center, routines, and channel marketing shots.
 *
 *   pnpm exec tsx site/shots/render-placeholders.ts
 *   pnpm exec tsx site/shots/render-placeholders.ts channel-slack-hero
 */
import { spawn } from "node:child_process";
import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

import { type Browser, chromium } from "@playwright/test";

import { CORE_SCENES } from "./placeholders/scenes-core.ts";
import { MESSENGER_EMAIL_SCENES } from "./placeholders/scenes-messenger-email.ts";
import { META_API_SCENES } from "./placeholders/scenes-meta-api.ts";
import { SLACK_WHATSAPP_SCENES } from "./placeholders/scenes-slack-whatsapp.ts";
import type { SceneSpec, Scheme } from "./placeholders/theme.ts";

const OUT_DIR = fileURLToPath(new URL("../public/shots/", import.meta.url));
const ALL_SCENES: SceneSpec[] = [
  ...CORE_SCENES,
  ...MESSENGER_EMAIL_SCENES,
  ...SLACK_WHATSAPP_SCENES,
  ...META_API_SCENES,
];

const FPS = 24;
const CONCURRENCY = 4;

async function captureWebp(browser: Browser, scene: SceneSpec, scheme: Scheme, atTime?: number): Promise<void> {
  const scale = scene.width >= 1600 ? 2 : 2.25;
  const context = await browser.newContext({
    viewport: { width: scene.width, height: scene.height },
    deviceScaleFactor: 1,
    colorScheme: scheme,
  });
  const page = await context.newPage();
  await page.setContent(scene.html(scheme), { waitUntil: "load" });
  await page.evaluate(() => document.fonts.ready);
  if (typeof atTime === "number") {
    await page.evaluate((t) => {
      const fn = (window as unknown as { __setTime?: (n: number) => void }).__setTime;
      if (fn) {
        fn(t);
      }
    }, atTime);
  }
  const cdp = await context.newCDPSession(page);
  const { data } = await cdp.send("Page.captureScreenshot", {
    format: "webp",
    quality: 94,
    clip: { x: 0, y: 0, width: scene.width, height: scene.height, scale },
  });
  writeFileSync(join(OUT_DIR, `${scene.name}-${scheme}.webp`), Buffer.from(data, "base64"));
  await context.close();
}

async function renderMp4(browser: Browser, scene: SceneSpec, scheme: Scheme): Promise<void> {
  const duration = scene.duration ?? 4.0;
  const totalFrames = Math.round(duration * FPS);
  const videoScale = scene.width >= 1600 ? 1 : 1.5;
  const outMp4 = join(OUT_DIR, `${scene.name}-${scheme}.mp4`);

  const context = await browser.newContext({
    viewport: { width: scene.width, height: scene.height },
    deviceScaleFactor: 1,
    colorScheme: scheme,
  });
  const page = await context.newPage();
  await page.setContent(scene.html(scheme), { waitUntil: "load" });
  await page.evaluate(() => document.fonts.ready);
  const cdp = await context.newCDPSession(page);

  const ffmpeg = spawn(
    "ffmpeg",
    [
      "-y",
      "-f",
      "image2pipe",
      "-vcodec",
      "mjpeg",
      "-framerate",
      String(FPS),
      "-i",
      "-",
      "-vf",
      "scale=trunc(iw/2)*2:trunc(ih/2)*2,format=yuv420p",
      "-c:v",
      "libx264",
      "-crf",
      "18",
      "-preset",
      "fast",
      "-pix_fmt",
      "yuv420p",
      "-movflags",
      "+faststart",
      "-an",
      outMp4,
    ],
    { stdio: ["pipe", "ignore", "pipe"] },
  );

  let stderr = "";
  ffmpeg.stderr.on("data", (chunk: Buffer) => {
    stderr += chunk.toString();
  });

  const done = new Promise<void>((resolve, reject) => {
    ffmpeg.on("error", reject);
    ffmpeg.on("close", (code) => {
      if (code === 0) {
        resolve();
      } else {
        reject(new Error(`ffmpeg exited ${code} for ${outMp4}: ${stderr.slice(-400)}`));
      }
    });
  });

  for (let i = 0; i < totalFrames; i += 1) {
    const t = i / FPS;
    await page.evaluate((sec) => {
      const fn = (window as unknown as { __setTime?: (n: number) => void }).__setTime;
      if (fn) {
        fn(sec);
      }
    }, t);
    const { data } = await cdp.send("Page.captureScreenshot", {
      format: "jpeg",
      quality: 94,
      clip: { x: 0, y: 0, width: scene.width, height: scene.height, scale: videoScale },
    });
    const buf = Buffer.from(data, "base64");
    if (!ffmpeg.stdin.write(buf)) {
      await new Promise<void>((resolve) => ffmpeg.stdin.once("drain", resolve));
    }
  }

  ffmpeg.stdin.end();
  await done;
  await context.close();
}

async function processJob(browser: Browser, scene: SceneSpec, scheme: Scheme): Promise<void> {
  if (scene.kind === "video") {
    await captureWebp(browser, scene, scheme, scene.posterTime ?? (scene.duration ?? 4) - 0.4);
    await renderMp4(browser, scene, scheme);
    process.stdout.write(`rendered video + poster: ${scene.name}-${scheme}\n`);
    return;
  }
  await captureWebp(browser, scene, scheme);
  process.stdout.write(`rendered still: ${scene.name}-${scheme}\n`);
}

async function main(): Promise<void> {
  mkdirSync(OUT_DIR, { recursive: true });
  const filter = process.argv.slice(2);
  const selected = filter.length > 0 ? ALL_SCENES.filter((s) => filter.includes(s.name)) : ALL_SCENES;
  const jobs: { scene: SceneSpec; scheme: Scheme }[] = [];
  for (const scene of selected) {
    jobs.push({ scene, scheme: "light" });
    jobs.push({ scene, scheme: "dark" });
  }

  const browser = await chromium.launch();
  let cursor = 0;

  async function worker(): Promise<void> {
    while (cursor < jobs.length) {
      const idx = cursor;
      cursor += 1;
      const job = jobs[idx];
      if (!job) {
        return;
      }
      await processJob(browser, job.scene, job.scheme);
    }
  }

  await Promise.all(Array.from({ length: CONCURRENCY }, () => worker()));
  await browser.close();
  process.stdout.write(`done: ${jobs.length} variants across ${selected.length} scenes\n`);
}

main().catch((err: unknown) => {
  process.stderr.write(`${err instanceof Error ? (err.stack ?? err.message) : String(err)}\n`);
  process.exit(1);
});
