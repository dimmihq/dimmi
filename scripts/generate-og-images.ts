/*
 * Generates the GitHub repository social preview (.github/assets/social-preview.png)
 * and one 1200x630 Open Graph PNG per marketing route in site/public/og/.
 *
 *   node scripts/generate-og-images.ts
 *   node scripts/generate-og-images.ts --only home,pricing
 *   node scripts/generate-og-images.ts --social-only
 */
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { basename, join } from "node:path";
import { fileURLToPath } from "node:url";

import { type Browser, chromium } from "@playwright/test";

const ROOT = fileURLToPath(new URL("..", import.meta.url));
const ROUTES_FILE = join(ROOT, "site/src/routes.ts");
const OG_OUT_DIR = join(ROOT, "site/public/og");
const SOCIAL_PREVIEW_OUT = join(ROOT, ".github/assets/social-preview.png");
const LOGO_SVG_PATH = join(ROOT, ".github/assets/dimmi-logo-dark.svg");
const INTER_FONT_PATH = join(ROOT, "node_modules/@fontsource-variable/inter/files/inter-latin-opsz-normal.woff2");
const MONO_FONT_PATH = join(
  ROOT,
  "node_modules/@fontsource-variable/jetbrains-mono/files/jetbrains-mono-latin-wght-normal.woff2",
);

const CONCURRENCY = 4;

type RouteMeta = {
  path: string;
  title: string;
  description: string;
  ogImage?: string;
};

type BrandAssets = {
  logoSvg: string;
  fontCss: string;
};

