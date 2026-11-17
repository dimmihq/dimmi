/*
 * Prints the Go packages one shard of a test pass runs, so CI can split
 * a slow pass across machines. Every package lands in exactly one
 * shard, and the same inputs always give the same split.
 *
 * Packages are weighed by the seconds they took in CI, kept in
 * scripts/test-timings.json (scripts/test-timings.ts rewrites it from a
 * run's log). A package with no time there yet is weighed by the size of
 * its test files instead, scaled to seconds by the packages that have
 * both; tests that talk to the database or a browser run far longer than
 * unit tests, which count for a fifth. The heaviest packages are placed
 * first, each into the lightest shard so far.
 *
 *   node scripts/test-shard.ts [-tags cloud] 2/4 ./app/... ./tests/...
 */
import { execFileSync } from "node:child_process";
import { existsSync, readFileSync, statSync } from "node:fs";
import path from "node:path";

type Package = { importPath: string; weight: number };

const unitShare = 0.2;
const baseline = 1000;
const timingsFile = "scripts/test-timings.json";

function parseArgs(argv: string[]): { tags: string[]; index: number; total: number; patterns: string[] } {
  const args = [...argv];
  const tags: string[] = [];

  if (args[0] === "-tags") {
    tags.push("-tags", args[1] ?? "");
    args.splice(0, 2);
  }

  const match = /^(\d+)\/(\d+)$/.exec(args[0] ?? "");
  if (!match) {
    throw new Error("usage: node scripts/test-shard.ts [-tags t] <index>/<total> <packages...>");
  }

  const index = Number(match[1]);
  const total = Number(match[2]);
  if (index < 1 || index > total) {
    throw new Error(`shard ${index} is outside 1..${total}`);
  }

  return { tags, index, total, patterns: args.slice(1) };
}

function listPackages(tags: string[], patterns: string[]): Package[] {
  const format = '{{.ImportPath}}\t{{.Dir}}\t{{join .TestGoFiles ","}},{{join .XTestGoFiles ","}}';
  const output = execFileSync("go", ["list", ...tags, "-f", format, ...patterns], { encoding: "utf8" });

  return output
    .trim()
    .split("\n")
    .filter(Boolean)
    .map((line) => {
      const [importPath = "", dir = "", files = ""] = line.split("\t");
      const bytes = files
        .split(",")
        .filter(Boolean)
        .reduce((sum, file) => sum + statSync(path.join(dir, file)).size, 0);
      const share = importPath.includes("/tests/") ? 1 : unitShare;

      return { importPath, weight: baseline + bytes * share };
    });
}

/*
 * Weighs each package in seconds: its recorded time, or its size scaled
 * by the seconds per unit of size of the packages that have a time.
 */
function inSeconds(packages: Package[]): Package[] {
  const timings: Record<string, number> = existsSync(timingsFile) ? JSON.parse(readFileSync(timingsFile, "utf8")) : {};
  const timed = packages.filter((pkg) => timings[pkg.importPath] !== undefined);
  if (timed.length === 0) {
    return packages;
  }

  const seconds = timed.reduce((sum, pkg) => sum + (timings[pkg.importPath] ?? 0), 0);
  const size = timed.reduce((sum, pkg) => sum + pkg.weight, 0);
  const perUnit = seconds / size;

  return packages.map((pkg) => ({
    importPath: pkg.importPath,
    weight: Math.max(timings[pkg.importPath] ?? pkg.weight * perUnit, 1),
  }));
}

function split(packages: Package[], total: number): string[][] {
  const shards = Array.from({ length: total }, () => ({ weight: 0, packages: [] as string[] }));
  const ordered = [...packages].sort((a, b) => b.weight - a.weight || a.importPath.localeCompare(b.importPath));

  for (const pkg of ordered) {
    const lightest = shards.reduce((min, shard) => (shard.weight < min.weight ? shard : min));
    lightest.weight += pkg.weight;
    lightest.packages.push(pkg.importPath);
  }

  return shards.map((shard) => shard.packages.sort());
}

const { tags, index, total, patterns } = parseArgs(process.argv.slice(2));
const shard = split(inSeconds(listPackages(tags, patterns)), total)[index - 1] ?? [];

if (shard.length === 0) {
  throw new Error(`shard ${index}/${total} has no packages`);
}

process.stdout.write(`${shard.join(" ")}\n`);
