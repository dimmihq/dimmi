import { describe, expect, it } from "vitest";

import { neighbours } from "./use-walk";

describe("neighbours", () => {
  const rows = [{ id: "a" }, { id: "b" }, { id: "c" }];

  it("finds the rows on either side", () => {
    expect(neighbours(rows, "b")).toEqual({ previous: "a", next: "c" });
    expect(neighbours(rows, "a")).toEqual({ previous: undefined, next: "b" });
  });

  it("finds nothing for a conversation outside the list", () => {
    expect(neighbours(rows, "z")).toEqual({ previous: undefined, next: undefined });
    expect(neighbours(undefined, "a")).toEqual({ previous: undefined, next: undefined });
  });
});
