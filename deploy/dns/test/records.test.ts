import { describe, expect, it } from "vitest";
import { environments } from "../../cdk/lib/config/environments.ts";
import { factsFromOutputs } from "../src/outputs.ts";
import { desiredRecords } from "../src/records.ts";
import { STAGING_OUTPUTS } from "./fixtures.ts";

describe("desired records", () => {
  it("points the host at every load balancer address and adds the mail records", () => {
    const records = desiredRecords(environments.staging, factsFromOutputs(STAGING_OUTPUTS, "staging"));

    expect(records.map((record) => `${record.type} ${record.name}`)).toEqual([
      "A staging.trydimmi.com",
      "A staging.trydimmi.com",
      "MX inbound.staging.trydimmi.com",
      "TXT inbound.staging.trydimmi.com",
      "CNAME abc._domainkey.inbound.staging.trydimmi.com",
    ]);
  });

  it("uses a records at the apex for production, where a cname is not allowed", () => {
    const outputs = {
      "dimmi-production-app": { LoadBalancerAddresses: "192.0.2.20" },
      "dimmi-production-mail": { DnsRecords: "[]" },
    };
    const records = desiredRecords(environments.production, factsFromOutputs(outputs, "production"));

    expect(records).toEqual([{ type: "A", name: "trydimmi.com", value: "192.0.2.20", ttl: 300 }]);
  });

  it("refuses outputs that are missing, rather than planning to delete live records", () => {
    expect(() => factsFromOutputs({}, "staging")).toThrow(/LoadBalancerAddresses of dimmi-staging-app is missing/);
  });

  it("refuses a record outside the zone", () => {
    const facts = { addresses: ["192.0.2.1"], mail: [{ type: "TXT" as const, name: "example.com", value: "x" }] };

    expect(() => desiredRecords(environments.staging, facts)).toThrow(/outside the zone/);
  });
});
