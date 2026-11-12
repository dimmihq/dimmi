/*
 * Copies the versions changesets wrote into the version records of the
 * SDKs that do not publish to npm into the places each language reads
 * them from. Runs after changeset version (pnpm sdk:version).
 *
 *   node scripts/sdk-versions.ts           write them
 *   node scripts/sdk-versions.ts --print   print name=version per SDK, for the release
 */
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.join(path.dirname(fileURLToPath(import.meta.url)), "..");

function version(sdk: string): string | null {
  const file = path.join(root, "sdk", sdk, "package.json");
  return fs.existsSync(file) ? (JSON.parse(fs.readFileSync(file, "utf8")).version as string) : null;
}

/* replace rewrites the first match of the pattern in a file, failing when there is none. */
function replace(file: string, pattern: RegExp, value: string) {
  const full = path.join(root, file);
  if (!fs.existsSync(full)) {
    return;
  }
  const text = fs.readFileSync(full, "utf8");
  if (!pattern.test(text)) {
    process.stderr.write(`sdk-versions: no version found in ${file}\n`);
    process.exit(1);
  }
  fs.writeFileSync(
    full,
    text.replace(pattern, (_m, before: string, after: string) => `${before}${value}${after}`),
  );
}

if (process.argv.includes("--print")) {
  for (const sdk of ["go", "php", "laravel", "python"]) {
    const v = version(sdk);
    if (v) {
      process.stdout.write(`${sdk}=${v}\n`);
    }
  }
  process.exit(0);
}

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
}

process.stdout.write("sdk-versions: synced\n");
