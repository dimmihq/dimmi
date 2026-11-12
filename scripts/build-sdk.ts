/*
 * Builds every package under sdk/ in dependency order. The framework
 * packages ship ES modules and CommonJS side by side, bundled by vite with
 * every import left external, and their declarations come from tsc. The
 * server and Next.js packages compile with tsc alone, as ES modules. Pass
 * package names to build only those: node scripts/build-sdk.ts browser react.
 */
import { execFileSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { gzipSync } from "node:zlib";

import { build } from "vite";

const root = path.join(path.dirname(fileURLToPath(import.meta.url)), "..");
const order = ["server", "browser", "react", "vue", "angular", "svelte", "api", "nextjs"];
const bundled: Record<string, { entry: string; banner?: string }> = {
  browser: { entry: "src/index.ts" },
  react: { entry: "src/index.ts", banner: '"use client";' },
  vue: { entry: "src/index.ts" },
  angular: { entry: "src/index.ts" },
  svelte: { entry: "src/index.ts" },
  api: { entry: "src/index.ts" },
};

const wanted = process.argv.slice(2);

/*
 * imports fails the build when a bundle imports a package its
 * package.json does not list, such as a compiler helper runtime.
 */
function imports(dir: string): void {
  const pkg = JSON.parse(fs.readFileSync(path.join(dir, "package.json"), "utf8"));
  const allowed = new Set([...Object.keys(pkg.dependencies ?? {}), ...Object.keys(pkg.peerDependencies ?? {})]);
  for (const file of ["index.js", "index.cjs"]) {
    const code = fs.readFileSync(path.join(dir, "dist", file), "utf8");
    for (const [, name] of code.matchAll(/(?:from\s*|require\(|import\()\s*"([^".][^"]*)"/g)) {
      const base = name.startsWith("@") ? name.split("/").slice(0, 2).join("/") : name.split("/")[0];
      if (!allowed.has(base)) {
        throw new Error(`${pkg.name}: dist/${file} imports ${name}, which package.json does not list`);
      }
    }
  }
}
const tsc = path.join(root, "node_modules", ".bin", "tsc");

for (const name of order.filter((n) => wanted.length === 0 || wanted.includes(n))) {
  const dir = path.join(root, "sdk", name);
  fs.rmSync(path.join(dir, "dist"), { recursive: true, force: true });
  const spec = bundled[name];
  if (spec) {
    await build({
      configFile: false,
      root: dir,
      logLevel: "warn",
      build: {
        outDir: "dist",
        emptyOutDir: false,
        minify: false,
        sourcemap: true,
        target: "es2022",
        lib: {
          entry: path.join(dir, spec.entry),
          formats: ["es", "cjs"],
          fileName: (format) => (format === "es" ? "index.js" : "index.cjs"),
        },
        rollupOptions: {
          external: (id) => !id.startsWith(".") && !path.isAbsolute(id),
          output: { banner: spec.banner },
        },
      },
    });
  }
  execFileSync(tsc, ["-p", path.join(dir, "tsconfig.build.json")], { stdio: "inherit" });
  if (spec) {
    imports(dir);
    fs.copyFileSync(path.join(dir, "dist", "index.d.ts"), path.join(dir, "dist", "index.d.cts"));
    const code = fs.readFileSync(path.join(dir, "dist", "index.js"));
    process.stdout.write(`@dimmi/${name}: ${code.length} B, ${gzipSync(code).length} B gzipped\n`);
  } else {
    process.stdout.write(`@dimmi/${name}: built\n`);
  }
}
