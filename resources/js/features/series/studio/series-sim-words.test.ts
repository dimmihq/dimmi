import { describe, expect, it } from "vitest";

import type { SeriesSimulation } from "@/lib/api/workflow-simulations";

import { didWords, journey, mockableNodes, walkedNote } from "./series-sim-words";

describe("series simulation words", () => {
  it("says what extract and action steps kept", () => {
    expect(
      walkedNote({
        node_id: "fit",
        kind: "extract",
        result: "extract:ok",
        detail: { values: { fit: "team" }, unsure: [], mocked: true },
      }),
    ).toBe("Found fit: team (mocked)");
    expect(
      walkedNote({ node_id: "seats", kind: "run_action", result: "action:ok", detail: { values: { seats: 12 } } }),
    ).toBe("Saved seats: 12");
    expect(walkedNote({ node_id: "seats", kind: "run_action", result: "action:failed", detail: { status: 500 } })).toBe(
      "The action answered 500",
    );
    expect(walkedNote({ node_id: "w", kind: "wait", result: "waited" })).toBe("");
  });

  it("lists the steps whose answers can be set", () => {
    expect(
      mockableNodes([
        { id: "entry", kind: "entry" },
        { id: "fit", kind: "extract" },
        { id: "seats", kind: "run_action" },
      ]),
    ).toEqual([
      { id: "fit", kind: "extract" },
      { id: "seats", kind: "run_action" },
    ]);
  });

  it("says what the person did or did not do", () => {
    expect(didWords("Ada", 'open the message "Welcome"', true)).toBe('Ada opened the message "Welcome"');
    expect(didWords("Ada", 'click a link in the message "Pricing"', false)).toBe(
      'Ada did not click a link in the message "Pricing"',
    );
    expect(didWords("Ada", 'do the event "upgraded" in the product', true)).toBe(
      'Ada did the event "upgraded" in the product',
    );
  });

  it("reads a walk as messages, what they did, and what the series did", () => {
    const result: SeriesSimulation = {
      object: "series_simulation",
      entered: true,
      steps: [
        { node_id: "entry", kind: "entry", result: "entered" },
        { node_id: "welcome", kind: "email", result: "sent" },
        { node_id: "wait", kind: "wait", result: "waited", skipped: true },
        { node_id: "check", kind: "condition", path: "else", result: "else" },
      ],
      messages: [{ node_id: "welcome", kind: "email", subject: "Welcome", body: "Hi" }],
      actions: [],
      goal_reached: false,
      end: "finished",
      decisions: [{ before: 3, what: 'open the message "Welcome"', did: false }],
    };

    expect(journey(result, "Ada").map((m) => (m.kind === "message" ? `mail:${m.subject}` : m.text))).toEqual([
      "mail:Welcome",
      "Wait, skipped",
      'Ada did not open the message "Welcome"',
      "Check: everyone else",
      "Finished",
    ]);
  });
});
