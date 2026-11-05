import { describe, expect, it } from "vitest";

import { destination } from "./access-destination";

describe("destination", () => {
  it("comes back to the page on screen after signing in again", () => {
    expect(destination("/login?reauth=1", "/d/inbox?x=1")).toBe("/login?reauth=1&redirect=%2Fd%2Finbox%3Fx%3D1");
  });

  it("adds the first query when the sign-in has none", () => {
    expect(destination("/login", "/d")).toBe("/login?redirect=%2Fd");
  });

  it("leaves any other page as it is", () => {
    expect(destination("/d/settings/security", "/d")).toBe("/d/settings/security");
  });
});
