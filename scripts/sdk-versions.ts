/*
 * Applies a release plan (scripts/sdk-release-plan.ts) to the working
 * tree before the SDKs are built and published: each released SDK's
 * manifest version, the ranges other SDKs depend on it with, and the
 * places each language reads its version from. The release never
 * commits these: versions live in the registries and mirrors.
 *
 *   node scripts/sdk-versions.ts --apply plan.json   write them
 *   node scripts/sdk-versions.ts --print plan.json   print dir=version per released SDK
 */
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

import type { Entry } from "./sdk-release-plan.ts";

const root = path.join(path.dirname(fileURLToPath(import.meta.url)), "..");
const args = process.argv.slice(2);
const mode = args[0];
const file = args[1];

if (!file || (mode !== "--apply" && mode !== "--print")) {
  process.stderr.write("usage: node scripts/sdk-versions.ts --apply|--print plan.json\n");
  process.exit(2);
}

const released = (JSON.parse(fs.readFileSync(file, "utf8")) as Entry[]).filter((e) => e.version !== null);

if (mode === "--print") {
  for (const e of released) {
    process.stdout.write(`${e.dir}=${e.version}\n`);
  }
  process.exit(0);
}

/* replace rewrites the first match of the pattern in a file, failing when there is none. */
function replace(rel: string, pattern: RegExp, value: string) {
  const full = path.join(root, rel);
  if (!fs.existsSync(full)) {
    return;
  }
  const text = fs.readFileSync(full, "utf8");
  if (!pattern.test(text)) {
    process.stderr.write(`sdk-versions: no version found in ${rel}\n`);
    process.exit(1);
  }
  fs.writeFileSync(
    full,
    text.replace(pattern, (_m, before: string, after: string) => `${before}${value}${after}`),
  );
}

const versions = new Map(released.map((e) => [e.name, e.version ?? ""]));

// Manifests: the released versions, and every range on a released SDK.
for (const dir of fs.readdirSync(path.join(root, "sdk"))) {
  const manifestFile = path.join(root, "sdk", dir, "package.json");
  if (!fs.existsSync(manifestFile)) {
    continue;
  }
  const manifest = JSON.parse(fs.readFileSync(manifestFile, "utf8"));
  manifest.version = versions.get(manifest.name) ?? manifest.version;
  for (const field of ["dependencies", "peerDependencies", "devDependencies"]) {
    for (const dep of Object.keys(manifest[field] ?? {})) {
      const v = versions.get(dep);
      if (v) {
        manifest[field][dep] = `^${v}`;
      }
    }
  }
  fs.writeFileSync(manifestFile, `${JSON.stringify(manifest, null, 2)}\n`);
}

const version = (dir: string) => released.find((e) => e.dir === dir)?.version ?? null;

const go = version("go");
if (go) {
  replace("sdk/go/client.go", /(const Version = ")[^"]+(")/, go);
}

const python = version("python");
if (python) {
  replace("sdk/python/src/dimmi/_version.py", /(__version__ = ")[^"]+(")/, python);
}

const php = version("php");
if (php) {
  replace("sdk/php/src/Client.php", /(public const VERSION = ')[^']+(')/, php);
  // Laravel requires the PHP SDK's current major, or minor while it is 0.x.
  const [major = "0", minor = "0"] = php.split(".");
  const range = major === "0" ? `^0.${minor}` : `^${major}.0`;
  replace("sdk/laravel/composer.json", /("dimmi\/dimmi-php": ")[^"]+(")/, range);
  // The path repository Laravel installs the PHP SDK from in this repository says which version it is.
  replace("sdk/laravel/composer.json", /("versions":\s*\{\s*"dimmi\/dimmi-php":\s*")[^"]+(")/, php);
}

process.stdout.write(`sdk-versions: applied ${released.length} release(s)\n`);
