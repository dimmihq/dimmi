/*
 * Writes a half-width variant next to every capture in site/public/shots
 * (inbox-light.webp gets inbox-light.1x.webp), so screens that show a
 * shot small or at 1x density stop downloading the 2x file. It also
 * records both pixel widths in site/src/shot-files.json, which the
 * components turn into srcset width descriptors.
 *
 *   node site/shots/downscale.ts
 *
 * The capture scripts run it when they finish. It skips variants newer
 * than their capture, so a rerun only redoes what changed. Chromium
 * (from Playwright) does the resizing and encoding, so it needs no
 * native image tools. Commit the variants and the JSON with the shots.
 */
import { readdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import { chromium } from "@playwright/test";

const dir = fileURLToPath(new URL("../public/shots/", import.meta.url));
const manifest = fileURLToPath(new URL("../src/shot-files.json", import.meta.url));
const QUALITY = 0.85;

/* A WebP's pixel size, read from its header (lossy, lossless or extended). */
function webpSize(file: string) {
  const b = readFileSync(file);
  const kind = b.toString("ascii", 12, 16);
  if (kind === "VP8 ") {
    return { width: b.readUInt16LE(26) & 0x3fff, height: b.readUInt16LE(28) & 0x3fff };
  }
  if (kind === "VP8L") {
    const bits = b.readUInt32LE(21);
    return { width: (bits & 0x3fff) + 1, height: ((bits >> 14) & 0x3fff) + 1 };
  }
  if (kind === "VP8X") {
    return { width: b.readUIntLE(24, 3) + 1, height: b.readUIntLE(27, 3) + 1 };
  }
  throw new Error(`${file}: not a WebP`);
}

export async function downscaleShots() {
  const sources = readdirSync(dir)
    .filter((name) => name.endsWith(".webp") && !name.endsWith(".1x.webp"))
    .sort();
  const files: Record<string, [number, number]> = {};
  const stale: { stem: string; width: number; height: number }[] = [];

  for (const name of sources) {
    const stem = name.slice(0, -".webp".length);
    const { width, height } = webpSize(`${dir}${name}`);
    const small = { width: Math.round(width / 2), height: Math.round(height / 2) };
    files[stem] = [small.width, width];
    const variant = `${dir}${stem}.1x.webp`;
    let fresh = false;
    try {
      const size = webpSize(variant);
      fresh =
        statSync(variant).mtimeMs >= statSync(`${dir}${name}`).mtimeMs &&
        size.width === small.width &&
        size.height === small.height;
    } catch {
      fresh = false;
    }
    if (!fresh) {
      stale.push({ stem, ...small });
    }
  }

  if (stale.length > 0) {
    const browser = await chromium.launch();
    const page = await browser.newPage();
    for (const shot of stale) {
      const source = readFileSync(`${dir}${shot.stem}.webp`).toString("base64");
      const encoded = await page.evaluate(
        async ({ source, width, height, quality }) => {
          const blob = await (await fetch(`data:image/webp;base64,${source}`)).blob();
          const bitmap = await createImageBitmap(blob, {
            resizeWidth: width,
            resizeHeight: height,
            resizeQuality: "high",
          });
          const canvas = new OffscreenCanvas(width, height);
          canvas.getContext("2d")?.drawImage(bitmap, 0, 0);
          const out = await canvas.convertToBlob({ type: "image/webp", quality });
          const bytes = new Uint8Array(await out.arrayBuffer());
          let binary = "";
          for (let i = 0; i < bytes.length; i += 0x8000) {
            binary += String.fromCharCode(...bytes.subarray(i, i + 0x8000));
          }
          return btoa(binary);
        },
        { source, width: shot.width, height: shot.height, quality: QUALITY },
      );
      writeFileSync(`${dir}${shot.stem}.1x.webp`, Buffer.from(encoded, "base64"));
      process.stdout.write(`saved ${shot.stem}.1x.webp (${shot.width}x${shot.height})\n`);
    }
    await browser.close();
  }

  // One entry per line: stem -> [1x width, full width] in pixels.
  const lines = Object.entries(files).map(([stem, widths]) => `  ${JSON.stringify(stem)}: [${widths.join(", ")}]`);
  writeFileSync(manifest, `{\n${lines.join(",\n")}\n}\n`);
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  await downscaleShots();
}
