/*
 * Fails when an SDK's public surface breaks, compared with its last
 * published release, without an approved major version. Each language
 * uses its own standard tool:
 *
 *   TypeScript  @microsoft/api-extractor reports of the published npm package
 *               and of dist/, plus @arethetypeswrong/cli on the packed package.
 *               Small reports are committed (sdk/<pkg>/api-report) for review.
 *               @dimmi/api is all generated types, which api-compat checks.
 *   Go          apidiff against the latest tag of the dimmihq/dimmi-go mirror
 *   PHP         roave/backward-compatibility-check against the latest tag of
 *               the dimmihq/dimmi-php and dimmihq/dimmi-laravel mirrors
 *   Python      griffe check against the latest wheel on PyPI
 *
 * Before an SDK's first release there is nothing to compare, and it is
 * skipped. A break passes only when a pending changeset
 * (sdk/.changeset/*.md, added since the last platform release) or the
 * release plan (--plan plan.json) bumps that SDK's major version.
 *
 *   node scripts/sdk-compat.ts               check every SDK
 *   node scripts/sdk-compat.ts ts python     only those languages
 *   node scripts/sdk-compat.ts --update      record the TypeScript reports
 *
 * SDK_COMPAT_MIRRORS (default https://github.com/dimmihq) and
 * SDK_COMPAT_PYPI (default https://pypi.org) point at other sources.
 * The TypeScript checks read dist/, so build first (pnpm sdk:build).
 */
