import { describe, expect, it } from "vitest";

import type { WorkflowSimResult } from "@/lib/api/workflow-simulations";

import { noSources } from "../procedure-writer/procedure-suggestions";
import { checkSentence, lined, type Steps, simStanding } from "./workflow-sim-words";

const steps: Steps = [
  {
    id: "s_ask0001",
    kind: "buttons",
    settings: { text: "What is it about?" },
    paths: [
      { id: "p_billing", label: "Billing", steps: [{ id: "s_tag0001", kind: "tag", settings: { tag_ids: [] } }] },
      { id: "p_timeout", outcome: "timeout", steps: [] },
    ],
  },
];

function ran(part: Partial<WorkflowSimResult>): WorkflowSimResult {
  return {
    passed: true,
    ran_at: "2026-11-24T10:00:00Z",
    version: "draft",
    transcript: [],
    turns: [],
    assertion_results: [],
    ...part,
  };
}

describe("workflow simulation words", () => {
  it("reads the workflow's own checks by the names the builder gives steps and paths", () => {
    expect(checkSentence({ type: "workflow_started" }, steps, noSources)).toBe("The workflow starts");
    expect(checkSentence({ type: "path_taken", step_id: "s_ask0001", branch: "p_billing" }, steps, noSources)).toBe(
      "“Reply buttons” takes the path Billing",
    );
    expect(checkSentence({ type: "step_reached", step_id: "s_gone" }, steps, noSources)).toBe(
      "The workflow reaches s_gone",
    );
    expect(checkSentence({ type: "action_called", action: "s_tag0001" }, steps, noSources)).toBe("“Tag” runs");
    expect(checkSentence({ type: "reply_contains", text: "billing" }, steps, noSources)).toBe(
      "The reply says “billing”",
    );
  });

  it("stands on the latest runs, and on the run itself when it could not finish", () => {
    const history = { state: "flaky" as const, passed: 1, total: 2, window: 5, runs: [] };
    expect(simStanding(ran({}), history).label).toBe("Flaky · 1 of 2");
    expect(simStanding(ran({ error: "No customer" }), history).label).toBe("Error");
    expect(simStanding(ran({}), null).label).toBe("Not run");
  });

  it("lines results up with the checks only while they still match", () => {
    const result = ran({ assertion_results: [{ type: "workflow_started", passed: true, message: "" }] });
    expect(lined([{ type: "workflow_started" }], result)).toHaveLength(1);
    expect(lined([], result)).toHaveLength(0);
  });
});
