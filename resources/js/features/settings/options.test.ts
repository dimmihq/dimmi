import { describe, expect, it } from "vitest";

import { optionList } from "./options";

describe("optionList", () => {
  it("trims, drops blanks and repeats, and keeps the order", () => {
    expect(optionList([" Billing ", "", "Bugs", "Billing", "  "])).toEqual(["Billing", "Bugs"]);
  });
});
