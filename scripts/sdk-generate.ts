/*
 * Generates every SDK's generated layer from the committed API contract,
 * tests/contracts/api_v1.openapi.json. The generated code is never
 * committed (.gitignore): this is how a clone, CI and the release get it.
 *
 *   node scripts/sdk-generate.ts                 generate every SDK
 *   node scripts/sdk-generate.ts php python      only those SDKs
 *   node scripts/sdk-generate.ts --if-missing    only SDKs whose output is missing
 *   node scripts/sdk-generate.ts --check         generate, then build and unit test every SDK
 *
 * Each SDK owns its generator; this file only lists them. A new SDK adds
 * an entry with the command that generates it, what it writes and how
 * it is built and tested.
 */
import { execFileSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.join(path.dirname(fileURLToPath(import.meta.url)), "..");
const spec = path.join(root, "tests/contracts/api_v1.openapi.json");

type Step = string[];
type Sdk = {
  /* The SDK's directory under sdk/. */
  dir: string;
  /* The generator, run from the SDK's directory, given the spec's path last; none for SDKs without one. */
  generate?: Step;
  /* What the generator writes, relative to the SDK's directory. */
  outputs: string[];
  /* How --check builds and tests it, from the SDK's directory. */
  verify: Step[];
};

const composer: Step[] = [
  ["composer", "install", "--no-interaction", "--no-progress", "--quiet"],
  ["vendor/bin/phpunit"],
];

const sdks: Record<string, Sdk> = {
  api: {
    dir: "sdk/api",
    generate: ["node", "scripts/generate.ts"],
    outputs: ["src/schema.ts", "src/paging.ts"],
    // Builds and tests every TypeScript package; they share the root's toolchain.
    verify: [
      ["pnpm", "--dir", "../..", "exec", "tsc", "--noEmit", "-p", "tsconfig.sdk.json"],
      ["node", "../../scripts/build-sdk.ts"],
      ["pnpm", "--dir", "../..", "exec", "vitest", "run", "--project", "sdk"],
    ],
  },
  go: {
    dir: "sdk/go",
    generate: ["sh", "scripts/generate.sh"],
    outputs: ["api/api.gen.go", "api/openapi30.json"],
    verify: [
      ["go", "vet", "./..."],
      ["go", "test", "./..."],
    ],
  },
  php: { dir: "sdk/php", generate: ["php", "bin/generate"], outputs: ["src/Generated"], verify: composer },
  // Laravel's API methods come from the PHP SDK, installed from ../php.
  laravel: { dir: "sdk/laravel", outputs: [], verify: composer },
  python: {
    dir: "sdk/python",
    generate: ["bash", "scripts/generate.sh"],
    outputs: ["src/dimmi/_generated"],
    verify: [["uv", "run", "--quiet", "--no-project", "--with-editable", ".[test]", "pytest", "-q"]],
  },
};

const args = process.argv.slice(2);
const check = args.includes("--check");
const ifMissing = args.includes("--if-missing");
const wanted = args.filter((a) => !a.startsWith("--"));

const failed: string[] = [];
const skipped: string[] = [];

function step(name: string, sdk: Sdk, cmd: Step, extra: string[] = []): boolean {
  const [bin = "", ...rest] = cmd;
  try {
    execFileSync(bin, [...rest, ...extra], { cwd: path.join(root, sdk.dir), stdio: "inherit" });
    return true;
  } catch {
    failed.push(`${name}: ${cmd.join(" ")} failed in ${sdk.dir}`);
    return false;
  }
}

for (const [name, sdk] of Object.entries(sdks)) {
  if (wanted.length > 0 && !wanted.includes(name)) {
    continue;
  }
  const dir = path.join(root, sdk.dir);
  if (!fs.existsSync(dir)) {
    skipped.push(`${name}: no ${sdk.dir}`);
    continue;
  }
  const missing = sdk.outputs.some((o) => !fs.existsSync(path.join(dir, o)));
  if (sdk.generate && (!ifMissing || missing)) {
    process.stdout.write(`sdk-generate: ${name}\n`);
    if (!step(name, sdk, sdk.generate, [spec])) {
      continue;
    }
  }
  if (check) {
    process.stdout.write(`sdk-check: ${name}\n`);
    sdk.verify.every((cmd) => step(name, sdk, cmd));
  }
}

for (const line of skipped) {
  process.stdout.write(`sdk-generate: skipped ${line}\n`);
}

if (failed.length > 0) {
  process.stderr.write(`
${check ? "sdk-check" : "sdk-generate"}: ${failed.length} step(s) failed:

  ${failed.join("\n  ")}

The SDKs are generated from tests/contracts/api_v1.openapi.json and their
hand-written code must build and pass its tests against it. Fix the SDK
or the API change; never break either. See AGENTS.md.
`);
  process.exit(1);
}

process.stdout.write(
  check ? "sdk-check: every SDK generates, builds and passes its tests.\n" : "sdk-generate: done.\n",
);
