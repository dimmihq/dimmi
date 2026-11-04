import { describe, expect, it } from "vitest";

import type { MemberImport } from "@/lib/api/workspace";

import { hasWork, problems, summary } from "./member-import";

const report: MemberImport = {
  dry_run: true,
  created: 1,
  updated: 2,
  unchanged: 3,
  failed: 1,
  rows: [
    { row: 2, email: "ada@example.com", result: "updated", changes: ["role"] },
    {
      row: 3,
      email: "",
      result: "failed",
      changes: [],
      error: { code: "invalid_email", message: "The email must be a valid email address." },
    },
  ],
};

describe("member import report", () => {
  it("says what a dry run would do, and what a run did", () => {
    expect(summary(report)).toBe("This file has 1 to invite, 2 to change, 3 unchanged, 1 with problems.");
    expect(summary({ ...report, dry_run: false })).toBe("Done: 1 invited, 2 changed, 3 unchanged, 1 with problems.");
  });

  it("lists the rows with problems", () => {
    expect(problems(report)).toEqual(["Row 3, no address: The email must be a valid email address."]);
  });

  it("knows whether applying would change anything", () => {
    expect(hasWork(report)).toBe(true);
    expect(hasWork({ ...report, created: 0, updated: 0 })).toBe(false);
  });
});
