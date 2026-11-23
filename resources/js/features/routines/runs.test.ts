import { describe, expect, it } from "vitest";

import type { RoutineRun } from "@/lib/api/routines";

import { lastLine, readLine, shownRuns, tookOf } from "./runs";

function run(over: Partial<RoutineRun>): RoutineRun {
  return {
    id: "orr_1",
    trigger: "scheduled",
    status: "completed",
    outcome: "findings",
    changesets_count: 3,
    suggestions_waiting: 0,
    read: { articles: 0, conversations: 0, procedures: 0 },
    ...over,
  } as RoutineRun;
}

describe("tookOf", () => {
  it("says how long a finished run took", () => {
    expect(tookOf({ started_at: "2026-11-23T09:00:00Z", completed_at: "2026-11-23T09:00:42Z" })).toBe("took 42 s");
    expect(tookOf({ started_at: "2026-11-23T09:00:00Z", completed_at: "2026-11-23T09:01:12Z" })).toBe(
      "took 1 min 12 s",
    );
    expect(tookOf({ started_at: "2026-11-23T09:00:00Z", completed_at: "2026-11-23T09:02:00Z" })).toBe("took 2 min");
  });

  it("says nothing while a run has not finished", () => {
    expect(tookOf({ started_at: "2026-11-23T09:00:00Z", completed_at: null })).toBe("");
  });
});

describe("readLine", () => {
  it("says how much a run read", () => {
    expect(readLine({ read: { articles: 100, conversations: 1, procedures: 0 } })).toBe(
      "read 100 articles and 1 conversation",
    );
    expect(readLine({ read: { articles: 0, conversations: 0, procedures: 0 } })).toBe("");
  });
});

describe("shownRuns", () => {
  const runs = [
    run({ id: "a" }),
    run({ id: "b", trigger: "dry_run" }),
    run({ id: "c", outcome: "noop" }),
    run({ id: "d", status: "failed", outcome: null }),
    run({ id: "e", trigger: "dry_run", status: "running", outcome: null }),
  ];

  it("leaves finished dry runs out until asked for", () => {
    expect(shownRuns(runs, "all", false).map((r) => r.id)).toEqual(["a", "c", "d", "e"]);
    expect(shownRuns(runs, "all", true)).toHaveLength(5);
  });

  it("keeps the runs that ended one way, and any still going", () => {
    expect(shownRuns(runs, "suggested", true).map((r) => r.id)).toEqual(["a", "b", "e"]);
    expect(shownRuns(runs, "failed", false).map((r) => r.id)).toEqual(["d", "e"]);
  });
});

describe("lastLine", () => {
  it("says what the last real run came to and what still waits", () => {
    expect(lastLine([run({ trigger: "dry_run" }), run({ suggestions_waiting: 2 })])).toBe(
      "The last run suggested 3 changes, 2 waiting for review.",
    );
    expect(lastLine([run({})])).toBe("The last run suggested 3 changes, all reviewed.");
    expect(lastLine([run({ trigger: "dry_run" })])).toBe("");
  });
});
