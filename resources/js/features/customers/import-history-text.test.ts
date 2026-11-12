import { describe, expect, it } from "vitest";

import type { ImportSummary } from "@/lib/api/imports";

import { busy, canUndo, countsLine, keptMore, statusLabel, undoneLine, undoPlan } from "./import-history-text";

const now = new Date("2026-11-12T12:00:00Z");

const base: ImportSummary = {
  id: "imp_1",
  kind: "contacts",
  status: "completed",
  file_name: "people.csv",
  row_count: 4,
  processed_rows: 4,
  created_count: 2,
  updated_count: 1,
  skipped_count: 0,
  failed_count: 1,
  finished_at: "2026-11-10T12:00:00Z",
  created_at: "2026-11-10T11:59:00Z",
  created_by_name: "Ana",
  error_report_url: "/api/v1/imports/imp_1/errors",
  rollback_until: "2026-11-17T12:00:00Z",
  rollback: null,
  file_expired_at: null,
};

const rollback = {
  status: "completed" as const,
  total: 3,
  processed: 3,
  removed: 1,
  reverted: 1,
  kept: 1,
  kept_records: [{ id: "ctc_1", name: "Eve", reason: "Changed since the import." }],
  error: null,
  requested_at: null,
  finished_at: null,
};

describe("import history", () => {
  it("offers an undo only for a finished import inside its week that was not undone", () => {
    expect(canUndo(base, now)).toBe(true);
    expect(canUndo({ ...base, status: "running" }, now)).toBe(false);
    expect(canUndo({ ...base, rollback_until: "2026-11-11T12:00:00Z" }, now)).toBe(false);
    expect(canUndo({ ...base, rollback }, now)).toBe(false);
    expect(canUndo({ ...base, rollback: { ...rollback, status: "failed" } }, now)).toBe(true);
  });

  it("names where an import and its undo stand", () => {
    expect(statusLabel(base)).toBe("Finished");
    expect(statusLabel({ ...base, rollback: { ...rollback, status: "running" } })).toBe("Undoing");
    expect(statusLabel({ ...base, rollback })).toBe("Undone");
    expect(busy({ ...base, rollback: { ...rollback, status: "queued" } })).toBe(true);
    expect(busy(base)).toBe(false);
  });

  it("says what an import and its undo came to", () => {
    expect(countsLine(base)).toBe("2 new · 1 updated · 1 with problems");
    expect(undoneLine({ ...base, rollback })).toBe("1 removed, 1 put back, 1 kept");
    expect(keptMore({ ...base, rollback: { ...rollback, kept: 8 } }, 5)).toBe("and 7 more");
    expect(keptMore({ ...base, rollback }, 5)).toBe("");
  });

  it("spells out what undoing removes and puts back", () => {
    const plan = undoPlan(base, "people");
    expect(plan[0]).toContain("Removes the 2 people it added");
    expect(plan[0]).toContain("talked to");
    expect(plan[1]).toContain("Puts back the previous values on the 1 people it changed");
    expect(undoPlan({ ...base, created_count: 0, updated_count: 0 }, "people")[0]).toContain("tags");
  });
});
