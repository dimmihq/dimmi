import { describe, expect, it } from "vitest";

import { merged } from "./merged-steps";

describe("merged", () => {
  it("folds a run of identical lines into one with a count", () => {
    const wait = { tool: "update_procedure_status", summary: "Step 3.2 waits for the customer's reply", ok: true };
    const search = { tool: "search_knowledge", summary: "Searched the help center", ok: true };
    expect(merged([search, wait, wait, wait, search, wait])).toEqual([
      search,
      { ...wait, summary: "Step 3.2 waits for the customer's reply ×3" },
      search,
      wait,
    ]);
  });
});
