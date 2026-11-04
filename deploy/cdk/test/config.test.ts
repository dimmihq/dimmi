import { describe, expect, it } from "vitest";
import type { EnvironmentConfig } from "../lib/config/environment.ts";
import { validate } from "../lib/config/environment.ts";
import { environmentNamed, environments } from "../lib/config/environments.ts";

describe("environment config", () => {
  const staging = environments.staging;

  it("staging and production live in different regions", () => {
    expect(environments.staging.region).not.toBe(environments.production.region);
    expect(environments.staging.region).toBe("us-east-2");
    expect(environments.production.region).toBe("us-east-1");
  });

  it("staging is one small task running every role", () => {
    expect(staging.services.web.roles).toEqual(["web", "worker", "scheduler"]);
    expect(staging.services.web.desiredCount).toBe(1);
    expect(staging.services.worker).toBe(undefined);
    expect(staging.database.instanceType).toBe("t4g.micro");
    expect(staging.cache.nodeType).toBe("cache.t4g.micro");
  });

  it("production splits web from worker and protects its data", () => {
    const production = environments.production;

    expect(production.services.web.desiredCount > 1).toBeTruthy();
    expect(production.services.worker?.roles).toEqual(["worker", "scheduler"]);
    expect(production.protectData).toBe(true);
    expect(production.gate).toBe(false);
    expect(production.promotesFrom).toBe("staging");
  });

  it("an unknown environment names the ones that exist", () => {
    expect(() => environmentNamed("qa")).toThrow(/known: staging, production/);
    expect(() => environmentNamed(undefined)).toThrow(/-c env=<name>/);
  });

  it("a config with problems is refused with every problem listed", () => {
    const broken: EnvironmentConfig = {
      ...staging,
      name: "Bad Name",
      host: "https://staging.trydimmi.com",
      mail: { inbound: "mail.example.com", system: "mail.example.com" },
      network: { maxAzs: 1, natGateways: 0 },
      services: { web: { ...staging.services.web, roles: ["web"] } },
    };

    for (const problem of [
      "name Bad Name",
      "host https://",
      "must differ",
      "maxAzs",
      "worker role",
      "scheduler role",
    ]) {
      expect(() => validate(broken)).toThrow(problem);
    }
  });

  it("adding an environment is one valid entry", () => {
    const qa = validate({ ...staging, name: "qa", region: "eu-west-1", host: "qa.trydimmi.com" });

    expect(qa.name).toBe("qa");
  });
});
