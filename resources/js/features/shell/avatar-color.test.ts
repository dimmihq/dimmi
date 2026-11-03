import { avatarColor } from "./avatar-color";

describe("avatarColor", () => {
  it("gives the same name the same tint every time", () => {
    expect(avatarColor("Emily Johnson")).toBe(avatarColor("Emily Johnson"));
  });

  it("spreads different names across the tints", () => {
    const tints = new Set(
      ["Emily Johnson", "Sam Okafor", "Lea Martin", "Ada Lovelace", "Noor Haddad"].map(avatarColor),
    );

    expect(tints.size).toBeGreaterThan(1);
  });
});
