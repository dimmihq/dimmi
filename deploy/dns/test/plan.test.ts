import { describe, expect, it } from "vitest";
import { environments } from "../../cdk/lib/config/environments.ts";
import { factsFromOutputs } from "../src/outputs.ts";
import { formatPlan, ownerComment, plan } from "../src/plan.ts";
import { desiredRecords } from "../src/records.ts";
import { live, STAGING_OUTPUTS } from "./fixtures.ts";

const desired = desiredRecords(environments.staging, factsFromOutputs(STAGING_OUTPUTS, "staging"));
const mine = ownerComment("staging");

describe("plan", () => {
  it("creates everything in an empty zone", () => {
    const result = plan(desired, [], "staging");

    expect(result.create).toHaveLength(5);
    expect(result.create.filter((record) => record.type === "A").map((record) => record.value)).toEqual([
      "192.0.2.10",
      "192.0.2.11",
    ]);
  });

  it("changes nothing when the zone already matches", () => {
    const zone = desired.map((record) => live({ ...record, comment: mine }));
    const result = plan(desired, zone, "staging");

    expect(result.unchanged).toBe(5);
    expect([...result.create, ...result.update, ...result.remove]).toHaveLength(0);
  });

  it("deletes only its own records that are no longer needed", () => {
    const zone = [
      live({ type: "A", name: "staging.trydimmi.com", value: "192.0.2.99", comment: mine }),
      live({ type: "A", name: "www.trydimmi.com", value: "192.0.2.50" }),
      live({ type: "A", name: "trydimmi.com", value: "192.0.2.60", comment: ownerComment("production") }),
    ];
    const result = plan(desired, zone, "staging");

    expect(result.remove.map((record) => record.value)).toEqual(["192.0.2.99"]);
  });

  it("updates a record whose ttl drifted", () => {
    const zone = desired.map((record) => live({ ...record, comment: mine, ttl: record.type === "MX" ? 60 : 300 }));

    expect(plan(desired, zone, "staging").update).toHaveLength(1);
  });

  it("leaves an identical record added by hand alone instead of duplicating it", () => {
    const zone = [
      live({ type: "TXT", name: "inbound.staging.trydimmi.com", value: "v=spf1 include:amazonses.com -all" }),
    ];
    const result = plan(desired, zone, "staging");

    expect(result.unmanaged).toHaveLength(1);
    expect(result.create.some((record) => record.type === "TXT")).toBe(false);
  });

  it("blocks records that clash with someone else's", () => {
    const zone = [
      live({ type: "CNAME", name: "staging.trydimmi.com", value: "cname.vercel-dns.com" }),
      live({ type: "TXT", name: "inbound.staging.trydimmi.com", value: "v=spf1 include:_spf.google.com ~all" }),
    ];
    const result = plan(desired, zone, "staging");

    expect(result.conflicts.map(({ record }) => record.type).sort()).toEqual(["A", "A", "TXT"]);
    expect(formatPlan(result)).toMatch(/3 blocked/);
  });

  it("names match without case or trailing dot", () => {
    const zone = desired.map((record) =>
      live({ ...record, name: `${record.name.toUpperCase()}.`, value: `${record.value}`, comment: mine }),
    );

    expect(plan(desired, zone, "staging").unchanged).toBe(5);
  });
});
