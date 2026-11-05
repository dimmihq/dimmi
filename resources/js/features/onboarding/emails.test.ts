import { describe, expect, it } from "vitest";

import { emailsIn } from "./emails";

describe("emailsIn", () => {
  it("splits on commas, semicolons and new lines and drops duplicates", () => {
    expect(emailsIn("grace@acme.com, alan@acme.com;\nGrace@acme.com  ")).toEqual(["grace@acme.com", "alan@acme.com"]);
  });

  it("finds nothing in blank text", () => {
    expect(emailsIn(" ,\n ")).toEqual([]);
  });
});
