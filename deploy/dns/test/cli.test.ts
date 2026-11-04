import { describe, expect, it } from "vitest";
import type { Dependencies } from "../src/cli.ts";
import { run } from "../src/cli.ts";
import { ownerComment } from "../src/plan.ts";
import type { Fetch } from "../src/vercel.ts";
import { STAGING_OUTPUTS } from "./fixtures.ts";

type Zone = {
  id: string;
  name: string;
  type: string;
  value: string;
  mxPriority?: number;
  ttl: number;
  comment: string;
}[];

/** An in-memory Vercel zone behind the fake fetch, so apply can be run twice. */
function harness(zone: Zone, env: Record<string, string> = { VERCEL_TOKEN: "t" }) {
  const output: string[] = [];
  const methods: string[] = [];
  const fetch: Fetch = async (url, init) => {
    methods.push(init.method);
    const body = init.body ? (JSON.parse(init.body) as Zone[number]) : undefined;
    if (init.method === "POST" && body) {
      zone.push({ ...body, id: `rec_${zone.length}` });
    }
    if (init.method === "DELETE") {
      zone.splice(
        zone.findIndex((record) => url.endsWith(`/${record.id}`)),
        1,
      );
    }
    const page = { records: zone, pagination: { next: null } };

    return { ok: true, status: 200, json: async () => page, text: async () => "" };
  };
  const deps: Dependencies = {
    env,
    fetch,
    resolver: {
      resolve4: async () => ["192.0.2.10", "192.0.2.11"],
      resolveCname: async () => ["abc.dkim.amazonses.com"],
      resolveMx: async () => [{ exchange: "inbound-smtp.us-east-2.amazonaws.com", priority: 10 }],
      resolveTxt: async () => {
        throw Object.assign(new Error("not found"), { code: "ENOTFOUND" });
      },
    },
    readFile: async () => JSON.stringify(STAGING_OUTPUTS),
    write: (text) => output.push(text),
  };

  return { deps, output: () => output.join(""), methods, zone };
}

const args = (command: string) => [command, "--env", "staging", "--outputs", "outputs.json"];

describe("dns command", () => {
  it("plan changes nothing", async () => {
    const h = harness([]);

    expect(await run(args("plan"), h.deps)).toBe(0);
    expect(h.methods).toEqual(["GET"]);
    expect(h.output()).toMatch(/5 to create/);
  });

  it("apply is idempotent: a second run has nothing to do", async () => {
    const h = harness([]);

    expect(await run(args("apply"), h.deps)).toBe(0);
    expect(h.zone.every((record) => record.comment === ownerComment("staging"))).toBe(true);
    expect(await run(args("apply"), h.deps)).toBe(0);
    expect(h.output()).toMatch(/0 to create, 0 to update, 0 to delete, 5 unchanged/);
  });

  it("apply changes nothing when a record is blocked", async () => {
    const h = harness([{ id: "x", name: "staging", type: "CNAME", value: "elsewhere.example", ttl: 60, comment: "" }]);

    expect(await run(args("apply"), h.deps)).toBe(1);
    expect(h.methods).toEqual(["GET"]);
    expect(h.output()).toMatch(/Nothing was changed/);
  });

  it("verify reports each record in plain words and fails while one is missing", async () => {
    const h = harness([]);

    expect(await run(args("verify"), h.deps)).toBe(1);
    expect(h.output()).toMatch(/inbound.staging.trydimmi.com has no TXT record yet \(ENOTFOUND\)/);
    expect(h.output()).toMatch(/staging.trydimmi.com A points where it should/);
    expect(h.output()).toMatch(/1 of 5 records are not visible yet/);
  });

  it("asks for a token before reaching vercel", async () => {
    const h = harness([], {});

    expect(await run(args("plan"), h.deps)).toBe(2);
    expect(h.methods).toEqual([]);
  });

  it("remove deletes only this environment's records", async () => {
    const h = harness([
      { id: "a", name: "staging", type: "A", value: "192.0.2.10", ttl: 300, comment: ownerComment("staging") },
      { id: "b", name: "", type: "A", value: "192.0.2.20", ttl: 300, comment: ownerComment("production") },
      { id: "c", name: "www", type: "CNAME", value: "cname.vercel-dns.com", ttl: 300, comment: "" },
    ]);

    expect(await run(["remove", "--env", "staging"], h.deps)).toBe(0);
    expect(h.zone.map((record) => record.id)).toEqual(["b", "c"]);
  });

  it("prints usage for anything else", async () => {
    const h = harness([]);

    expect(await run(["deploy"], h.deps)).toBe(2);
    expect(h.output()).toMatch(/usage: pnpm dns/);
  });
});
