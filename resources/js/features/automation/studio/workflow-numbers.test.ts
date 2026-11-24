import { describe, expect, it } from "vitest";

import type { Spec } from "@/lib/api/workflows";

import { daysBack, goalWords, lift, placeSteps, rate, stepLabel, workflowChanges } from "./workflow-numbers";

const base: Spec = {
  schema: 1,
  trigger: "conversation.first_message",
  schedule: { mode: "any" },
  reentry: { mode: "every_time" },
  exclusive: false,
  steps: [
    {
      id: "s_ask",
      kind: "buttons",
      settings: { text: "What is it about?" },
      paths: [
        { id: "p_billing", label: "Billing", steps: [{ id: "s_assign", kind: "assign", settings: { mode: "team" } }] },
        { id: "p_other", label: "Something else", steps: [{ id: "s_msg", kind: "message", settings: { text: "Ok" } }] },
      ],
    },
    { id: "s_close", kind: "close" },
  ],
};

describe("placeSteps", () => {
  it("numbers steps in reading order, paths by letter", () => {
    expect(placeSteps(base.steps).map((p) => [p.step.id, p.number, p.under])).toEqual([
      ["s_ask", "1", null],
      ["s_assign", "1a.1", "Step 1, Billing"],
      ["s_msg", "1b.1", "Step 1, Something else"],
      ["s_close", "2", null],
    ]);
  });
});

describe("stepLabel", () => {
  it("reads a step by its kind and the start of its text", () => {
    expect(stepLabel(base.steps[0])).toBe("Reply buttons: “What is it about?”");
    expect(stepLabel(base.steps[1])).toBe("Close");
    expect(stepLabel(undefined, "http_request")).toBe("Call your system");
    expect(stepLabel(undefined)).toBe("A removed step");
  });
});

describe("rates", () => {
  it("reads a part of a total and a lift in points", () => {
    expect(rate(1, 3)).toBe("33%");
    expect(rate(1, 0)).toBe("–");
    expect(lift({ entered: 100, goal: 30 }, { entered: 20, goal: 4 })).toBe("+10 pts");
    expect(lift({ entered: 10, goal: 1 }, { entered: 10, goal: 2 })).toBe("-10 pts");
    expect(lift({ entered: 10, goal: 1 }, { entered: 0, goal: 0 })).toBe("–");
  });

  it("starts a range on the first of its days", () => {
    expect(daysBack(7, new Date(2026, 10, 24))).toBe("2026-11-18");
    expect(daysBack(1, new Date(2026, 0, 1))).toBe("2026-01-01");
  });

  it("says a goal in words", () => {
    expect(goalWords(null)).toBe("No goal");
    expect(goalWords({ kind: "resolved", window: "3d" })).toBe("Resolved without a handover within 3 days");
    expect(goalWords({ kind: "csat", window: "1d", min_rating: 4 })).toBe("Rated 4 or more within 1 day");
  });
});

describe("workflowChanges", () => {
  it("finds nothing between a copy and itself, whatever its key order", () => {
    const shuffled = JSON.parse(JSON.stringify(base)) as Spec;
    shuffled.steps[0].settings = { text: "What is it about?" };
    expect(workflowChanges(base, shuffled)).toEqual([]);
  });

  it("lists the rules, then steps added, moved, edited and removed", () => {
    const after: Spec = {
      ...base,
      reentry: { mode: "once_per_conversation" },
      goal: { kind: "resolved", window: "7d" },
      steps: [
        {
          ...base.steps[0],
          settings: { text: "What is it about?\nPick one." },
          paths: [
            { id: "p_billing", label: "Billing", steps: [] },
            {
              id: "p_other",
              label: "Something else",
              steps: [{ id: "s_msg", kind: "message", settings: { text: "Ok" } }],
            },
          ],
        },
        { id: "s_assign", kind: "assign", settings: { mode: "team" } },
        { id: "s_tag", kind: "tag" },
      ],
    };

    const changes = workflowChanges(base, after);
    expect(changes.map((c) => [c.key, c.mark])).toEqual([
      ["reentry", "changed"],
      ["goal", "changed"],
      ["s_ask:edit", "changed"],
      ["s_assign", "moved"],
      ["s_tag", "added"],
      ["s_close", "removed"],
    ]);
    expect(changes[1]).toMatchObject({ before: "No goal", after: "Resolved without a handover within 7 days" });
    expect(changes[2].lines?.filter((l) => l.mark !== "same")).toEqual([{ mark: "added", text: "Pick one." }]);
    expect(changes[3]).toMatchObject({ before: "Step 1, Billing", after: "The top" });
  });
});
