import { describe, expect, it } from "vitest";

import { ApiError } from "@/lib/api/client";

import { problemsOf, sameJSON } from "./use-audience-draft";

describe("sameJSON", () => {
  it("compares values whatever their key order", () => {
    expect(
      sameJSON(
        { visitors: "everyone", rules: { op: "eq", field: "x" } },
        { rules: { field: "x", op: "eq" }, visitors: "everyone" },
      ),
    ).toBe(true);
    expect(sameJSON({ a: [1, 2] }, { a: [2, 1] })).toBe(false);
  });
});

describe("problemsOf", () => {
  it("files refused conditions by path and keeps the rest apart", () => {
    const error = new ApiError(
      422,
      "audience_invalid",
      "The rule cannot be used as it is.",
      {},
      {
        errors: [
          { code: "value_invalid", path: "rules.all[0]", message: "Enter a number." },
          { code: "audience_cycle", path: "rules", message: "Loop." },
        ],
      },
    );

    expect(problemsOf(error)).toEqual({ byPath: { "rules.all[0]": "Enter a number." }, rest: ["Loop."] });
    expect(problemsOf(new Error("Offline"))).toEqual({ byPath: {}, rest: ["Offline"] });
    expect(problemsOf(null)).toEqual({ byPath: {}, rest: [] });
  });
});
