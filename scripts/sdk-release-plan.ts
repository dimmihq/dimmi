/*
 * Decides which SDKs a platform release ships, and at which version.
 * sdk-release.yml runs it for each vX.Y.Z tag, comparing that tag with
 * the previous platform release:
 *
 *   never published                         → first release, at its manifest version
 *   API contract and its source unchanged   → not released
 *   only its hand-written source changed    → patch
 *   API changed additively                  → minor, for the SDKs that expose the API
 *   API changed with an approved break      → major, for the SDKs that expose the API
 *   an internal dependency is released      → at least patch, so its range follows
 *   a changeset added since the last release → at least the level it names
 *
 * The previous version of each SDK is what users have: the latest version
 * on npm, PyPI, or the dimmi-go, dimmi-php and dimmi-laravel mirrors.
 *
 *   node scripts/sdk-release-plan.ts --tag v0.2.0 [--from v0.1.1] [--out plan.json]
 */
import { spawnSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

export type Level = "none" | "patch" | "minor" | "major";
export type ApiChange = "none" | "additive" | "breaking";

export type PackageInput = {
  dir: string;
  name: string;
  /* The version in its manifest, used for a first release. */
  manifest: string;
  /* The latest published version, or null before the first release. */
  published: string | null;
  sourceChanged: boolean;
  exposesApi: boolean;
  /* Names of the SDKs it depends on. */
  dependsOn: string[];
  /* The highest level a changeset added since the last release names. */
  changeset: Level;
};

export type Entry = {
  dir: string;
  name: string;
  level: Level | "first";
  from: string | null;
  version: string | null;
  reason: string;
};

const order: Level[] = ["none", "patch", "minor", "major"];
const max = (a: Level, b: Level): Level => (order.indexOf(a) >= order.indexOf(b) ? a : b);

/* bump raises a semantic version by a level. */
export function bump(version: string, level: Level): string {
  const [major = 0, minor = 0, patch = 0] = version.split(/[.-]/).map(Number);
  switch (level) {
    case "major":
      return `${major + 1}.0.0`;
    case "minor":
      return `${major}.${minor + 1}.0`;
    case "patch":
      return `${major}.${minor}.${patch + 1}`;
    default:
      return version;
  }
}

function automatic(p: PackageInput, api: ApiChange): [Level, string] {
  if (p.exposesApi && api === "breaking") {
    return ["major", "approved breaking API change"];
  }
  if (p.exposesApi && api === "additive") {
    return ["minor", "API changed additively"];
  }
  if (p.sourceChanged) {
    return ["patch", "source changed"];
  }
  return ["none", "unchanged"];
}

/* plan decides the release of every SDK. */
export function plan(packages: PackageInput[], api: ApiChange): Entry[] {
  const levels = new Map<string, { level: Level | "first"; reason: string }>();
  for (const p of packages) {
    if (p.published === null) {
      levels.set(p.name, { level: "first", reason: "first release" });
      continue;
    }
    const [auto, why] = automatic(p, api);
    const level = max(auto, p.changeset);
    levels.set(p.name, { level, reason: level !== auto ? `changeset asks for ${p.changeset}` : why });
  }

  // A released dependency moves its dependents, so their ranges follow it.
  for (let changed = true; changed; ) {
    changed = false;
    for (const p of packages) {
      const own = levels.get(p.name);
      if (own?.level !== "none") {
        continue;
      }
      const dep = p.dependsOn.find((d) => (levels.get(d)?.level ?? "none") !== "none");
      if (dep) {
        levels.set(p.name, { level: "patch", reason: `depends on ${dep}, released` });
        changed = true;
      }
    }
  }

  return packages.map((p) => {
    const { level, reason } = levels.get(p.name) ?? { level: "none" as Level, reason: "unchanged" };
    const version = level === "first" ? p.manifest : level === "none" ? null : bump(p.published ?? p.manifest, level);
    return { dir: p.dir, name: p.name, level, from: p.published, version, reason };
  });
}

/* ------------------------------------------------------------------ git and registries */

const root = path.join(path.dirname(fileURLToPath(import.meta.url)), "..");
const exposing = new Set(["api", "go", "php", "laravel", "python"]);
const release = /^v(\d+)\.(\d+)\.(\d+)$/;

function out(cmd: string, argv: string[]): string | null {
  const res = spawnSync(cmd, argv, { cwd: root, encoding: "utf8", maxBuffer: 64 << 20 });
  return res.status === 0 ? res.stdout : null;
}

const git = (...argv: string[]) => out("git", argv) ?? "";

function newer(a: string, b: string): number {
  const [x, y] = [release.exec(a), release.exec(b)];
  for (let i = 1; i <= 3; i++) {
    const d = Number(x?.[i]) - Number(y?.[i]);
    if (d !== 0) {
      return d;
    }
  }
  return 0;
}

/* previousRelease is the newest platform release before the tag, without prereleases. */
export function previousRelease(tag: string, tags: string[]): string | null {
  return (
    tags
      .filter((t) => release.test(t) && (!release.test(tag) || newer(t, tag) < 0))
      .sort(newer)
      .at(-1) ?? null
  );
}

/* latest picks the newest X.Y.Z version from a list. */
function latest(versions: string[]): string | null {
  return (
    versions
      .filter((v) => release.test(`v${v}`))
      .sort((a, b) => newer(`v${a}`, `v${b}`))
      .at(-1) ?? null
  );
}

const mirrors = process.env.SDK_COMPAT_MIRRORS ?? "https://github.com/dimmihq";
const pypi = process.env.SDK_COMPAT_PYPI ?? "https://pypi.org";

/* publishedVersion asks where users get the SDK which version they have. */
function publishedVersion(dir: string, name: string): string | null {
  if (["go", "php", "laravel"].includes(dir)) {
    const refs = out("git", ["ls-remote", "--tags", "--refs", `${mirrors}/dimmi-${dir}`]) ?? "";
    return latest([...refs.matchAll(/refs\/tags\/v(\S+)$/gm)].map((m) => m[1] ?? ""));
  }
  if (dir === "python") {
    const res = out("curl", ["-fsSL", `${pypi}/pypi/dimmi-sdk/json`]);
    return res ? latest(Object.keys(JSON.parse(res).releases ?? {})) : null;
  }
  const res = out("npm", ["view", name, "versions", "--json"]);
  if (!res) {
    return null;
  }
  const versions: string | string[] = JSON.parse(res);
  return latest(Array.isArray(versions) ? versions : [versions]);
}

const ignoredSource = /(^|\/)(tests?\/|api-report\/|README\.md$|CHANGELOG\.md$)|\.test\.ts$/;

function changesetLevels(from: string | null, to: string): Map<string, Level> {
  const levels = new Map<string, Level>();
  const added = from
    ? git("diff", "--name-only", "--diff-filter=A", from, to, "--", "sdk/.changeset")
    : git("ls-tree", "-r", "--name-only", to, "sdk/.changeset");
  for (const file of added.split("\n").filter((f) => f.endsWith(".md") && !f.endsWith("README.md"))) {
    const front = git("show", `${to}:${file}`).split("---")[1] ?? "";
    for (const m of front.matchAll(/^\s*["']?([^"':]+)["']?\s*:\s*(patch|minor|major)\s*$/gm)) {
      const [name = "", level = "none"] = [m[1], m[2]];
      levels.set(name, max(levels.get(name) ?? "none", level as Level));
    }
  }
  return levels;
}

function apiChange(from: string | null, to: string): ApiChange {
  if (!from) {
    return "none";
  }
  const res = spawnSync("node", ["scripts/api-compat.ts", "--ref", from, "--to", to, "--json"], {
    cwd: root,
    encoding: "utf8",
    maxBuffer: 64 << 20,
  });
  const summary: { changed: boolean; approved: unknown[]; blocking: { id: string; operation: string }[] } = JSON.parse(
    res.stdout,
  );
  if (summary.blocking.length > 0) {
    throw new Error(
      `the API breaks clients since ${from} without approval:\n${summary.blocking.map((b) => `  ${b.id} ${b.operation}`).join("\n")}`,
    );
  }
  if (summary.approved.length > 0) {
    return "breaking";
  }
  return summary.changed ? "additive" : "none";
}

function inputs(from: string | null, to: string): PackageInput[] {
  const changesets = changesetLevels(from, to);
  const dirs = git("ls-tree", "-d", "--name-only", to, "sdk/")
    .split("\n")
    .map((d) => d.replace(/^sdk\//, ""))
    .filter((d) => d && git("cat-file", "-t", `${to}:sdk/${d}/package.json`).trim() === "blob");
  const manifests = dirs.map((dir) => ({ dir, json: JSON.parse(git("show", `${to}:sdk/${dir}/package.json`)) }));
  const names = new Set(manifests.map((m) => m.json.name as string));
  return manifests.map(({ dir, json }) => {
    const changed = from
      ? git("diff", "--name-only", from, to, "--", `sdk/${dir}`)
          .split("\n")
          .filter((f) => f && !ignoredSource.test(f))
      : ["new"];
    const deps = Object.keys({ ...json.dependencies, ...json.peerDependencies }).filter((d) => names.has(d));
    return {
      dir,
      name: json.name,
      manifest: json.version,
      published: publishedVersion(dir, json.name),
      sourceChanged: changed.length > 0,
      exposesApi: exposing.has(dir),
      dependsOn: dir === "laravel" ? [...deps, "@dimmi/php-sdk"] : deps,
      changeset: changesets.get(json.name) ?? "none",
    };
  });
}

function main() {
  const args = process.argv.slice(2);
  const flag = (name: string) => (args.includes(name) ? args[args.indexOf(name) + 1] : undefined);
  const tag = flag("--tag") ?? "HEAD";
  const target = tag;
  const outFile = flag("--out");

  if (tag !== "HEAD" && !release.test(tag)) {
    process.stdout.write(`sdk-release-plan: ${tag} is a prerelease or not a release tag; no SDK is released.\n`);
    if (outFile) {
      fs.writeFileSync(outFile, "[]\n");
    }
    return;
  }
  const from = flag("--from") ?? previousRelease(tag, git("tag", "--list", "v*").split("\n"));
  const api = apiChange(from, target);
  const entries = plan(inputs(from, target), api);

  process.stdout.write(`sdk-release-plan: ${from ?? "(no earlier release)"} → ${tag}, API ${api}\n\n`);
  for (const e of entries) {
    const to = e.version ? `${e.from ?? "-"} → ${e.version}` : "";
    process.stdout.write(`  ${e.name.padEnd(20)} ${e.level.padEnd(6)} ${to.padEnd(18)} ${e.reason}\n`);
  }
  if (outFile) {
    fs.writeFileSync(outFile, `${JSON.stringify(entries, null, 2)}\n`);
  }
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    main();
  } catch (err) {
    process.stderr.write(`sdk-release-plan: ${(err as Error).message}\n`);
    process.exit(1);
  }
}
