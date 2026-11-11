import { describe, expect, it } from "vitest";
import { environments } from "../../cdk/lib/config/environments.ts";
import { factsFromOutputs } from "../src/outputs.ts";
import { desiredRecords } from "../src/records.ts";
import { STAGING, STAGING_OUTPUTS } from "./fixtures.ts";

describe("desired records", () => {
  it("points the host at every load balancer address and adds the mail records", () => {
    const records = desiredRecords(STAGING, factsFromOutputs(STAGING_OUTPUTS, STAGING));

    expect(records.map((record) => `${record.type} ${record.name}`)).toEqual([
      "A staging.trydimmi.com",
      "A staging.trydimmi.com",
      "MX inbound.staging.trydimmi.com",
      "TXT inbound.staging.trydimmi.com",
      "CNAME abc._domainkey.inbound.staging.trydimmi.com",
    ]);
  });

  it("uses a records at the apex for production, where a cname is not allowed", () => {
    const production = { ...environments.production, ingressAddresses: ["192.0.2.20"] };
    const outputs = { "dimmi-production-mail": { DnsRecords: "[]" } };
    const records = desiredRecords(production, factsFromOutputs(outputs, production));

    expect(records).toEqual([{ type: "A", name: "trydimmi.com", value: "192.0.2.20", ttl: 300 }]);
  });

  it("refuses outputs that are missing, rather than planning to delete live records", () => {
    expect(() => factsFromOutputs({}, STAGING)).toThrow(/DnsRecords of dimmi-staging-mail is missing/);
  });

  it("refuses a record outside the zone", () => {
    const facts = { addresses: ["192.0.2.1"], mail: [{ type: "TXT" as const, name: "example.com", value: "x" }] };

    expect(() => desiredRecords(environments.staging, facts)).toThrow(/outside the zone/);
  });
});
