/*
 * End-to-end tests drive a real browser against the release cloud
 * binary, started fresh for every run by tests/e2e/server.ts, so they
 * check what customers get: the embedded build, the production content
 * security policy, real cookies and mail through Mailpit. Every test
 * runs in light and in dark.
 *
 * With E2E_BASE_URL set they run against a deployed environment instead,
 * such as staging: no server is started, E2E_GATE_KEY is sent as the
 * gate's key on every request, and specs that need what only a local run
 * has (the database, Mailpit, the test inbound driver) are left out.
 */
import { globSync, readFileSync } from "node:fs";
import { defineConfig, devices } from "@playwright/test";

const remote = process.env.E2E_BASE_URL;
const baseURL = remote ?? `http://localhost:${process.env.E2E_PORT ?? 18090}`;
const gateKey = process.env.E2E_GATE_KEY;

/*
 * localOnly lists the specs that reach into the local services, which a
 * deployed environment does not expose.
 */
function localOnly(): string[] {
  return globSync("tests/e2e/**/*.spec.ts").filter((file) =>
    /support\/(database|mail|inbound)/.test(readFileSync(file, "utf8")),
  );
}

export default defineConfig({
  testDir: "tests/e2e",
  testMatch: "**/*.spec.ts",
  testIgnore: remote ? localOnly().map((file) => file.replace(/^tests\/e2e\//, "")) : [],
  fullyParallel: true,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? "github" : "list",
  use: {
    baseURL,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    extraHTTPHeaders: gateKey ? { "X-Gate-Key": gateKey } : undefined,
  },
  projects: [
    { name: "light", use: { ...devices["Desktop Chrome"], colorScheme: "light" } },
    { name: "dark", use: { ...devices["Desktop Chrome"], colorScheme: "dark" } },
  ],
  webServer: remote
    ? undefined
    : {
        command: "node tests/e2e/server.ts",
        url: `${baseURL}/ready`,
        reuseExistingServer: false,
        timeout: 120_000,
        stdout: "ignore",
        stderr: "pipe",
      },
});