export function routePathToOgSlug(routePath: string): string {
  const clean = routePath.replace(/^\/+|\/+$/g, "");
  return clean === "" ? "home" : clean.replace(/\//g, "-");
}

function ogOutputFile(route: RouteMeta): string {
  if (route.ogImage?.startsWith("/og/")) {
    return join(OG_OUT_DIR, basename(route.ogImage));
  }
  return join(OG_OUT_DIR, `${routePathToOgSlug(route.path)}.png`);
}

function readField(block: string, field: string): string | undefined {
  const re = new RegExp(`\\b${field}\\s*:\\s*(?:"((?:[^"\\\\]|\\\\.)*)"|'((?:[^'\\\\]|\\\\.)*)')`);
  const match = re.exec(block);
  if (!match) {
    return undefined;
  }
  const raw = match[1] ?? match[2] ?? "";
  return raw.replace(/\\(["'\\])/g, "$1");
}

function extractRouteBlocks(source: string): string[] {
  const start = source.indexOf("export const routes");
  if (start < 0) {
    return [];
  }
  const arrayStart = source.indexOf("[", start);
  const arrayEnd = source.indexOf("];", arrayStart);
  if (arrayStart < 0 || arrayEnd < 0) {
    return [];
  }
  const body = source.slice(arrayStart + 1, arrayEnd);
  return body.match(/\{[^{}]*\}/g) ?? [];
}

export function extractRoutes(filePath = ROUTES_FILE): RouteMeta[] {
  const source = readFileSync(filePath, "utf8");
  const blocks = extractRouteBlocks(source);
  if (blocks.length === 0) {
    throw new Error(`Could not find routes array in ${filePath}`);
  }
  const routes: RouteMeta[] = [];
  for (const block of blocks) {
    const path = readField(block, "path");
    const title = readField(block, "title");
    const description = readField(block, "description");
    if (!path || !title || !description) {
      continue;
    }
    routes.push({
      path,
      title,
      description,
      ogImage: readField(block, "ogImage"),
    });
  }
  return routes;
}

function escapeHtml(text: string): string {
  return text
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

function formatCardTitle(route: RouteMeta): string {
  const withoutBrand = route.title.replace(/\s*\|\s*dimmi$/i, "").trim();
  if (route.path === "/") {
    const stripped = withoutBrand.replace(/^dimmi:\s*/i, "");
    return stripped.charAt(0).toUpperCase() + stripped.slice(1);
  }
  return withoutBrand;
}

/**
 * Keeps hyphenated words such as "in-app" whole, so a title never
 * breaks after its hyphen.
 */
function titleHtml(title: string): string {
  return title
    .split(" ")
    .map((word) => (word.includes("-") ? `<span class="whole">${escapeHtml(word)}</span>` : escapeHtml(word)))
    .join(" ");
}

/**
 * Shrinks a card's title, down to a floor, until it fits on one line.
 * A title still too long at the floor wraps, balanced.
 */
const FIT_TITLE_SCRIPT = `(() => {
  const title = document.querySelector(".title");
  if (!title) return;
  const floor = 40;
  const start = parseFloat(getComputedStyle(title).fontSize);
  let size = start;
  const lines = () => title.getBoundingClientRect().height / (size * 1.12);
  while (lines() > 1.5 && size > floor) {
    size -= 1;
    title.style.fontSize = size + "px";
  }
  if (lines() > 1.5) {
    size = start;
    title.style.fontSize = start + "px";
  }
})()`;

function titleFontSize(title: string): number {
  if (title.length > 76) {
    return 42;
  }
  if (title.length > 52) {
    return 48;
  }
  return 54;
}

const CHANNEL_EYEBROWS: Record<string, string> = {
  api: "CHANNEL · API & SDKS",
  email: "CHANNEL · EMAIL",
  "facebook-messenger": "CHANNEL · FACEBOOK MESSENGER",
  instagram: "CHANNEL · INSTAGRAM",
  messenger: "CHANNEL · WEB MESSENGER",
  slack: "CHANNEL · SLACK CONNECT",
  whatsapp: "CHANNEL · WHATSAPP",
};

function formatEyebrow(routePath: string): string {
  if (routePath === "/") {
    return "OPEN-SOURCE SUPPORT PLATFORM";
  }
  if (routePath === "/pricing") {
    return "PRICING";
  }
  if (routePath === "/self-host") {
    return "SELF-HOSTING";
  }
  if (routePath === "/channels") {
    return "CHANNELS";
  }
  if (routePath.startsWith("/channels/")) {
    const sub = routePath.slice("/channels/".length);
    return CHANNEL_EYEBROWS[sub] ?? `CHANNEL · ${sub.replace(/-/g, " ").toUpperCase()}`;
  }
  if (routePath.startsWith("/vs/")) {
    const sub = routePath.slice("/vs/".length);
    return `COMPARE · ${sub.replace(/-/g, " ").toUpperCase()}`;
  }
  if (routePath.startsWith("/blog/")) {
    return "BLOG";
  }
  if (routePath === "/terms" || routePath === "/privacy") {
    return "LEGAL";
  }
  return "PRODUCT";
}

function formatPathBadge(routePath: string): string {
  return routePath === "/" ? "trydimmi.com" : `trydimmi.com${routePath}`;
}

function loadBrandAssets(): BrandAssets {
  const logoSvg = readFileSync(LOGO_SVG_PATH, "utf8").trim();
  const interData = readFileSync(INTER_FONT_PATH).toString("base64");
  const monoData = readFileSync(MONO_FONT_PATH).toString("base64");
  const fontCss = `
    @font-face {
      font-family: "Inter Variable";
      font-style: normal;
      font-weight: 100 900;
      src: url("data:font/woff2;base64,${interData}") format("woff2");
    }
    @font-face {
      font-family: "JetBrains Mono Variable";
      font-style: normal;
      font-weight: 100 800;
      src: url("data:font/woff2;base64,${monoData}") format("woff2");
    }
  `;
  return { logoSvg, fontCss };
}

const BASE_SURFACE_CSS = `
  * {
    box-sizing: border-box;
    margin: 0;
    padding: 0;
  }
  html, body {
    width: 100%;
    height: 100%;
    overflow: hidden;
    -webkit-font-smoothing: antialiased;
    text-rendering: geometricPrecision;
    font-family: "Inter Variable", system-ui, sans-serif;
    background-color: #022c22;
    color: #ffffff;
  }
  .canvas {
    position: relative;
    width: 100%;
    height: 100%;
    background-color: #022c22;
    background-image:
      radial-gradient(64% 88% at 96% 4%, rgba(16, 185, 129, 0.34) 0%, rgba(16, 185, 129, 0.12) 38%, transparent 72%),
      radial-gradient(50% 62% at 6% 96%, rgba(5, 150, 105, 0.22) 0%, transparent 66%),
      radial-gradient(rgba(209, 250, 229, 0.085) 1.15px, transparent 1.15px);
    background-size: 100% 100%, 100% 100%, 22px 22px;
    padding: 28px;
    display: flex;
  }
  .frame {
    width: 100%;
    height: 100%;
    border-radius: 20px;
    border: 1px solid rgba(167, 243, 208, 0.16);
    background: linear-gradient(160deg, rgba(6, 78, 59, 0.16) 0%, rgba(2, 44, 34, 0.04) 55%, rgba(2, 44, 34, 0.24) 100%);
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.06);
  }
`;

function buildSocialPreviewHtml(assets: BrandAssets): string {
  return `<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <style>
      ${assets.fontCss}
      ${BASE_SURFACE_CSS}
      .frame {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        text-align: center;
        padding: 72px 84px;
        gap: 34px;
      }
      .logo svg {
        height: 76px;
        width: auto;
        display: block;
      }
      .headline {
        font-size: 62px;
        font-weight: 700;
        line-height: 1.08;
        letter-spacing: -0.032em;
        color: #f8fafc;
        max-width: 1040px;
      }
      .meta-pill {
        display: inline-flex;
        align-items: center;
        gap: 18px;
        padding: 15px 34px;
        border-radius: 999px;
        background: rgba(3, 47, 36, 0.72);
        border: 1px solid rgba(110, 231, 183, 0.28);
        font-family: "JetBrains Mono Variable", ui-monospace, monospace;
        font-size: 22px;
        font-weight: 500;
        letter-spacing: -0.01em;
        color: #d1fae5;
      }
      .sep {
        color: #34d399;
        font-weight: 700;
      }
    </style>
  </head>
  <body>
    <div class="canvas">
      <div class="frame">
        <div class="logo">${assets.logoSvg}</div>
        <div class="headline">Open-source Intercom alternative</div>
        <div class="meta-pill">
          <span>MIT</span>
          <span class="sep">&middot;</span>
          <span>Free seats</span>
          <span class="sep">&middot;</span>
          <span>Bring your own AI key</span>
        </div>
      </div>
    </div>
  </body>
</html>`;
}

function buildPageCardHtml(route: RouteMeta, assets: BrandAssets): string {
  const cardTitle = formatCardTitle(route);
  const size = titleFontSize(cardTitle);
  const eyebrow = formatEyebrow(route.path);
  const pathBadge = formatPathBadge(route.path);

  return `<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <style>
      ${assets.fontCss}
      ${BASE_SURFACE_CSS}
      .frame {
        display: flex;
        flex-direction: column;
        justify-content: space-between;
        padding: 52px 60px 46px;
      }
      .top-bar {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 24px;
      }
      .logo svg {
        height: 44px;
        width: auto;
        display: block;
      }
      .path-pill {
        display: inline-flex;
        align-items: center;
        padding: 9px 18px;
        border-radius: 999px;
        background: rgba(6, 78, 59, 0.56);
        border: 1px solid rgba(110, 231, 183, 0.24);
        font-family: "JetBrains Mono Variable", ui-monospace, monospace;
        font-size: 16px;
        font-weight: 500;
        color: #a7f3d0;
        white-space: nowrap;
      }
      .body {
        display: flex;
        flex-direction: column;
        gap: 20px;
        max-width: 980px;
      }
      .eyebrow {
        font-family: "JetBrains Mono Variable", ui-monospace, monospace;
        font-size: 15px;
        font-weight: 600;
        letter-spacing: 0.11em;
        text-transform: uppercase;
        color: #34d399;
      }
      .title {
        font-size: ${size}px;
        font-weight: 700;
        line-height: 1.12;
        letter-spacing: -0.032em;
        color: #ffffff;
        text-wrap: balance;
      }
      .title .whole {
        white-space: nowrap;
      }
      .description {
        font-size: 24px;
        font-weight: 400;
        line-height: 1.46;
        letter-spacing: -0.01em;
        color: rgba(236, 253, 245, 0.8);
        max-width: 940px;
        display: -webkit-box;
        -webkit-line-clamp: 3;
        -webkit-box-orient: vertical;
        overflow: hidden;
      }
      .footer {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding-top: 24px;
        border-top: 1px solid rgba(167, 243, 208, 0.14);
      }
      .chips {
        display: flex;
        align-items: center;
        gap: 12px;
      }
      .chip {
        display: inline-flex;
        align-items: center;
        padding: 7px 15px;
        border-radius: 999px;
        background: rgba(4, 47, 36, 0.72);
        border: 1px solid rgba(110, 231, 183, 0.22);
        font-family: "JetBrains Mono Variable", ui-monospace, monospace;
        font-size: 13px;
        font-weight: 500;
        letter-spacing: 0.06em;
        text-transform: uppercase;
        color: #a7f3d0;
      }
      .domain {
        font-family: "JetBrains Mono Variable", ui-monospace, monospace;
        font-size: 15px;
        font-weight: 500;
        color: rgba(167, 243, 208, 0.72);
      }
    </style>
  </head>
  <body>
    <div class="canvas">
      <div class="frame">
        <div class="top-bar">
          <div class="logo">${assets.logoSvg}</div>
          <div class="path-pill">${escapeHtml(pathBadge)}</div>
        </div>
        <div class="body">
          <div class="eyebrow">${escapeHtml(eyebrow)}</div>
          <div class="title">${titleHtml(cardTitle)}</div>
          <div class="description">${escapeHtml(route.description)}</div>
        </div>
        <div class="footer">
          <div class="chips">
            <span class="chip">MIT Open Source</span>
            <span class="chip">Unlimited Free Seats</span>
            <span class="chip">Cloud or Self-Hosted</span>
          </div>
          <div class="domain">trydimmi.com</div>
        </div>
      </div>
    </div>
  </body>
</html>`;
}

async function launchBrowser(): Promise<Browser> {
  try {
    return await chromium.launch();
  } catch (err) {
    if (existsSync("/usr/bin/google-chrome")) {
      return await chromium.launch({ executablePath: "/usr/bin/google-chrome" });
    }
    throw err;
  }
}

async function renderPng(
  browser: Browser,
  html: string,
  width: number,
  height: number,
  outPath: string,
): Promise<number> {
  const context = await browser.newContext({
    viewport: { width, height },
    deviceScaleFactor: 1,
  });
  const page = await context.newPage();
  await page.setContent(html, { waitUntil: "load" });
  await page.evaluate("document.fonts.ready");
  await page.evaluate(FIT_TITLE_SCRIPT);
  const buffer = await page.screenshot({ type: "png" });
  writeFileSync(outPath, buffer);
  await context.close();
  return buffer.byteLength;
}

type CliOptions = {
  only: Set<string> | null;
  socialOnly: boolean;
};

function parseCliArgs(argv: string[]): CliOptions {
  const socialOnly = argv.includes("--social-only");
  const onlyIdx = argv.indexOf("--only");
  const rawOnly = onlyIdx >= 0 ? argv[onlyIdx + 1] : undefined;
  const only = rawOnly
    ? new Set(
        rawOnly
          .split(",")
          .map((s) => s.trim())
          .filter(Boolean),
      )
    : null;
  return { only, socialOnly };
}

type RenderJob = {
  label: string;
  html: string;
  width: number;
  height: number;
  outPath: string;
};

async function runJobs(browser: Browser, jobs: RenderJob[]): Promise<void> {
  let cursor = 0;
  async function worker(): Promise<void> {
    while (cursor < jobs.length) {
      const idx = cursor;
      cursor += 1;
      const job = jobs[idx];
      if (!job) {
        return;
      }
      const bytes = await renderPng(browser, job.html, job.width, job.height, job.outPath);
      const kb = (bytes / 1024).toFixed(1);
      process.stdout.write(`rendered ${job.label} (${job.width}x${job.height}, ${kb} KB)\n`);
    }
  }
  await Promise.all(Array.from({ length: Math.min(CONCURRENCY, jobs.length) }, () => worker()));
}

async function main(): Promise<void> {
  const opts = parseCliArgs(process.argv.slice(2));
  const assets = loadBrandAssets();
  mkdirSync(OG_OUT_DIR, { recursive: true });

  const jobs: RenderJob[] = [];

  if (opts.socialOnly || !opts.only || opts.only.has("social-preview")) {
    jobs.push({
      label: ".github/assets/social-preview.png",
      html: buildSocialPreviewHtml(assets),
      width: 1280,
      height: 640,
      outPath: SOCIAL_PREVIEW_OUT,
    });
  }

  if (!opts.socialOnly) {
    const routes = extractRoutes();
    for (const route of routes) {
      const slug = routePathToOgSlug(route.path);
      if (opts.only && !opts.only.has(slug)) {
        continue;
      }
      jobs.push({
        label: `site/public/og/${basename(ogOutputFile(route))}`,
        html: buildPageCardHtml(route, assets),
        width: 1200,
        height: 630,
        outPath: ogOutputFile(route),
      });
    }
  }

  const browser = await launchBrowser();
  try {
    await runJobs(browser, jobs);
  } finally {
    await browser.close();
  }
  process.stdout.write(`done: ${jobs.length} image(s)\n`);
}

if (process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1]) {
  main().catch((err: unknown) => {
    process.stderr.write(`${err instanceof Error ? (err.stack ?? err.message) : String(err)}\n`);
    process.exit(1);
  });
}
