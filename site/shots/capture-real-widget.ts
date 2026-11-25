/*
 * Captures the ACTUAL live dimmi web messenger widget (/widget/v1/loader.js
 * in preview mode) in both light and dark themes for:
 *   - channel-messenger-hero-{light,dark}.{webp,mp4} (1680x1000)
 *   - channel-messenger-home-{light,dark}.{webp,mp4} (720x640)
 *
 * Requires a local dimmi server serving /widget/v1/loader.js (default: http://127.0.0.1:8084).
 *
 *   node site/shots/capture-real-widget.ts
 */
import { execFileSync } from "node:child_process";
import { mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { chromium, type Page } from "@playwright/test";

import { BASE_CSS, ICONS, type Scheme } from "./placeholders/theme.ts";

const WIDGET_ORIGIN = process.env.WIDGET_ORIGIN ?? "http://127.0.0.1:8097";
const INTER_B64 = readFileSync(
  resolve(process.cwd(), "node_modules/@fontsource-variable/inter/files/inter-latin-wght-normal.woff2"),
).toString("base64");
const OUT_DIR = fileURLToPath(new URL("../public/shots/", import.meta.url));
const TMP_DIR = join(tmpdir(), "dimmi-real-widget-videos");

function hostHeroPage(scheme: Scheme): string {
  return `<!doctype html>
<html lang="en" class="${scheme === "dark" ? "dark" : ""}">
<head>
<meta charset="utf-8" />
<style>${BASE_CSS}</style>
</head>
<body>
  <div style="width: 100%; height: 100%; background: var(--bg-shell); display: flex; flex-direction: column; overflow: hidden;">
    <div style="height: 46px; background: var(--bg); border-bottom: 1px solid var(--border); padding: 0 22px; display: flex; align-items: center; justify-content: space-between;">
      <div style="display: flex; align-items: center; gap: 18px;">
        <div style="display: flex; gap: 7px;">
          <span style="width: 11px; height: 11px; border-radius: 999px; background: var(--border-strong);"></span>
          <span style="width: 11px; height: 11px; border-radius: 999px; background: var(--border-strong);"></span>
          <span style="width: 11px; height: 11px; border-radius: 999px; background: var(--border-strong);"></span>
        </div>
        <div class="mono" style="padding: 4px 12px; border-radius: 7px; background: var(--bg-sub); border: 1px solid var(--border); font-size: 12px; color: var(--fg-2);">
          https://app.stowline.co/shipments
        </div>
      </div>
      <div style="display: flex; align-items: center; gap: 10px; font-size: 12.5px; color: var(--fg-2);">
        <span class="pill pill-accent">${ICONS.lock} Signed in as Hana Kowalski (Growth plan)</span>
      </div>
    </div>

    <div style="flex: 1; display: flex; padding: 26px 32px; gap: 24px;">
      <div style="width: 210px; display: flex; flex-direction: column; gap: 6px; padding-top: 6px;">
        <div style="display: flex; align-items: center; gap: 9px; padding: 6px 10px; margin-bottom: 12px;">
          <div style="width: 26px; height: 26px; border-radius: 7px; background: var(--fg); color: var(--bg); font-weight: 700; font-size: 13px; display: flex; align-items: center; justify-content: center;">S</div>
          <span style="font-weight: 600; font-size: 15px;">Stowline</span>
        </div>
        ${["Overview", "Shipments", "Batch labels", "Carriers", "Returns", "Billing"]
          .map(
            (item, i) =>
              `<div style="padding: 9px 12px; border-radius: 9px; font-size: 13.5px; font-weight: ${i === 1 ? "600" : "500"}; background: ${i === 1 ? "var(--bg)" : "transparent"}; border: 1px solid ${i === 1 ? "var(--border)" : "transparent"}; color: ${i === 1 ? "var(--fg)" : "var(--fg-2)"};">${item}</div>`,
          )
          .join("")}
      </div>

      <div style="flex: 1; display: flex; flex-direction: column; gap: 18px; min-width: 0;">
        <div style="display: flex; align-items: center; justify-content: space-between;">
          <div>
            <h2 style="font-size: 22px; font-weight: 600; letter-spacing: -0.02em;">Shipments</h2>
            <p style="font-size: 13.5px; color: var(--fg-3); margin-top: 2px;">142 labels created today · USPS &amp; UPS connected</p>
          </div>
          <span class="btn btn-primary">+ Create label</span>
        </div>

        <div style="display: grid; grid-template-columns: repeat(3, 1fr); gap: 14px;">
          ${[
            ["In transit", "86", "+12 today"],
            ["Delivered", "1,294", "99.1% on time"],
            ["Carrier adjustments", "2", "Action needed"],
          ]
            .map(
              ([label, value, sub]) => `<div class="card" style="padding: 16px 18px;">
              <div style="font-size: 12px; color: var(--fg-3);">${label}</div>
              <div style="font-size: 26px; font-weight: 600; letter-spacing: -0.02em; margin-top: 4px;">${value}</div>
              <div style="font-size: 12px; color: var(--fg-2); margin-top: 3px;">${sub}</div>
            </div>`,
            )
            .join("")}
        </div>

        <div class="card" style="padding: 0; overflow: hidden; flex: 1;">
          <div style="display: grid; grid-template-columns: 1.1fr 1.5fr 1fr 1fr 0.8fr; padding: 12px 18px; background: var(--bg-sub); border-bottom: 1px solid var(--border); font-size: 11.5px; font-weight: 600; color: var(--fg-3); text-transform: uppercase; letter-spacing: 0.04em;">
            <span>Tracking</span><span>Recipient</span><span>Carrier</span><span>Status</span><span>Label</span>
          </div>
          ${[
            ["#STW-4829", "Kieran Patel · Austin, TX", "USPS Priority", "Adjustment +$4.20", "pill-amber", "$11.40"],
            ["#STW-4828", "Nadia Okafor · Seattle, WA", "UPS Ground", "In transit", "pill-blue", "$14.80"],
            ["#STW-4827", "Lucas Berg · Denver, CO", "USPS Ground", "Delivered", "pill-accent", "$8.15"],
            ["#STW-4826", "Sofia Lind · Chicago, IL", "UPS 2nd Day", "In transit", "pill-blue", "$22.50"],
            ["#STW-4825", "Devon Brooks · Boston, MA", "USPS Priority", "Delivered", "pill-accent", "$10.90"],
            ["#STW-4824", "Mina Park · Portland, OR", "FedEx Home", "Label created", "pill-muted", "$16.20"],
          ]
            .map(
              ([id, rec, car, st, cls, price], idx) =>
                `<div style="display: grid; grid-template-columns: 1.1fr 1.5fr 1fr 1fr 0.8fr; align-items: center; padding: 14px 18px; font-size: 13px; ${idx > 0 ? "border-top: 1px solid var(--border);" : ""}">
              <span class="mono" style="font-weight: 600;">${id}</span>
              <span style="color: var(--fg-2);">${rec}</span>
              <span style="color: var(--fg-2);">${car}</span>
              <span><span class="pill ${cls}">${st}</span></span>
              <span class="mono" style="color: var(--fg-2);">${price}</span>
            </div>`,
            )
            .join("")}
        </div>
      </div>
    </div>
  </div>
  <script src="/widget/v1/loader.js" data-preview data-latency="120" data-app-id="preview"></script>
</body>
</html>`;
}

function hostHomePage(scheme: Scheme): string {
  return `<!doctype html>
<html lang="en" class="${scheme === "dark" ? "dark" : ""}">
<head>
<meta charset="utf-8" />
<style>
${BASE_CSS}
body {
  background: var(--bg-shell);
}
</style>
</head>
<body>
  <script src="/widget/v1/loader.js" data-preview data-latency="120" data-app-id="preview"></script>
</body>
</html>`;
}

async function bootRealWidget(page: Page, scheme: Scheme, opts: { centerInViewport?: boolean } = {}) {
  await page.waitForFunction(() => typeof (window as unknown as { Dimmi?: unknown }).Dimmi === "function");
  await page.evaluate(
    ({ theme, center }) => {
      const dimmi = (window as unknown as { Dimmi: (...args: unknown[]) => void }).Dimmi;
      dimmi("update", {
        appearance: {
          primary_color: "#059669",
          theme,
          position: "right",
          padding_x: center ? 24 : 32,
          padding_y: center ? 16 : 24,
          font: "system",
        },
      });
      dimmi("open");
    },
    { theme: scheme, center: Boolean(opts.centerInViewport) },
  );
  // Wait for shadow root panel to mount and open
  await page.waitForFunction(() => {
    const hosts = [...document.querySelectorAll("*")].filter((el) => el.shadowRoot);
    return hosts.some((el) => el.shadowRoot?.querySelector(".panel"));
  });
  // Inject crisp Inter Variable font into the shadow root + replace sample "Acme" with "Stowline"
  await page.evaluate(
    ({ interB64, center }) => {
      const host = [...document.querySelectorAll("*")].find((el) => el.shadowRoot?.querySelector(".panel"));
      if (!host?.shadowRoot) {
        return;
      }
      const style = document.createElement("style");
      style.textContent = `
        @font-face {
          font-family: "Inter Variable";
          src: url("data:font/woff2;base64,${interB64}") format("woff2");
          font-weight: 100 900;
          font-style: normal;
          font-display: block;
        }
        :host, .messenger, .panel, button, input, textarea {
          font-family: "Inter Variable", -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif !important;
        }
        ${
          center
            ? `
        .launcher { display: none !important; }
        :host {
          position: fixed !important;
          inset: 0 !important;
          width: 100vw !important;
          height: 100vh !important;
        }
        .messenger {
          position: fixed !important;
          inset: 0 !important;
          width: 100vw !important;
          height: 100vh !important;
          display: flex !important;
          align-items: flex-start !important;
          justify-content: center !important;
          padding-top: 18px !important;
        }
        .panel {
          position: relative !important;
          inset: auto !important;
          left: auto !important;
          right: auto !important;
          top: auto !important;
          bottom: auto !important;
          margin: 0 auto !important;
          width: 420px !important;
          height: 590px !important;
          max-height: 590px !important;
          transform: none !important;
        }`
            : ""
        }
      `;
      const shadow = host.shadowRoot;
      shadow.appendChild(style);
      const replaceAcme = () => {
        const walker = document.createTreeWalker(shadow, NodeFilter.SHOW_TEXT);
        let node = walker.nextNode();
        while (node) {
          if (node.nodeValue?.includes("Acme")) {
            node.nodeValue = node.nodeValue.replaceAll("Acme", "Stowline");
          }
          node = walker.nextNode();
        }
      };
      replaceAcme();
      new MutationObserver(replaceAcme).observe(host.shadowRoot, {
        childList: true,
        subtree: true,
        characterData: true,
      });
    },
    { interB64: INTER_B64, center: Boolean(opts.centerInViewport) },
  );
  await page.evaluate(() => document.fonts.ready);
  await page.waitForTimeout(450);
}

async function clickInWidget(page: Page, selector: string) {
  const box = await page.evaluate((sel) => {
    const host = [...document.querySelectorAll("*")].find((el) => el.shadowRoot?.querySelector(".panel"));
    const el = host?.shadowRoot?.querySelector<HTMLElement>(sel);
    if (!el) {
      return null;
    }
    const r = el.getBoundingClientRect();
    return { x: r.x + r.width / 2, y: r.y + r.height / 2 };
  }, selector);
  if (!box) {
    throw new Error(`Widget element not found: ${selector}`);
  }
  await page.mouse.move(box.x, box.y, { steps: 14 });
  await page.waitForTimeout(120);
  await page.mouse.click(box.x, box.y);
}

function encodeMp4(rawWebm: string, destMp4: string, trimStartSec: number, width: number, height: number) {
  execFileSync(
    "ffmpeg",
    [
      "-y",
      "-ss",
      trimStartSec.toFixed(2),
      "-i",
      rawWebm,
      "-vf",
      `scale=${width}:${height}:flags=lanczos,format=yuv420p`,
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
    ],
    { stdio: "pipe" },
  );
}

async function captureHero(browser: Awaited<ReturnType<typeof chromium.launch>>, scheme: Scheme) {
  const context = await browser.newContext({
    viewport: { width: 1680, height: 1000 },
    deviceScaleFactor: 2,
    colorScheme: scheme,
    recordVideo: { dir: TMP_DIR, size: { width: 1680, height: 1000 } },
  });
  const page = await context.newPage();
  const startedAt = Date.now();

  await page.route(`${WIDGET_ORIGIN}/__shot-messenger-hero`, (route) =>
    route.fulfill({ status: 200, contentType: "text/html; charset=utf-8", body: hostHeroPage(scheme) }),
  );
  await page.goto(`${WIDGET_ORIGIN}/__shot-messenger-hero`, { waitUntil: "load" });
  await bootRealWidget(page, scheme);

  // Start recording cleanly on the real widget Home tab
  const trimStart = (Date.now() - startedAt) / 1000;
  await page.waitForTimeout(900);

  // Click into Messages tab, then open the sample conversation with AI agent + source citation + teammate reply
  await clickInWidget(page, 'button[data-tab="messages"]');
  await page.waitForTimeout(700);
  await clickInWidget(page, '.view[data-view="messages"] [data-conversation]');
  await page.waitForTimeout(900);

  // Capture crisp 2x WebP poster right on the rich conversation thread!
  const cdp = await context.newCDPSession(page);
  const { data } = await cdp.send("Page.captureScreenshot", {
    format: "webp",
    quality: 94,
    clip: { x: 0, y: 0, width: 1680, height: 1000, scale: 2 },
  });
  writeFileSync(join(OUT_DIR, `channel-messenger-hero-${scheme}.webp`), Buffer.from(data, "base64"));

  // Type a real message in the real widget composer and send it so the live typing + reply plays in the video!
  await clickInWidget(page, ".composer textarea, .composer [contenteditable], .composer input");
  await page.keyboard.type("Thanks! Is the webhook active now?", { delay: 32 });
  await page.waitForTimeout(250);
  await page.keyboard.press("Enter");
  await page.waitForTimeout(2400);

  const videoPath = await page.video()?.path();
  await context.close();
  if (!videoPath) {
    throw new Error("Missing recorded video for channel-messenger-hero");
  }
  encodeMp4(videoPath, join(OUT_DIR, `channel-messenger-hero-${scheme}.mp4`), trimStart, 1680, 1000);
  process.stdout.write(`captured real widget hero: channel-messenger-hero-${scheme}\n`);
}

async function captureHome(browser: Awaited<ReturnType<typeof chromium.launch>>, scheme: Scheme) {
  const context = await browser.newContext({
    viewport: { width: 720, height: 640 },
    deviceScaleFactor: 2,
    colorScheme: scheme,
    recordVideo: { dir: TMP_DIR, size: { width: 720, height: 640 } },
  });
  const page = await context.newPage();
  const startedAt = Date.now();

  await page.route(`${WIDGET_ORIGIN}/__shot-messenger-home`, (route) =>
    route.fulfill({ status: 200, contentType: "text/html; charset=utf-8", body: hostHomePage(scheme) }),
  );
  await page.goto(`${WIDGET_ORIGIN}/__shot-messenger-home`, { waitUntil: "load" });
  await bootRealWidget(page, scheme, { centerInViewport: true });

  // Capture crisp 2.25x WebP poster on the real Home tab
  const cdp = await context.newCDPSession(page);
  const { data } = await cdp.send("Page.captureScreenshot", {
    format: "webp",
    quality: 94,
    clip: { x: 0, y: 0, width: 720, height: 640, scale: 2.25 },
  });
  writeFileSync(join(OUT_DIR, `channel-messenger-home-${scheme}.webp`), Buffer.from(data, "base64"));

  const trimStart = (Date.now() - startedAt) / 1000;
  await page.waitForTimeout(1000);

  // Switch to Help tab, then Changelog tab, then back to Home
  await clickInWidget(page, 'button[data-tab="help"]');
  await page.waitForTimeout(1100);
  await clickInWidget(page, 'button[data-tab="changelog"]');
  await page.waitForTimeout(1100);
  await clickInWidget(page, 'button[data-tab="home"]');
  await page.waitForTimeout(900);

  const videoPath = await page.video()?.path();
  await context.close();
  if (!videoPath) {
    throw new Error("Missing recorded video for channel-messenger-home");
  }
  encodeMp4(videoPath, join(OUT_DIR, `channel-messenger-home-${scheme}.mp4`), trimStart, 1080, 960);
  process.stdout.write(`captured real widget home: channel-messenger-home-${scheme}\n`);
}

async function main() {
  mkdirSync(TMP_DIR, { recursive: true });
  mkdirSync(OUT_DIR, { recursive: true });
  const only = process.argv[2];
  const browser = await chromium.launch();
  try {
    for (const scheme of ["light", "dark"] as const) {
      if (!only || only === "hero") {
        await captureHero(browser, scheme);
      }
      if (!only || only === "home") {
        await captureHome(browser, scheme);
      }
    }
  } finally {
    await browser.close();
    rmSync(TMP_DIR, { recursive: true, force: true });
  }
}

main().catch((err: unknown) => {
  process.stderr.write(`${err instanceof Error ? (err.stack ?? err.message) : String(err)}\n`);
  process.exit(1);
});
