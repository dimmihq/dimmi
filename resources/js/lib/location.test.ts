import { displayZone, fragment, safeRedirect, shouldOfferZone, zoneChoices } from "./location";

describe("safeRedirect", () => {
  it("keeps a path on this site", () => {
    expect(safeRedirect("/d/inbox?view=mine")).toBe("/d/inbox?view=mine");
  });

  it("falls back for another site, a protocol-relative path, a backslash or no path", () => {
    for (const path of ["https://evil.example", "//evil.example", "/\\evil.example", "/x\nLocation: y", undefined, 3]) {
      expect(safeRedirect(path)).toBe("/d");
    }
  });
});

describe("fragment", () => {
  it("reads a named value after the hash, and nothing for a missing one", () => {
    expect(fragment("token", "#id=prs_1&token=abc")).toBe("abc");
    expect(fragment("id", "#id=prs_1&token=abc")).toBe("prs_1");
    expect(fragment("code", "#id=prs_1")).toBe("");
  });
});

describe("displayZone", () => {
  it("uses the person's own zone and otherwise leaves it to the browser", () => {
    expect(displayZone({ user: { timezone: "Asia/Tokyo" } })).toBe("Asia/Tokyo");
    expect(displayZone({ user: { timezone: null } })).toBeUndefined();
    expect(displayZone(null)).toBeUndefined();
  });
});

describe("shouldOfferZone", () => {
  it("offers the browser's zone only to someone following a different workspace zone", () => {
    expect(shouldOfferZone(null, "UTC", "Europe/Lisbon")).toBe(true);
    expect(shouldOfferZone(null, "UTC", "UTC")).toBe(false);
    expect(shouldOfferZone("Asia/Tokyo", "UTC", "Europe/Lisbon")).toBe(false);
  });
});

describe("zoneChoices", () => {
  it("always holds UTC and the person's current zone, once each", () => {
    expect(zoneChoices("Mars/Base", ["Asia/Tokyo", "UTC"])).toEqual(["UTC", "Asia/Tokyo", "Mars/Base"]);
    expect(zoneChoices(null, ["Asia/Tokyo"])).toEqual(["UTC", "Asia/Tokyo"]);
  });
});
