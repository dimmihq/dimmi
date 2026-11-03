import { fragment, safeRedirect } from "./location";

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
