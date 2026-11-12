import { describe, expect, it } from "vitest";

import { deliveryOutcome } from "./delivery-format";

describe("deliveryOutcome", () => {
  it("names a sent delivery plainly", () => {
    expect(deliveryOutcome({ status: "sent", reason: null })).toBe("Sent");
  });

  it("says why a delivery was held back", () => {
    expect(deliveryOutcome({ status: "skipped", reason: "quiet_hours" })).toBe("Skipped: held during quiet hours");
  });

  it("reads an unknown reason as its words", () => {
    expect(deliveryOutcome({ status: "failed", reason: "mailbox_full" })).toBe("Failed: mailbox full");
  });
});
