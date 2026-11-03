import { ApiError } from "@/lib/api/client";

import { fieldError } from "./form-errors";

const fields = ["email", "password"];

describe("fieldError", () => {
  it("shows a field's own message under it", () => {
    const error = new ApiError(422, "validation_failed", "Invalid.", { password: "Too short." });

    expect(fieldError(error, "password", fields, "email")).toBe("Too short.");
    expect(fieldError(error, "email", fields, "email")).toBeUndefined();
  });

  it("shows a failure that names no field of the form under the main field only", () => {
    const error = new ApiError(429, "too_many_attempts", "Too many attempts.", { token: "Ignored." });

    expect(fieldError(error, "email", fields, "email")).toBe("Too many attempts.");
    expect(fieldError(error, "password", fields, "email")).toBeUndefined();
  });

  it("says the server could not be reached when the request never got an answer", () => {
    expect(fieldError(new TypeError("Failed to fetch"), "email", fields, "email")).toContain("could not be reached");
    expect(fieldError(new TypeError("Failed to fetch"), "password", fields, "email")).toBeUndefined();
  });

  it("shows nothing without an error", () => {
    expect(fieldError(null, "email", fields, "email")).toBeUndefined();
  });
});
