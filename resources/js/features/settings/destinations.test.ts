import { describe, expect, it } from "vitest";

import type { NotificationDestination } from "@/lib/api/notification-destinations";

import { destinationLine, stopped, toggled } from "./destinations";

const destination = {
  status: "active",
  triggers: ["new_conversation"],
  digests: ["daily_summary"],
  last_error: null,
} as unknown as NotificationDestination;

describe("destinationLine", () => {
  it("lists what a working destination hears about", () => {
    expect(destinationLine(destination)).toBe("New conversations · Daily summary");
    expect(destinationLine({ ...destination, triggers: [], digests: [] })).toBe("Hears about nothing yet.");
  });

  it("says why a destination stopped", () => {
    expect(stopped({ ...destination, status: "error", last_error: "The app was removed from the channel." })).toBe(
      "Stopped: The app was removed from the channel.",
    );
    expect(destinationLine({ ...destination, status: "disabled" })).toBe("Turned off.");
    expect(stopped(destination)).toBeNull();
  });
});

describe("toggled", () => {
  it("adds and removes a value once", () => {
    expect(toggled(["a"], "b", true)).toEqual(["a", "b"]);
    expect(toggled(["a", "b"], "a", false)).toEqual(["b"]);
    expect(toggled(["a"], "a", true)).toEqual(["a"]);
  });
});
