#!/usr/bin/env node
import { Resolver as DnsResolver } from "node:dns/promises";
import { readFile } from "node:fs/promises";
import { parseArgs } from "node:util";
import { environmentNamed } from "../../cdk/lib/config/environments.ts";
import type { OutputsFile } from "./outputs.ts";
import { factsFromOutputs } from "./outputs.ts";
import { formatPlan, ownerComment, plan } from "./plan.ts";
import { desiredRecords } from "./records.ts";
import type { Fetch } from "./vercel.ts";
import { Vercel } from "./vercel.ts";
import type { Resolver } from "./verify.ts";
import { formatChecks, verify } from "./verify.ts";

/** What the commands reach for, so tests can hand in fakes. */
export type Dependencies = {
  env: Record<string, string | undefined>;
  fetch: Fetch;
  resolver: Resolver;
  readFile(path: string): Promise<string>;
  write(text: string): void;
};

const USAGE = `usage: pnpm dns <plan|apply|verify> --env <name> --outputs <file>

  plan    show what apply would change at Vercel, changing nothing
  apply   create and update this environment's records, delete the ones it no longer needs
  verify  look every record up in public DNS and say which are visible

--outputs is the file 'cdk deploy --outputs-file' wrote for the environment.
plan and apply read VERCEL_TOKEN, and VERCEL_TEAM_ID when the domain belongs to a team.`;

/**
 * Runs one command and returns the exit code. Plan and apply never touch a
 * record they do not own, and apply refuses to start while anything is
 * blocked, so a half-written zone never results from a clash.
 */
export async function run(argv: string[], deps: Dependencies): Promise<number> {
  const { positionals, values } = parseArgs({
    args: argv,
    allowPositionals: true,
    options: { env: { type: "string" }, outputs: { type: "string" } },
  });
  const command = positionals[0];

  if (!(command === "plan" || command === "apply" || command === "verify") || !values.env || !values.outputs) {
    deps.write(`${USAGE}\n`);
    return 2;
  }

  const config = environmentNamed(values.env);
  const outputs = JSON.parse(await deps.readFile(values.outputs)) as OutputsFile;
  const desired = desiredRecords(config, factsFromOutputs(outputs, config.name));

  if (command === "verify") {
    const checks = await verify(desired, deps.resolver);
    deps.write(`${formatChecks(checks)}\n`);
    return checks.every((item) => item.ok) ? 0 : 1;
  }

  const token = deps.env.VERCEL_TOKEN;
  if (!token) {
    deps.write("VERCEL_TOKEN is not set. Create one as described in docs/deploy/dns.md.\n");
    return 2;
  }

  const vercel = new Vercel({ token, domain: config.dnsZone, teamId: deps.env.VERCEL_TEAM_ID, fetch: deps.fetch });
  const changes = plan(desired, await vercel.list(), config.name);
  deps.write(`${formatPlan(changes)}\n`);

  if (command === "plan") {
    return 0;
  }
  if (changes.conflicts.length > 0) {
    deps.write("Nothing was changed. Remove or rename the clashing records at Vercel, then apply again.\n");
    return 1;
  }

  const comment = ownerComment(config.name);
  for (const record of changes.create) {
    await vercel.create(record, comment);
  }
  for (const { id, record } of changes.update) {
    await vercel.update(id, record, comment);
  }
  for (const record of changes.remove) {
    await vercel.remove(record.id);
  }
  deps.write("Applied. Run verify in a few minutes to see the records in public DNS.\n");

  return 0;
}

/**
 * Public resolvers rather than the machine's own, so `verify` sees what
 * the internet sees, not a stale local cache.
 */
function publicResolver(): Resolver {
  const resolver = new DnsResolver();
  resolver.setServers(["1.1.1.1", "8.8.8.8"]);

  return resolver;
}

if (import.meta.main) {
  const code = await run(process.argv.slice(2), {
    env: process.env,
    fetch: (url, init) => fetch(url, init),
    resolver: publicResolver(),
    readFile: (path) => readFile(path, "utf8"),
    write: (text) => process.stdout.write(text),
  });
  process.exitCode = code;
}
