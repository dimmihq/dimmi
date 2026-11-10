/*
 * Starts the release cloud binary the way production runs it, for the
 * end-to-end tests: a database of its own created fresh, migrations
 * applied with the production command, Valkey keys under a prefix of
 * this run, mail through Mailpit, and nothing from the developer's
 * shell or dimmi.yaml: it runs in an empty directory of its own, where
 * no config file waits. Playwright starts it and stops it. E2E_DATABASE
 * and E2E_PORT let several checkouts run their suites side by side.
 * Google sign-in points at a local provider started here on a free port,
 * which the binary is allowed to reach and nothing else is. It serves
 * the pseudo locale en-XA beside English, so the language tests can
 * switch to a language whose every word looks different. Like every cloud
 * install it has built-in model credentials, pointed at a closed local
 * port so no test can ever reach a real model. It runs in the
 * local environment: production would refuse its local disk and plain
 * http address, and testing would swap real services for fakes.
 *
 * Generate the pseudo locale and build the binary first, with
 * `make pseudo-locale build`; `make e2e` does both.
 */
import { execFileSync, spawn } from "node:child_process";
import { existsSync, mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { serveZone } from "./support/dns.ts";
import { startIdentityProvider } from "./support/identity-provider.ts";

const binary = resolve("bin/dimmi-cloud");
const database = process.env.E2E_DATABASE ?? "dimmi_e2e";
const port = Number(process.env.E2E_PORT ?? 18090);

if (!existsSync(binary)) {
  throw new Error(`${binary} is missing: run \`make build\` first`);
}

if (!existsSync(resolve("lang/en-XA"))) {
  throw new Error("the pseudo locale is missing: run `make pseudo-locale build` first");
}

function psql(sql: string) {
  execFileSync("docker", ["compose", "exec", "-T", "postgres", "psql", "-U", "postgres", "-qc", sql], {
    stdio: "inherit",
  });
}

psql(`DROP DATABASE IF EXISTS ${database} WITH (FORCE)`);
psql(`CREATE DATABASE ${database}`);

const home = mkdtempSync(join(tmpdir(), "dimmi-e2e-"));

/*
 * resolver answers the server's domain checks from the records tests
 * publish through support/dns.ts.
 */
const resolver = await serveZone(port);

/*
 * inboundSecret opens the test inbound source, through which the tests
 * hand the server raw emails; support/inbound.ts sends the same one.
 */
const inboundSecret = "e2e-inbound-secret";

const provider = await startIdentityProvider();

const env = {
  PATH: process.env.PATH ?? "",
  HOME: process.env.HOME ?? "",
  DIMMI_APP__ENV: "local",
  DIMMI_APP__URL: `http://localhost:${port}`,
  DIMMI_APP__KEY_AUTO_GENERATE: "true",
  DIMMI_APP__DATA_DIR: join(home, "storage"),
  DIMMI_APP__REGION: "us",
  DIMMI_APP__LOCALES: "en,en-XA",
  DIMMI_HTTP__ADDR: `127.0.0.1:${port}`,
  DIMMI_HTTP__ADMIN_ADDR: "",
  DIMMI_HTTP__TRUSTED_PROXIES: "127.0.0.1/32",
  DIMMI_DATABASE__URL: `postgres://postgres:postgres@127.0.0.1:15432/${database}?sslmode=disable`,
  DIMMI_REDIS__URL: "redis://127.0.0.1:16379/5",
  DIMMI_REDIS__KEY_PREFIX: `e2e${Date.now()}:`,
  DIMMI_AUTH__SIGNUP__MAX_PER_HOUR_PER_IP: "1000",
  DIMMI_API__LIMITS__BAD_CREDENTIAL_PER_MINUTE: "10000",
  DIMMI_MAIL__MAILERS__SYSTEM__DRIVER: "smtp",
  DIMMI_MAIL__MAILERS__SYSTEM__HOST: "127.0.0.1",
  DIMMI_MAIL__MAILERS__SYSTEM__PORT: "11025",
  DIMMI_MAIL__MAILERS__SYSTEM__ALLOW_INSECURE: "true",
  DIMMI_CHANNELS__EMAIL__INBOUND__DRIVERS: "ses,test",
  DIMMI_CHANNELS__EMAIL__INBOUND__TEST_SECRET: inboundSecret,
  DIMMI_INBOX__GIF__GIPHY_API_KEY: "e2e-giphy-key",
  DIMMI_AI__MANAGED__OPENAI__API_KEY: "e2e-managed-key",
  DIMMI_AI__MANAGED__OPENAI__BASE_URL: "http://127.0.0.1:9/v1",
  DIMMI_AUTH__OAUTH__GOOGLE__CLIENT_ID: provider.clientId,
  DIMMI_AUTH__OAUTH__GOOGLE__CLIENT_SECRET: provider.clientSecret,
  DIMMI_SOCIALITE__PROVIDERS__GOOGLE__ISSUER: provider.issuer,
  DIMMI_HTTP_CLIENT__ALLOW_HOSTS: new URL(provider.issuer).host,
  DIMMI_CHANNELS__EMAIL__SENDING__RESOLVERS: resolver,
};

execFileSync(binary, ["migrate", "--force"], { cwd: home, env, stdio: "inherit" });

/*
 * The tests run operator commands against this same install, so they
 * read its directory and environment from a file named after the
 * database; support/operator.ts finds it.
 */
writeFileSync(join(tmpdir(), `${database}.install.json`), JSON.stringify({ home, env }));

const server = spawn(binary, ["serve"], { cwd: home, env, stdio: "inherit" });
for (const signal of ["SIGINT", "SIGTERM"] as const) {
  process.on(signal, () => server.kill("SIGTERM"));
}
server.on("exit", (code) => process.exit(code ?? 0));
