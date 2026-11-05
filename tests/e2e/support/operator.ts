/*
 * Runs commands of the release binary the way the operator of an install
 * would, against the install the tests started: same directory, same
 * database, same keys.
 */
import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

interface Install {
  home: string;
  env: Record<string, string>;
}

/*
 * operator runs one command and returns what it printed.
 */
export function operator(...args: string[]): string {
  const database = process.env.E2E_DATABASE ?? "dimmi_e2e";
  const install = JSON.parse(readFileSync(join(tmpdir(), `${database}.install.json`), "utf8")) as Install;

  return execFileSync(resolve("bin/dimmi-cloud"), args, { cwd: install.home, env: install.env, encoding: "utf8" });
}
