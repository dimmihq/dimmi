import { describe, expect, it } from "vitest";

import type { Simulation } from "@/lib/api/procedures";

import { defaultAssertions } from "./simulator";

function turn(part: Partial<Simulation>): Simulation {
  return {
    object: "procedure_simulation",
    run_id: "air_1",
    reply: "",
    outcome: "answered",
    matched: true,
    handoff: null,
    active_state: { called_action_ids: [], completed_step_ids: [], step_outputs: {}, pending: [] },
    action_calls: [],
    ...part,
  } as Simulation;
}

describe("defaultAssertions", () => {
  it("keeps the path the chat took and the team it went to", () => {
    const checks = defaultAssertions([
      turn({
        procedure_trace: {
          procedure_id: "prc_1",
          procedure_name: "Rates",
          version: 1,
          status: "active",
          retries: 0,
          guard_events: [],
          steps: [
            { step_id: "enterprise", kind: "branch", status: "evaluated", branch_taken: "else" },
            { step_id: "volume", kind: "branch", status: "evaluated", branch_taken: "then" },
          ],
        },
      }),
      turn({
        handoff: { reason: "other", team_id: "team_ent" },
        procedure_trace: {
          procedure_id: "prc_1",
          procedure_name: "Rates",
          version: 1,
          status: "handed_off",
          retries: 0,
          guard_events: [],
          steps: [{ step_id: "quote", kind: "condition", status: "done", branch_taken: "then" }],
        },
      }),
    ]);

    expect(checks).toEqual([
      { type: "procedure_matched" },
      { type: "branch_taken", step_id: "enterprise", branch: "else" },
      { type: "branch_taken", step_id: "volume", branch: "then" },
      { type: "branch_taken", step_id: "quote", branch: "then" },
      { type: "handed_off", handed_off: true, team_id: "team_ent" },
    ]);
  });

  it("says the chat was not handed over when nobody was handed it", () => {
    expect(defaultAssertions([turn({})])).toEqual([
      { type: "procedure_matched" },
      { type: "handed_off", handed_off: false },
    ]);
  });
});
