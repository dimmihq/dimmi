import { describe, expect, it } from "vitest";

import { storedMethods } from "./methods";

describe("storedMethods", () => {
  it("stores no rule when everything is ticked", () => {
    expect(storedMethods(["password", "google"], ["google", "password"])).toEqual([]);
  });

  it("stores the ticked ways otherwise", () => {
    expect(storedMethods(["google"], ["google", "password"])).toEqual(["google"]);
  });
});
