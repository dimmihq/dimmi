/*
 * Fails when the public API or the webhooks break a client built against
 * an earlier contract. It compares tests/contracts/api_v1.openapi.json
 * with a base through oasdiff, and tests/contracts/webhooks_v1.json with
 * the base's webhook contract. Additive changes pass; anything that could
 * break a client fails unless contracts/breaking-changes.json approves it.
 *
 *   node scripts/api-compat.ts                    against the last platform release (newest vX.Y.Z tag)
 *   node scripts/api-compat.ts --ref origin/main  against a git ref
 *   node scripts/api-compat.ts --ref v0.1.0 --to v0.2.0 --json
 *                                                 compare two refs and print a summary, for the SDK release
 *
 * Severity: everything oasdiff calls an error or a warning, plus the
 * removals contracts/oasdiff-severity.txt raises (optional response
 * fields, operation ids the SDKs name methods after, deprecated paths).
 */
import { execFileSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const OASDIFF = "github.com/oasdiff/oasdiff@v1.37.0";

const root = path.join(path.dirname(fileURLToPath(import.meta.url)), "..");
const current = { api: "tests/contracts/api_v1.openapi.json", webhooks: "tests/contracts/webhooks_v1.json" };
const allowlistFile = "contracts/breaking-changes.json";

type Finding = { id: string; operation: string; text: string };
type Allowed = {
  id: string;
  operation?: string;
  event?: string;
  match?: string;
  reason: string;
  deprecated: string;
  sunset: string;
  approved_by: string;
};

const args = process.argv.slice(2);
const refIndex = args.indexOf("--ref");
const toIndex = args.indexOf("--to");
const to = toIndex >= 0 ? args[toIndex + 1] : undefined;
const json = args.includes("--json");
const ref = refIndex >= 0 ? args[refIndex + 1] : (lastRelease() ?? "origin/main");
const work = fs.mkdtempSync(path.join(os.tmpdir(), "dimmi-api-compat-"));

/*
 * lastRelease returns the newest platform release tag, vX.Y.Z without a
 * prerelease suffix, or null before the first. The release is the
 * baseline: no copy of a released contract is kept in the tree.
 */
function lastRelease(): string | null {
  try {
    const tags = execFileSync("git", ["tag", "--list", "v*.*.*", "--sort=-v:refname"], {
      cwd: root,
      encoding: "utf8",
      stdio: ["ignore", "pipe", "ignore"],
    });
    return (
      tags
        .split("\n")
        .find((t) => /^v\d+\.\d+\.\d+$/.test(t.trim()))
        ?.trim() ?? null
    );
  } catch {
    return null;
  }
}

/* at returns a file's path as it is at a git ref, or null when the ref has none. */
function at(at: string, file: string, name: string): string | null {
  try {
    const text = execFileSync("git", ["show", `${at}:${file}`], {
      cwd: root,
      encoding: "utf8",
      maxBuffer: 64 << 20,
      stdio: ["ignore", "pipe", "ignore"],
    });
    const out = path.join(work, name);
    fs.writeFileSync(out, text);
    return out;
  } catch {
    return null;
  }
}

/* base returns the base file's path at the ref, or null when the ref has none yet. */
function base(file: string): string | null {
  return at(ref, file, `base-${path.basename(file)}`);
}

/* target returns the compared file: at --to, or in the working tree. */
function target(file: string): string | null {
  if (to) {
    return at(to, file, `to-${path.basename(file)}`);
  }
  const full = path.join(root, file);
  return fs.existsSync(full) ? full : null;
}

function apiFindings(basePath: string): Finding[] {
  const out = execFileSync(
    "go",
    [
      "run",
      OASDIFF,
      "changelog",
      basePath,
      target(current.api) ?? "",
      "--format",
      "json",
      "--level",
      "WARN",
      "--severity-levels",
      path.join(root, "contracts/oasdiff-severity.txt"),
    ],
    { cwd: root, encoding: "utf8", maxBuffer: 64 << 20 },
  );
  const changes: { id: string; text: string; operation?: string; path?: string }[] = JSON.parse(out || "[]");
  return changes.map((c) => ({
    id: c.id,
    operation: c.operation ? `${c.operation} ${c.path}` : (c.path ?? ""),
    text: c.text,
  }));
}

type Schema = { properties?: Record<string, { type?: unknown; format?: string; $ref?: string }> };
type Webhooks = {
  envelope: Record<string, Schema>;
  events: { type: string; object: string; named_only: boolean; notes_opt_in: boolean }[];
};

type WebhookEvent = Webhooks["events"][number];

/* eventChange says how an event narrowed for consumers, or null when it did not. */
function eventChange(before: WebhookEvent, after: WebhookEvent | undefined): Omit<Finding, "operation"> | null {
  if (!after) {
    return { id: "webhook-event-removed", text: "the event is no longer sent" };
  }
  if (after.object !== before.object) {
    return {
      id: "webhook-event-object-changed",
      text: `its data object changed from ${before.object} to ${after.object}`,
    };
  }
  if (after.named_only && !before.named_only) {
    return { id: "webhook-event-became-named-only", text: "wildcard subscriptions no longer receive it" };
  }
  if (after.notes_opt_in !== before.notes_opt_in) {
    return { id: "webhook-event-notes-changed", text: "which endpoints receive notes changed" };
  }
  return null;
}

function envelopeFindings(was: Webhooks["envelope"], now: Webhooks["envelope"]): Finding[] {
  const findings: Finding[] = [];
  for (const [name, schema] of Object.entries(was)) {
    for (const [field, type] of Object.entries(schema.properties ?? {})) {
      const operation = `envelope ${name}.${field}`;
      const after = now[name]?.properties?.[field];
      if (!after) {
        findings.push({ id: "webhook-envelope-property-removed", operation, text: "the field was removed" });
      } else if (JSON.stringify(after) !== JSON.stringify(type)) {
        findings.push({
          id: "webhook-envelope-property-type-changed",
          operation,
          text: `its type changed from ${JSON.stringify(type)} to ${JSON.stringify(after)}`,
        });
      }
    }
  }
  return findings;
}

function webhookFindings(basePath: string): Finding[] {
  const was: Webhooks = JSON.parse(fs.readFileSync(basePath, "utf8"));
  const now: Webhooks = JSON.parse(fs.readFileSync(target(current.webhooks) ?? "", "utf8"));
  const events = new Map(now.events.map((e) => [e.type, e]));
  const findings: Finding[] = [];
  for (const before of was.events) {
    const change = eventChange(before, events.get(before.type));
    if (change) {
      findings.push({ ...change, operation: `event ${before.type}` });
    }
  }
  return [...findings, ...envelopeFindings(was.envelope, now.envelope)];
}

function entryProblems(e: Allowed, i: number): string[] {
  const problems: string[] = [];
  for (const key of ["id", "reason", "deprecated", "sunset", "approved_by"] as const) {
    if (!e[key]) {
      problems.push(`entry ${i} has no ${key}`);
    }
  }
  if (!e.operation && !e.event) {
    problems.push(`entry ${i} names neither an operation nor an event`);
  }
  for (const key of ["deprecated", "sunset"] as const) {
    if (e[key] && Number.isNaN(Date.parse(e[key]))) {
      problems.push(`entry ${i} ${key} is not a date`);
    }
  }
  return problems;
}

function loadAllowlist(): Allowed[] {
  const file = target(allowlistFile);
  const { entries }: { entries: Allowed[] } = file ? JSON.parse(fs.readFileSync(file, "utf8")) : { entries: [] };
  const problems = entries.flatMap(entryProblems);
  if (problems.length > 0) {
    process.stderr.write(`${allowlistFile} is invalid:\n  ${problems.join("\n  ")}\n`);
    process.exit(1);
  }
  return entries;
}

function allows(entry: Allowed, f: Finding): boolean {
  const target = entry.operation ?? `event ${entry.event}`;
  return (
    entry.id === f.id &&
    (target === f.operation || f.operation.startsWith(`${target}.`)) &&
    (!entry.match || f.text.includes(entry.match))
  );
}

const allowlist = loadAllowlist();
const against = ref;
const findings: Finding[] = [];
const say = (line: string) => (json ? undefined : process.stdout.write(line));

/* changed says whether the contract files differ between the base and the target. */
const contractChanged = [current.api, current.webhooks].some((file) => {
  const read = (f: string | null) => (f ? fs.readFileSync(f, "utf8") : null);
  return read(base(file)) !== read(target(file));
});

const apiBase = base(current.api);
if (apiBase && target(current.api)) {
  findings.push(...apiFindings(apiBase));
} else {
  say(`api-compat: ${ref} has no ${current.api} yet, skipping the API.\n`);
}
const hooksBase = base(current.webhooks);
if (hooksBase && target(current.webhooks)) {
  findings.push(...webhookFindings(hooksBase));
} else {
  say(`api-compat: ${ref} has no ${current.webhooks} yet, skipping webhooks.\n`);
}

const blocking = findings.filter((f) => !allowlist.some((e) => allows(e, f)));
const approved = findings.length - blocking.length;
const unused = allowlist.filter((e) => !findings.some((f) => allows(e, f)));

if (json) {
  const baseless = !apiBase || !hooksBase;
  process.stdout.write(
    `${JSON.stringify({ base: ref, target: to ?? "working tree", baseless, changed: contractChanged, approved: findings.filter((f) => !blocking.includes(f)), blocking }, null, 2)}\n`,
  );
  process.exit(0);
}

if (approved > 0) {
  process.stdout.write(`api-compat: ${approved} breaking change(s) approved in ${allowlistFile}.\n`);
}
if (unused.length > 0) {
  process.stdout.write(
    `api-compat: ${unused.length} entr(ies) in ${allowlistFile} match nothing since ${ref}; remove them:\n${unused
      .map((e) => `  ${e.id} ${e.operation ?? e.event}`)
      .join("\n")}\n`,
  );
}

if (blocking.length > 0) {
  process.stderr.write(`\napi-compat: ${blocking.length} breaking change(s) against ${against}:\n\n`);
  for (const f of blocking) {
    process.stderr.write(`  ✗ [${f.id}] ${f.operation}: ${f.text}\n`);
  }
  process.stderr.write(`
The public API and webhooks are additive only (docs/api/versioning.md).
Keep the old field, parameter, path or event and add the new one beside it.
A break that must ship needs a new API version, or an entry in
${allowlistFile} with a reason, deprecation and sunset dates and an approver.
`);
  process.exit(1);
}

process.stdout.write(`api-compat: no breaking changes against ${against}.\n`);
