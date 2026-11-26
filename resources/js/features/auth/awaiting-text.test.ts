import { describe, expect, it } from "vitest";

import { awaitingLine } from "./awaiting-text";

describe("awaiting access", () => {
  it("names the workspaces the person waits in", () => {
    expect(awaitingLine([{ id: "ws_1", name: "Acme Support" }])).toBe(
      "You're set up in Acme Support. Its admins know you're here. We'll email you the moment you're in.",
    );
    expect(
      awaitingLine([
        { id: "ws_1", name: "Acme" },
        { id: "ws_2", name: "Beta" },
        { id: "ws_3", name: "Core" },
      ]),
    ).toContain("set up in Acme, Beta and Core.");
  });
});
