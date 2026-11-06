import { describe, expect, it } from "vitest";

import type { DataExport } from "@/lib/api/data-exports";

import { building, exportDetail, statusNames } from "./export-format";

const base: DataExport = {
  id: "dex_1",
  object: "data_export",
  kind: "conversations",
  format: "csv",
  status: "pending",
  include_notes: false,
  row_count: null,
  warnings: [],
  file_name: "export.csv",
  size_bytes: null,
  error: null,
  expires_at: null,
  url: null,
  created_at: "2026-11-05T10:00:00Z",
};

describe("export format", () => {
  it("names what an export covers and how many rows it holds", () => {
    expect(exportDetail(base)).toMatch(/^Spreadsheet of conversations · /);
    expect(exportDetail({ ...base, kind: "contacts", format: "json", row_count: 1 })).toMatch(
      /^Data of customers, 1 row · /,
    );
    expect(exportDetail({ ...base, format: "txt", row_count: 3 })).toMatch(/^Transcripts of conversations, 3 rows/);
  });

  it("says until when a ready export downloads", () => {
    const ready = { ...base, status: "ready" as const, expires_at: "2026-11-12T10:00:00Z" };

    expect(exportDetail(ready)).toContain(" · until ");
    expect(exportDetail(base)).not.toContain("until");
  });

  it("knows when an export is still being built", () => {
    expect(building([base])).toBe(true);
    expect(building([{ ...base, status: "ready" }])).toBe(false);
    expect(building(undefined)).toBe(false);
    expect(statusNames.running).toBe("Preparing");
  });
});
