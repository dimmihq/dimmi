/*
 * End-to-end tests drive a real browser against the release cloud
 * binary, started fresh for every run by tests/e2e/server.ts, so they
 * check what customers get: the embedded build, the production content
 * security policy, real cookies and mail through Mailpit. Every test
 * runs in light and in dark.
 */
import { defineConfig, devices } from "@playwright/test";

const baseURL = `http://localhost:${process.env.E2E_PORT ?? 18090}`;

export default defineConfig({
  testDir: "tests/e2e",
  testMatch: "**/*.spec.ts",
  fullyParallel: true,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? "github" : "list",
  use: { baseURL, trace: "retain-on-failure", screenshot: "only-on-failure" },
  projects: [
    { name: "light", use: { ...devices["Desktop Chrome"], colorScheme: "light" } },
    { name: "dark", use: { ...devices["Desktop Chrome"], colorScheme: "dark" } },
  ],
  webServer: {
    command: "node tests/e2e/server.ts",
    url: `${baseURL}/ready`,
    reuseExistingServer: false,
    timeout: 120_000,
    stdout: "ignore",
    stderr: "pipe",
  },
});
