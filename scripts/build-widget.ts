/*
 * Builds the messenger widget into public/widget, which the Go binary
 * embeds and serves. The loader is a classic script with a fixed name,
 * since customers paste its address; the core and anything it loads later
 * are ES modules named after their content, so browsers keep them forever.
 *
 * Each hashed file gets gzip and brotli copies, so the server compresses
 * nothing per request, and a sha384 integrity hash. Files the previous
 * build listed stay one more build, so a visitor whose page still runs the
 * old core can fetch its lazy parts after a deploy. manifest.json records
 * all of it, and the server reads it to fill in the loader.
 *
 * The widget must stay small on every customer's page: the build fails
 * when the loader or the core grows past its budget in raw, uncompressed
 * bytes.
 */
import { createHash } from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { brotliCompressSync, constants, gzipSync } from "node:zlib";

import { build } from "vite";

interface Chunk {
  fileName: string;
  code: string;
  isEntry: boolean;
}

interface FileEntry {
  name: string;
  size: number;
  gzip: number;
  brotli: number;
  integrity: string;
}

const root = fileURLToPath(new URL("..", import.meta.url));
const out = path.join(root, "public/widget");
const assets = path.join(out, "assets");
const budgets = { loader: 2048, core: 73728 };

async function bundle(entry: string, format: "iife" | "es"): Promise<Chunk[]> {
  const result = await build({
    configFile: path.join(root, "vite.widget.config.ts"),
    root,
    logLevel: "warn",
    build: {
      write: false,
      minify: true,
      target: "es2022",
      cssTarget: ["chrome123", "edge123", "firefox120", "safari17.5"],
      modulePreload: false,
      rollupOptions: {
        input: path.join(root, entry),
        output: {
          format,
          entryFileNames: format === "iife" ? "loader.js" : "core-[hash].js",
          chunkFileNames: "[name]-[hash].js",
        },
      },
    },
  });
  const outputs = (Array.isArray(result) ? result : [result]) as { output: (Chunk & { type: string })[] }[];

  return outputs.flatMap(({ output }) => output.filter((item) => item.type === "chunk"));
}

function measure(name: string, code: Buffer): FileEntry {
  return {
    name,
    size: code.length,
    gzip: gzipSync(code, { level: 9 }).length,
    brotli: brotliCompressSync(code, { params: { [constants.BROTLI_PARAM_QUALITY]: 11 } }).length,
    integrity: `sha384-${createHash("sha384").update(code).digest("base64")}`,
  };
}

function writeAsset(name: string, code: Buffer): FileEntry {
  fs.writeFileSync(path.join(assets, name), code);
  fs.writeFileSync(path.join(assets, `${name}.gz`), gzipSync(code, { level: 9 }));
  fs.writeFileSync(
    path.join(assets, `${name}.br`),
    brotliCompressSync(code, { params: { [constants.BROTLI_PARAM_QUALITY]: 11 } }),
  );

  return measure(name, code);
}

function previousFiles(): string[] {
  try {
    const manifest = JSON.parse(fs.readFileSync(path.join(out, "manifest.json"), "utf8"));
    return manifest.files.map((file: FileEntry) => file.name);
  } catch {
    return [];
  }
}

function prune(keep: Set<string>) {
  for (const name of fs.readdirSync(assets)) {
    if (!keep.has(name.replace(/\.(gz|br)$/, ""))) {
      fs.rmSync(path.join(assets, name));
    }
  }
}

function checkBudget(name: keyof typeof budgets, size: number) {
  if (size > budgets[name]) {
    throw new Error(`widget ${name} is ${size} bytes, over its ${budgets[name]} byte budget`);
  }
}

const [loader] = await bundle("resources/widget/entries/loader.ts", "iife");
const chunks = await bundle("resources/widget/entries/core.ts", "es");
const core = chunks.find((chunk) => chunk.isEntry);
if (!loader || !core) {
  throw new Error("widget build produced no loader or no core");
}

const loaderCode = Buffer.from(loader.code);
checkBudget("loader", loaderCode.length);
checkBudget("core", Buffer.byteLength(core.code));

const previous = previousFiles();
fs.mkdirSync(assets, { recursive: true });
const files = chunks.map((chunk) => writeAsset(chunk.fileName, Buffer.from(chunk.code)));
const current = new Set(files.map((file) => file.name));
const kept = previous.filter((name) => !current.has(name) && fs.existsSync(path.join(assets, name)));
prune(new Set([...current, ...kept]));

fs.writeFileSync(path.join(out, "loader.js"), loaderCode);
const { integrity: _, ...loaderSizes } = measure("loader.js", loaderCode);
const manifest = {
  version: core.fileName.replace(/^core-|\.js$/g, ""),
  entry: core.fileName,
  loader: loaderSizes,
  files,
  previous: kept,
  budgets,
};
fs.writeFileSync(path.join(out, "manifest.json"), `${JSON.stringify(manifest, null, 2)}\n`);

for (const file of [loaderSizes, ...files]) {
  process.stdout.write(
    `widget ${file.name.padEnd(24)} ${String(file.size).padStart(6)} B  ${String(file.gzip).padStart(6)} B gz\n`,
  );
}
