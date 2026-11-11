import { describe, expect, it } from "vitest";

import type { KnowledgeSource, SourceSchema, SyncRun } from "@/lib/api/knowledge-sources";

import { ago, broken, initialValues, missing, nameFor, patterns, runSummary, settingsOf, sourceState } from "./model";

const schema: SourceSchema = {
  type: "object",
  required: ["start_url", "access_token"],
  properties: {
    start_url: { type: "string", title: "Address" },
    access_token: { type: "string", title: "Token", "x-secret": true },
    max_pages: { type: "integer", title: "Pages", default: 500 },
    follow_links: { type: "boolean", title: "Follow", default: true },
    paths: { type: "array", title: "Files", items: { type: "string" } },
  },
};

function source(overrides: Partial<KnowledgeSource>): KnowledgeSource {
  return {
    id: "ks_1",
    type: "website",
    name: "Docs",
    mode: "sync",
    targets: ["agent"],
    help_center_id: null,
    settings: {},
    secrets_set: [],
    include: [],
    exclude: [],
    status: "idle",
    schedule: "weekly",
    next_sync_at: null,
    last_synced_at: "2026-11-01T10:00:00Z",
    last_success_at: null,
    last_error_code: null,
    last_error: null,
    document_count: 3,
    counts: {},
    last_run: null,
    created_at: "2026-11-01T10:00:00Z",
    ...overrides,
  };
}

describe("connect form values", () => {
  it("fills defaults and keeps secrets empty", () => {
    expect(initialValues(schema, { access_token: "x", start_url: "https://a.test" })).toEqual({
      start_url: "https://a.test",
      access_token: "",
      max_pages: 500,
      follow_links: true,
      paths: [],
    });
  });

  it("counts a stored secret as given", () => {
    const values = { start_url: "https://a.test", access_token: "" };
    expect(missing(schema, values)).toEqual(["access_token"]);
    expect(missing(schema, values, ["access_token"])).toEqual([]);
  });

  it("sends numbers as numbers and leaves blank secrets out", () => {
    expect(settingsOf(schema, { start_url: " https://a.test ", access_token: "", max_pages: "20" })).toEqual({
      start_url: "https://a.test",
      max_pages: 20,
    });
  });

  it("reads one pattern per line and names a source after its address", () => {
    expect(patterns("/docs/**\n\n  /blog/** ")).toEqual(["/docs/**", "/blog/**"]);
    expect(nameFor("Website", { start_url: "https://docs.acme.com/" })).toBe("docs.acme.com");
    expect(nameFor("Notion", {})).toBe("Notion");
  });
});

describe("source state", () => {
  it("says how a source stands", () => {
    expect(sourceState(source({ status: "syncing" })).label).toBe("Syncing");
    expect(sourceState(source({ status: "error" })).tone).toBe("red");
    expect(sourceState(source({ last_synced_at: null })).label).toBe("Not synced yet");
    expect(sourceState(source({})).label).toBe("Live");
    expect(broken(source({ last_run: { status: "partial" } as SyncRun }))).toBe(true);
  });

  it("sums up a run and how long ago it was", () => {
    const run = { stats: { created: 3, unchanged: 0, removed: 1 } } as unknown as SyncRun;
    expect(runSummary(run)).toBe("3 new · 1 removed");
    expect(ago("2026-11-01T10:00:00Z", Date.parse("2026-11-01T12:00:00Z"))).toBe("2 hours ago");
    expect(ago(null)).toBe("Never");
  });
});
