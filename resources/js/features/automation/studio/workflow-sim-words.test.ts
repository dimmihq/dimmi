import { describe, expect, it } from "vitest";

import type { WorkflowSimResult } from "@/lib/api/workflow-simulations";

import { noSources } from "../procedure-writer/procedure-suggestions";
import {
  checkSentence,
  lined,
  mockableSteps,
  mockDrafts,
  mocksOf,
  type Steps,
  simStanding,
  stepNote,
} from "./workflow-sim-words";

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

  it("says what a wait, an extract, an action and a call did", () => {
    expect(stepNote({ step_id: "w", kind: "wait", outcome: "interrupted", detail: { message: "Any news?" } })).toBe(
      "The customer wrote first: “Any news?”",
    );
    expect(stepNote({ step_id: "w", kind: "wait", outcome: "elapsed", skipped: true })).toBe("The time passed");
    expect(
      stepNote({
        step_id: "x",
        kind: "extract",
        outcome: "none",
        detail: { values: { "run.vars.order": "A-1" }, unsure: ["refund"], mocked: true },
      }),
    ).toBe("Found order: A-1 · Not sure of refund (mocked)");
    expect(stepNote({ step_id: "a", kind: "run_action", outcome: "failed", detail: { status: 503 } })).toBe(
      "The action answered 503",
    );
    expect(
      stepNote({
        step_id: "a",
        kind: "run_action",
        outcome: "succeeded",
        detail: { saved: { "run.vars.plan": "pro" } },
      }),
    ).toBe("Saved plan: pro");
    expect(
      stepNote({ step_id: "c", kind: "run_workflow", outcome: "completed", detail: { workflow: "Ask plan" } }),
    ).toBe("Ran “Ask plan”, then came back");
    expect(stepNote({ step_id: "c", kind: "run_workflow", outcome: "depth_limit" })).toContain("five deep");
  });

  it("lists the steps a mock can answer for, nested ones too", () => {
    const list: Steps = [
      {
        id: "s_ask",
        kind: "buttons",
        settings: {},
        paths: [{ id: "p1", label: "A", steps: [{ id: "s_act", kind: "run_action", settings: {} }] }],
      },
      { id: "s_ext", kind: "extract", settings: {} },
      { id: "s_tag", kind: "tag", settings: {} },
    ];
    expect(mockableSteps(list)).toEqual([
      { id: "s_act", kind: "run_action" },
      { id: "s_ext", kind: "extract" },
    ]);
  });

  it("edits mocks over the saved ones and holds back what does not read", () => {
    const saved = { s_cls: { body: { path_id: "p1" } }, s_act: { status: 200, body: { plan: "pro" } } };
    const drafts = mockDrafts(saved, ["s_act", "s_ext"]);
    expect(drafts.s_act).toEqual({ status: "200", body: '{\n  "plan": "pro"\n}' });
    expect(drafts.s_ext).toEqual({ status: "", body: "" });

    const edited = mocksOf(saved, {
      ...drafts,
      s_act: { status: "503", body: "" },
      s_ext: { status: "", body: '{"a":1}' },
    });
    expect(edited.bad).toEqual([]);
    expect(edited.mocks).toEqual({
      s_cls: { body: { path_id: "p1" } },
      s_act: { status: 503 },
      s_ext: { body: { a: 1 } },
    });

    expect(mocksOf(saved, { s_act: { status: "", body: "" } }).mocks).toEqual({ s_cls: { body: { path_id: "p1" } } });
    expect(mocksOf(saved, { s_ext: { status: "", body: "{oops" } }).bad).toEqual(["s_ext"]);
    expect(mocksOf(saved, { s_ext: { status: "abc", body: "" } }).bad).toEqual(["s_ext"]);
  });
});
