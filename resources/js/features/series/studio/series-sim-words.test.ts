import { describe, expect, it } from "vitest";

import { mockableNodes, walkedNote } from "./series-sim-words";

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
});