import { execFileSync, spawnSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const API_EXTRACTOR = "@microsoft/api-extractor@7.60.1";
const ATTW = "@arethetypeswrong/cli@0.18.5";
const APIDIFF = "golang.org/x/exp/cmd/apidiff@v0.0.0-20261109185158-a52e3aef5920";
const ROAVE = "roave/backward-compatibility-check:^8.14";
const GRIFFE = "griffe==2.4.1";

const root = path.join(path.dirname(fileURLToPath(import.meta.url)), "..");
const args = process.argv.slice(2);
const update = args.includes("--update");
const wanted = args.filter((a, i) => !a.startsWith("--") && args[i - 1] !== "--plan");
const work = fs.mkdtempSync(path.join(os.tmpdir(), "dimmi-sdk-compat-"));
const mirrors = process.env.SDK_COMPAT_MIRRORS ?? "https://github.com/dimmihq";
const pypi = process.env.SDK_COMPAT_PYPI ?? "https://pypi.org";

/* Packages whose report would be their generated types; api-compat checks those. */
const noReport = new Set(["@dimmi/api"]);

/* Changeset package names of the SDKs that do not publish to npm. */
const changesetName: Record<string, string> = {
  go: "@dimmi/go-sdk",
  php: "@dimmi/php-sdk",
  laravel: "@dimmi/laravel-sdk",
  python: "@dimmi/python-sdk",
};

const failures: string[] = [];
const notes: string[] = [];

function run(cmd: string, argv: string[], cwd = root): { ok: boolean; out: string; stdout: string } {
  const res = spawnSync(cmd, argv, { cwd, encoding: "utf8", maxBuffer: 64 << 20 });
  return { ok: res.status === 0, out: `${res.stdout ?? ""}${res.stderr ?? ""}`, stdout: res.stdout ?? "" };
}

/*
 * majorBumps lists the SDKs a break is approved for: those a release plan
 * (--plan plan.json) raises a major version, or, outside a release, those
 * a changeset added since the last platform release bumps a major version.
 */
function majorBumps(): Set<string> {
  const planFile = args.includes("--plan") ? args[args.indexOf("--plan") + 1] : undefined;
  if (planFile) {
    const entries: { name: string; level: string }[] = JSON.parse(fs.readFileSync(planFile, "utf8"));
    return new Set(entries.filter((e) => e.level === "major").map((e) => e.name));
  }
  const since =
    run("git", ["tag", "--list", "v*.*.*", "--sort=-v:refname"])
      .stdout.split("\n")
      .find((t) => /^v\d+\.\d+\.\d+$/.test(t)) ?? null;
  const added = since
    ? run("git", ["diff", "--name-only", "--diff-filter=A", since, "--", "sdk/.changeset"]).stdout
    : run("git", ["ls-files", "sdk/.changeset"]).stdout;
  const untracked = run("git", ["ls-files", "--others", "--exclude-standard", "sdk/.changeset"]).stdout;
  const majors = new Set<string>();
  for (const file of `${added}\n${untracked}`.split("\n")) {
    if (!file.endsWith(".md") || file.endsWith("README.md") || !fs.existsSync(path.join(root, file))) {
      continue;
    }
    const front = fs.readFileSync(path.join(root, file), "utf8").split("---")[1] ?? "";
    for (const match of front.matchAll(/^\s*["']?([^"':]+)["']?\s*:\s*major\s*$/gm)) {
      majors.add(match[1] ?? "");
    }
  }
  return majors;
}

const majors = majorBumps();

function breaking(pkg: string, sdk: string, details: string) {
  if (majors.has(pkg)) {
    notes.push(`${sdk}: breaking changes approved by a major changeset for ${pkg}`);
    return;
  }
  failures.push(`${sdk}: breaking changes need a major changeset for ${pkg}\n${details}`);
}

/* mirrorTag returns the latest vX.Y.Z tag of a mirror repository, or null before its first release. */
function mirrorTag(repo: string): string | null {
  const res = run("git", ["ls-remote", "--tags", "--refs", `${mirrors}/${repo}`]);
  if (!res.ok) {
    return null;
  }
  const tags = [...res.stdout.matchAll(/refs\/tags\/(v\d+\.\d+\.\d+)$/gm)].map((m) => m[1] ?? "");
  const key = (t: string) => t.slice(1).split(".").map(Number) as [number, number, number];
  tags.sort((x, y) => {
    const [a, b] = [key(x), key(y)];
    return a[0] - b[0] || a[1] - b[1] || a[2] - b[2];
  });
  return tags.at(-1) ?? null;
}

/* clone checks out a mirror's tag into a scratch directory. */
function clone(repo: string, tag: string): string {
  const dir = path.join(work, `${repo}-${tag}`);
  execFileSync("git", ["clone", "-q", "--depth", "1", "--branch", tag, `${mirrors}/${repo}`, dir], { stdio: "ignore" });
  fs.rmSync(path.join(dir, ".git"), { recursive: true, force: true });
  return dir;
}

/*
 * history commits a released copy and the current copy of a package into
 * a scratch repository, tagged base and HEAD, for tools that compare git
 * revisions.
 */
function history(name: string, base: string, current: string, edit?: (dir: string) => void): string {
  const scratch = path.join(work, `${name}-repo`);
  const g = (...argv: string[]) => execFileSync("git", ["-C", scratch, ...argv], { stdio: "ignore" });
  const commit = (message: string) =>
    g("-c", "user.name=compat", "-c", "user.email=compat@localhost", "commit", "-qm", message, "--allow-empty");
  const copy = (from: string) => {
    for (const entry of fs.existsSync(scratch) ? fs.readdirSync(scratch) : []) {
      if (entry !== ".git") {
        fs.rmSync(path.join(scratch, entry), { recursive: true, force: true });
      }
    }
    fs.cpSync(from, scratch, {
      recursive: true,
      filter: (src) => !/\/(vendor|\.venv|__pycache__|\.git|node_modules)(\/|$)/.test(`/${path.relative(from, src)}`),
    });
    edit?.(scratch);
    g("add", "-A", "-f");
  };
  fs.mkdirSync(scratch, { recursive: true });
  g("init", "-q");
  copy(base);
  commit("base");
  g("tag", "base");
  copy(current);
  commit("current");
  return scratch;
}

/* ---------------------------------------------------------------- TypeScript */

type Exports = Record<string, string | { types?: string; import?: { types?: string } }>;
type Manifest = { name: string; version: string; private?: boolean; exports?: Exports; main?: string };

/* declarations reports a normalized line per declaration, without comments. */
function declarations(report: string): string[] {
  return report
    .split("\n")
    .map((l) => l.trim())
    .filter((l) => l !== "" && !l.startsWith("//") && !l.startsWith("```") && !l.startsWith(">") && !l.startsWith("#"));
}

/* missing lists the lines of before that after has fewer of, so a member dropped from one of many alike types counts. */
function missing(before: string[], after: string[]): string[] {
  const left = new Map<string, number>();
  for (const line of after) {
    left.set(line, (left.get(line) ?? 0) + 1);
  }
  const gone: string[] = [];
  for (const line of before) {
    const n = left.get(line) ?? 0;
    if (n === 0) {
      gone.push(line);
    } else {
      left.set(line, n - 1);
    }
  }
  return gone;
}

function extract(dir: string, entry: string, types: string, label = path.basename(dir)): string | null {
  const out = path.join(work, label, entry);
  fs.mkdirSync(path.join(out, "tmp"), { recursive: true });
  const config = path.join(dir, ".api-extractor.tmp.json");
  fs.writeFileSync(
    config,
    JSON.stringify({
      projectFolder: ".",
      newlineKind: "lf",
      mainEntryPointFilePath: `<projectFolder>/${types.replace(/^\.\//, "")}`,
      compiler: { overrideTsconfig: { compilerOptions: { lib: ["ES2023", "DOM"], skipLibCheck: true, types: [] } } },
      apiReport: { enabled: true, reportFolder: `${out}/`, reportTempFolder: `${out}/tmp/`, reportFileName: entry },
      docModel: { enabled: false },
      dtsRollup: { enabled: false },
      tsdocMetadata: { enabled: false },
      messages: {
        extractorMessageReporting: { default: { logLevel: "none" } },
        tsdocMessageReporting: { default: { logLevel: "none" } },
        compilerMessageReporting: { default: { logLevel: "none" } },
      },
    }),
  );
  try {
    const res = run("pnpm", ["dlx", API_EXTRACTOR, "run", "--local", "-c", config], dir);
    const file = path.join(out, `${entry}.api.md`);
    if (!fs.existsSync(file)) {
      failures.push(`${path.relative(root, dir)}: api-extractor failed for ${entry}\n${res.out}`);
      return null;
    }
    return fs.readFileSync(file, "utf8");
  } finally {
    fs.rmSync(config, { force: true });
  }
}

function entries(manifest: Manifest): [string, string][] {
  const list: [string, string][] = [];
  for (const [key, value] of Object.entries(manifest.exports ?? {})) {
    const types = typeof value === "string" ? undefined : (value.types ?? value.import?.types);
    if (types) {
      list.push([key === "." ? "index" : key.replace(/^\.\//, "").replaceAll("/", "-"), types]);
    }
  }
  return list;
}

/* published unpacks the latest npm release of a package, or returns null before its first. */
function published(name: string): string | null {
  const version = run("npm", ["view", name, "version"]);
  if (!version.ok || version.stdout.trim() === "") {
    return null;
  }
  const into = path.join(work, "npm", name.replace("/", "__"));
  fs.mkdirSync(into, { recursive: true });
  const pack = run("npm", ["pack", `${name}@${version.stdout.trim()}`, "--pack-destination", into, "--silent"]);
  if (!pack.ok) {
    failures.push(`${name}: could not download the published package\n${pack.out}`);
    return null;
  }
  execFileSync("tar", ["-xzf", path.join(into, pack.stdout.trim().split("\n").at(-1) ?? ""), "-C", into]);
  return path.join(into, "package");
}

function checkReport(dir: string, entry: string, fresh: string, base: string | null): string[] {
  const rel = path.relative(root, dir);
  const file = path.join(dir, "api-report", `${entry}.api.md`);
  const committed = fs.existsSync(file) ? fs.readFileSync(file, "utf8") : null;
  const removed = base === null ? [] : missing(declarations(base), declarations(fresh));

  if (update) {
    fs.mkdirSync(path.dirname(file), { recursive: true });
    fs.writeFileSync(file, fresh);
  } else if (committed !== fresh) {
    failures.push(
      `${rel}: ${path.relative(root, file)} is ${committed === null ? "missing" : "out of date"}; ` +
        "run make sdk-compat-update and commit it",
    );
  }
  return removed.map((l) => `    - ${entry}: ${l}`);
}

/* releasedEntries maps each entry point of the published package to its report, or is null before the first release. */
function releasedReports(name: string, packageName: string): Map<string, string | null> | null {
  const release = published(packageName);
  if (!release) {
    return null;
  }
  const manifest: Manifest = JSON.parse(fs.readFileSync(path.join(release, "package.json"), "utf8"));
  return new Map(
    entries(manifest).map(([entry, types]) => [entry, extract(release, entry, types, `published-${name}`)]),
  );
}

function reports(name: string, dir: string, manifest: Manifest): string[] {
  const released = releasedReports(name, manifest.name);
  if (!released) {
    notes.push(`${manifest.name}: not published yet, only the committed report is checked`);
  }
  const current = entries(manifest);
  const removed = [...(released?.keys() ?? [])]
    .filter((entry) => !current.some(([e]) => e === entry))
    .map((entry) => `    - ${entry}: entry point removed`);
  for (const [entry, types] of current) {
    const fresh = extract(dir, entry, types);
    if (fresh !== null) {
      removed.push(...checkReport(dir, entry, fresh, released?.get(entry) ?? null));
    }
  }
  return removed;
}

function typescriptPackage(name: string, dir: string, manifest: Manifest) {
  if (!fs.existsSync(path.join(dir, "dist"))) {
    failures.push(`sdk/${name}: no dist/; run pnpm sdk:build first`);
    return;
  }
  process.stdout.write(`sdk-compat: ${manifest.name}\n`);
  const removed = noReport.has(manifest.name) ? [] : reports(name, dir, manifest);
  if (removed.length > 0) {
    breaking(manifest.name, `sdk/${name}`, removed.join("\n"));
  }

  const esmOnly = !manifest.main && !JSON.stringify(manifest.exports ?? {}).includes('"require"');
  const profile = esmOnly ? ["--profile", "esm-only"] : [];
  const attw = run("pnpm", ["dlx", ATTW, "--pack", ".", "--format", "ascii", ...profile], dir);
  if (!attw.ok) {
    failures.push(`sdk/${name}: @arethetypeswrong/cli found packaging problems\n${attw.out}`);
  }
}

function typescript() {
  for (const name of fs.readdirSync(path.join(root, "sdk")).sort()) {
    const dir = path.join(root, "sdk", name);
    const pkgFile = path.join(dir, "package.json");
    if (!fs.existsSync(pkgFile)) {
      continue;
    }
    const manifest: Manifest = JSON.parse(fs.readFileSync(pkgFile, "utf8"));
    if (!manifest.private) {
      typescriptPackage(name, dir, manifest);
    }
  }
}

/* ---------------------------------------------------------------- Go */

/*
 * apidiff writes the export data of the latest dimmihq/dimmi-go release
 * and of sdk/go with its generated code, then lists incompatible changes.
 */
function golang() {
  const dir = path.join(root, "sdk/go");
  if (!fs.existsSync(path.join(dir, "go.mod"))) {
    notes.push("go: no sdk/go/go.mod yet, skipped");
    return;
  }
  const tag = mirrorTag("dimmi-go");
  if (!tag) {
    notes.push("go: dimmi-go has no release yet, skipped");
    return;
  }
  const base = clone("dimmi-go", tag);
  const module = /^module\s+(\S+)/m.exec(fs.readFileSync(path.join(dir, "go.mod"), "utf8"))?.[1] ?? "";
  const write = (cwd: string, file: string) => run("go", ["run", APIDIFF, "-m", "-w", file, module], cwd);
  const old = write(base, path.join(work, "go-old.export"));
  const now = write(dir, path.join(work, "go-new.export"));
  if (!old.ok || !now.ok) {
    failures.push(`sdk/go: apidiff could not read the module\n${old.out}${now.out}`);
    return;
  }
  const res = run(
    "go",
    ["run", APIDIFF, "-m", "-incompatible", path.join(work, "go-old.export"), path.join(work, "go-new.export")],
    dir,
  );
  if (!res.ok) {
    failures.push(`sdk/go: apidiff failed\n${res.out}`);
  } else if (res.stdout.trim() !== "") {
    breaking(changesetName.go ?? "", `sdk/go (against dimmi-go ${tag})`, res.stdout.trim());
  }
}

/* ---------------------------------------------------------------- PHP */

function roave(): string | null {
  const tools = path.join(os.homedir(), ".cache", "dimmi-tools", "roave");
  const bin = path.join(tools, "vendor", "bin", "roave-backward-compatibility-check");
  if (!fs.existsSync(bin)) {
    fs.mkdirSync(tools, { recursive: true });
    const res = run("composer", ["--working-dir", tools, "--no-interaction", "--quiet", "require", ROAVE]);
    if (!res.ok) {
      failures.push(`php: could not install ${ROAVE}\n${res.out}`);
      return null;
    }
  }
  return bin;
}

/* dropPathRepositories removes what a mirror cannot have: path repositories, such as Laravel's ../php, and the lock. */
function dropPathRepositories(dir: string) {
  const file = path.join(dir, "composer.json");
  const manifest = JSON.parse(fs.readFileSync(file, "utf8"));
  if (Array.isArray(manifest.repositories)) {
    manifest.repositories = manifest.repositories.filter((r: { type?: string }) => r.type !== "path");
    fs.writeFileSync(file, `${JSON.stringify(manifest, null, 4)}\n`);
  }
  fs.rmSync(path.join(dir, "composer.lock"), { force: true });
}

/*
 * roave compares two revisions of a repository whose root is the package:
 * the latest release from the mirror, then sdk/<name> with its generated
 * code on top.
 */
function php(sdk: "php" | "laravel") {
  const dir = path.join(root, "sdk", sdk);
  if (!fs.existsSync(path.join(dir, "composer.json"))) {
    notes.push(`${sdk}: no sdk/${sdk}/composer.json yet, skipped`);
    return;
  }
  const repo = `dimmi-${sdk}`;
  const tag = mirrorTag(repo);
  if (!tag) {
    notes.push(`${sdk}: ${repo} has no release yet, skipped`);
    return;
  }
  const bin = roave();
  if (!bin) {
    return;
  }
  const scratch = history(sdk, clone(repo, tag), dir, dropPathRepositories);
  const res = run(bin, ["--from=base", "--to=HEAD", "--format=markdown"], scratch);
  if (!res.ok) {
    const found = res.out.split("\n").filter((l) => /\[BC\]|backwards-incompatible/.test(l));
    breaking(
      changesetName[sdk] ?? "",
      `sdk/${sdk} (against ${repo} ${tag})`,
      (found.length > 0 ? found : [res.out]).join("\n"),
    );
  }
}

/* ---------------------------------------------------------------- Python */

/* wheel unpacks the latest release of the Python SDK from PyPI, or returns null before its first. */
function wheel(): { version: string; dir: string } | null {
  const res = run("curl", ["-fsSL", `${pypi}/pypi/dimmi-sdk/json`]);
  if (!res.ok) {
    return null;
  }
  const info: { info: { version: string }; urls: { packagetype: string; url: string }[] } = JSON.parse(res.stdout);
  const url = info.urls.find((u) => u.packagetype === "bdist_wheel")?.url;
  if (!url) {
    return null;
  }
  const into = path.join(work, "pypi");
  fs.mkdirSync(path.join(into, "src"), { recursive: true });
  execFileSync("curl", ["-fsSL", url, "-o", path.join(into, "dimmi.whl")]);
  execFileSync("unzip", ["-q", path.join(into, "dimmi.whl"), "dimmi/*", "-d", path.join(into, "src")]);
  return { version: info.info.version, dir: into };
}

function python() {
  const dir = path.join(root, "sdk/python");
  if (!fs.existsSync(path.join(dir, "pyproject.toml"))) {
    notes.push("python: no sdk/python/pyproject.toml yet, skipped");
    return;
  }
  const release = wheel();
  if (!release) {
    notes.push("python: dimmi has no release on PyPI yet, skipped");
    return;
  }
  const current = path.join(work, "python-current");
  fs.mkdirSync(path.join(current, "src"), { recursive: true });
  fs.cpSync(path.join(dir, "src", "dimmi"), path.join(current, "src", "dimmi"), { recursive: true });
  const scratch = history("python", release.dir, current, (d) => fs.rmSync(path.join(d, "dimmi.whl"), { force: true }));
  const res = run("uvx", [GRIFFE, "check", "dimmi", "--search", "src", "--against", "base"], scratch);
  if (!res.ok) {
    breaking(changesetName.python ?? "", `sdk/python (against PyPI ${release.version})`, res.out);
  }
}

/* ---------------------------------------------------------------- main */

// The released SDKs are compared with the current ones as users get them: generated.
execFileSync(
  "node",
  [path.join(root, "scripts/sdk-generate.ts"), "--if-missing", ...wanted.filter((w) => w !== "ts")],
  {
    stdio: "inherit",
  },
);

const checks: Record<string, () => void> = {
  ts: typescript,
  go: golang,
  php: () => php("php"),
  laravel: () => php("laravel"),
  python,
};
for (const [name, check] of Object.entries(checks)) {
  if (wanted.length === 0 || wanted.includes(name)) {
    check();
  }
}

for (const note of notes) {
  process.stdout.write(`sdk-compat: ${note}\n`);
}
if (failures.length > 0) {
  process.stderr.write(`\nsdk-compat: ${failures.length} problem(s):\n\n${failures.join("\n\n")}\n`);
  process.stderr.write(`
SDKs never break their users. Keep the old function, type or field and
add the new one beside it, marked deprecated. A break that must ship
needs a major changeset for that SDK (pnpm changeset). See AGENTS.md.
`);
  process.exit(1);
}
process.stdout.write("sdk-compat: every SDK is backward compatible.\n");
