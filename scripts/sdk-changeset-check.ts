/*
 * Fails when the API contract or an SDK changed since a base without a
 * changeset, so every API change reaches a released SDK version.
 *
 *   node scripts/sdk-changeset-check.ts [base]   base defaults to origin/main
 *
 * Changes in the working tree count too, so it can run before a commit.
 * A release commit, which consumes changesets, passes.
 */
import { execFileSync } from "node:child_process";

const base = process.argv[2] ?? "origin/main";

function lines(...args: string[]): string[] {
  return execFileSync("git", args, { encoding: "utf8" }).split("\n").filter(Boolean);
}

const committed = lines("diff", "--name-status", `${base}...HEAD`);
const local = lines("status", "--porcelain", "--untracked-files=all").map(
  (l) => `${l.slice(0, 2).trim()}\t${l.slice(3)}`,
);
const changes = [...committed, ...local].map((l) => {
  const [status = "", ...rest] = l.split("\t");
  return { status: status.charAt(0), file: rest.at(-1) ?? "" };
});

const contract = /^tests\/contracts\/(api_v1\.openapi|webhooks_v1)\.json$/;
const sdkSource = /^sdk\/(?!examples\/)[^/.][^/]*\/(?!tests\/|api-report\/|dist\/|vendor\/|README\.md$|CHANGELOG\.md$)/;

const touched = changes
  .filter((c) => contract.test(c.file) || (sdkSource.test(c.file) && !c.file.endsWith(".test.ts")))
  .map((c) => c.file);
const changeset = (c: { file: string }) => /^sdk\/\.changeset\/.+\.md$/.test(c.file) && !c.file.endsWith("README.md");
const added = changes.some((c) => changeset(c) && c.status !== "D");
const released = changes.some((c) => changeset(c) && c.status === "D");

if (touched.length === 0 || added || released) {
  process.stdout.write("sdk-changeset-check: ok\n");
  process.exit(0);
}

const shown = [...new Set(touched)].slice(0, 10);
process.stderr.write(`sdk-changeset-check: the API contract or an SDK changed since ${base} without a changeset:

  ${shown.join("\n  ")}${touched.length > shown.length ? "\n  ..." : ""}

Run make sdk-generate and make sdk-check, then pnpm changeset and pick
every SDK whose generated code or surface changed (patch for fixes,
minor for new API surface, major only for an approved break).
`);
process.exit(1);
