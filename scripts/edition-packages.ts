/*
 * Prints the Go packages that behave differently in the self-hosted
 * build, so CI can test the self-hosted edition on those alone while
 * every package is tested in the cloud build.
 *
 * A package counts when the cloud tag changes which of its files are
 * built, or when its tests branch on the edition the binary was built
 * as. Everything else compiles to the same code in both editions.
 *
 *   node scripts/edition-packages.ts
 */
import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import path from "node:path";

const editionNames = /\b(BuiltEdition|EditionCloud|EditionSelfHost)\b/;

type Listed = { importPath: string; dir: string; files: string[]; tests: string[] };

function list(tags: string[]): Map<string, Listed> {
  const format =
    '{{.ImportPath}}\t{{.Dir}}\t{{join .GoFiles ","}}\t{{join .TestGoFiles ","}},{{join .XTestGoFiles ","}}';
  const output = execFileSync("go", ["list", ...tags, "-f", format, "./..."], { encoding: "utf8" });
  const packages = new Map<string, Listed>();

  for (const line of output.trim().split("\n").filter(Boolean)) {
    const [importPath = "", dir = "", files = "", tests = ""] = line.split("\t");
    packages.set(importPath, {
      importPath,
      dir,
      files: files.split(",").filter(Boolean).sort(),
      tests: tests.split(",").filter(Boolean).sort(),
    });
  }

  return packages;
}

const selfHost = list([]);
const cloud = list(["-tags", "cloud"]);
const found = new Set<string>();

for (const [importPath, pkg] of selfHost) {
  const other = cloud.get(importPath);
  const sameFiles =
    other !== undefined &&
    other.files.join(",") === pkg.files.join(",") &&
    other.tests.join(",") === pkg.tests.join(",");

  if (!sameFiles) {
    found.add(importPath);
    continue;
  }

  if (pkg.tests.some((file) => editionNames.test(readFileSync(path.join(pkg.dir, file), "utf8")))) {
    found.add(importPath);
  }
}

if (found.size === 0) {
  throw new Error("no package differs between the editions");
}

process.stdout.write(`${[...found].sort().join(" ")}\n`);
