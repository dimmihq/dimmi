import { describe, expect, it } from "vitest";
import type { Fetch } from "../src/vercel.ts";
import { Vercel } from "../src/vercel.ts";

type Call = { url: string; method: string; body?: unknown; auth: string };

/** A fake Vercel API answering from a queue and recording every call. */
function fakeApi(answers: unknown[]): { fetch: Fetch; calls: Call[] } {
  const calls: Call[] = [];
  const fetch: Fetch = async (url, init) => {
    calls.push({
      url,
      method: init.method,
      body: init.body ? JSON.parse(init.body) : undefined,
      auth: init.headers.Authorization,
    });
    const answer = answers.shift() ?? {};

    return { ok: true, status: 200, json: async () => answer, text: async () => "" };
  };

  return { fetch, calls };
}

describe("vercel client", () => {
  it("lists every page and turns relative names into full ones", async () => {
    const api = fakeApi([
      { records: [{ id: "1", name: "", type: "A", value: "192.0.2.1", ttl: 60 }], pagination: { next: 42 } },
      {
        records: [{ id: "2", name: "inbound", type: "MX", value: "x.example", mxPriority: 10 }],
        pagination: { next: null },
      },
    ]);
    const vercel = new Vercel({ token: "t", domain: "trydimmi.com", teamId: "team_1", fetch: api.fetch });

    const records = await vercel.list();

    expect(records.map((record) => record.name)).toEqual(["trydimmi.com", "inbound.trydimmi.com"]);
    expect(records[1].priority).toBe(10);
    expect(api.calls[1].url).toMatch(/until=42/);
    expect(api.calls[0].url).toMatch(/teamId=team_1/);
    expect(api.calls[0].auth).toBe("Bearer t");
  });

  it("creates with a relative name, the owner comment and an mx priority", async () => {
    const api = fakeApi([{ uid: "new" }]);
    const vercel = new Vercel({ token: "t", domain: "trydimmi.com", fetch: api.fetch });

    await vercel.create(
      { type: "MX", name: "inbound.staging.trydimmi.com", value: "m.example", priority: 10, ttl: 300 },
      "c",
    );

    expect(api.calls[0]).toMatchObject({
      method: "POST",
      url: "https://api.vercel.com/v2/domains/trydimmi.com/records",
      body: { type: "MX", name: "inbound.staging", value: "m.example", ttl: 300, comment: "c", mxPriority: 10 },
    });
  });

  it("writes the apex as an empty name and deletes by id", async () => {
    const api = fakeApi([{}, {}]);
    const vercel = new Vercel({ token: "t", domain: "trydimmi.com", fetch: api.fetch });

    await vercel.update("rec_1", { type: "A", name: "trydimmi.com", value: "192.0.2.1", ttl: 300 }, "c");
    await vercel.remove("rec_2");

    expect(api.calls[0]).toMatchObject({ method: "PATCH", body: { name: "" } });
    expect(api.calls[1]).toMatchObject({
      method: "DELETE",
      url: "https://api.vercel.com/v2/domains/trydimmi.com/records/rec_2",
    });
  });

  it("reports the status and body of a refused call", async () => {
    const fetch: Fetch = async () => ({
      ok: false,
      status: 403,
      json: async () => ({}),
      text: async () => "forbidden",
    });
    const vercel = new Vercel({ token: "t", domain: "trydimmi.com", fetch });

    await expect(vercel.list()).rejects.toThrow(/answered 403: forbidden/);
  });
});
