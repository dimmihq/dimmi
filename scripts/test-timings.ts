/*
 * Rewrites scripts/test-timings.json, the seconds each Go test package
 * took in CI, which scripts/test-shard.ts splits the test pass by. Feed
 * it the log of a green run of main:
 *
 *   gh run view <run id> --log | node scripts/test-timings.ts
 *
 * Only the cloud pass is read, since that is the pass CI splits; when a
 * package shows more than once the slowest time is kept.
 */
import { readFileSync, writeFileSync } from "node:fs";

const line = /^go tests \(\d+\/\d+\)\t.*\bok\s+(github\.com\/\S+)\s+([0-9.]+)s/;
const timings: Record<string, number> = {};

for (const row of readFileSync(0, "utf8").split("\n")) {
  const match = line.exec(row);
  if (!match) {
    continue;
  }
  const [, importPath = "", seconds = "0"] = match;
  timings[importPath] = Math.max(timings[importPath] ?? 0, Math.round(Number(seconds)));
}

if (Object.keys(timings).length === 0) {
  throw new Error("no package timings found; pass the log of a run of main on standard input");
}

const sorted = Object.fromEntries(Object.entries(timings).sort(([a], [b]) => a.localeCompare(b)));
writeFileSync("scripts/test-timings.json", `${JSON.stringify(sorted, null, 2)}\n`);
process.stdout.write(`${Object.keys(sorted).length} packages\n`);
