import { describe, expect, it } from "vitest";

import { ApiError } from "@/lib/api/client";

import { mergedTarget } from "./use-follow-merge";

describe("mergedTarget", () => {
  it("reads the target of a merged conversation", () => {
    const error = new ApiError(404, "merged_into", "Merged.", {}, { target_id: "conv_1", target_number: 12 });
    expect(mergedTarget(error)).toEqual({ id: "conv_1", number: 12 });
  });

  it("ignores any other error", () => {
    expect(mergedTarget(new ApiError(404, "not_found", "Not Found."))).toBeNull();
    expect(mergedTarget(new Error("boom"))).toBeNull();
  });
});
