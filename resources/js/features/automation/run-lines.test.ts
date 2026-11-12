import type { RunStep } from "@/lib/api/workflows";

import { detailLine } from "./run-lines";

function done(kind: string, detail: Record<string, unknown>): RunStep {
  return {
    step_id: "s_1",
    kind,
    status: "completed",
    outcome: null,
    path_id: null,
    detail,
    started_at: "2026-11-12T10:00:00Z",
    finished_at: null,
  };
}

describe("detailLine", () => {
  it("says how a call was answered and what it kept", () => {
    expect(detailLine(done("http_request", { status: 200, saved: ["run.vars.plan"] }))).toBe(
      "Answered 200 · saved run.vars.plan",
    );
    expect(detailLine(done("http_request", { error: "timed out" }))).toBe("timed out");
  });

  it("says which category a conversation was sorted into", () => {
    expect(detailLine(done("classify", { category: "Billing", confidence: "high" }))).toBe(
      "Sorted into Billing (high confidence)",
    );
  });

  it("names the workflow that ran and how it ended", () => {
    expect(
      detailLine(
        done("run_workflow", { workflow_id: "wf_1", child_status: "completed" }),
        new Map([["wf_1", "Refund check"]]),
      ),
    ).toBe("Ran Refund check · completed");
  });

  it("quotes the reply time it gave", () => {
    expect(detailLine(done("reply_time", { reply_time: "within an hour", open: true }))).toBe(
      "Said “within an hour”, during office hours",
    );
  });

  it("says nothing for kinds without a detail worth reading", () => {
    expect(detailLine(done("tag", {}))).toBe("");
  });
});
