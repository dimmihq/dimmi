import { describe, expect, it } from "vitest";
import type { EnvironmentConfig } from "../lib/config/environment.ts";
import { validate } from "../lib/config/environment.ts";
import { environmentNamed, environments } from "../lib/config/environments.ts";

describe("environment config", () => {
  const staging = environments.staging;

  it("staging and production keep their mail in different regions", () => {
    expect(environments.staging.region).toBe("us-east-2");
    expect(environments.production.region).toBe("us-east-1");
  });

  it("both environments sit behind the same load balancer", () => {
    expect(environments.staging.ingressAddresses).toEqual(environments.production.ingressAddresses);
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
      ingressAddresses: ["300.1.1.1"],
    };

    for (const problem of ["name Bad Name", "host https://", "must differ", "ingress address 300.1.1.1"]) {
      expect(() => validate(broken)).toThrow(problem);
    }
    expect(() => validate({ ...staging, ingressAddresses: [] })).toThrow("at least one address");
  });

  it("adding an environment is one valid entry", () => {
    const qa = validate({ ...staging, name: "qa", region: "eu-west-1", host: "qa.trydimmi.com" });

    expect(qa.name).toBe("qa");
  });
});
