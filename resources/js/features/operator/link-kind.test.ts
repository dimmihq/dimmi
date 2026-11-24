import { describe, expect, it } from "vitest";

import { readLink } from "./link-kind";

const here = "http://localhost:8102";

describe("readLink", () => {
  it("opens dimmi's own pages in the app, whichever host the address names", () => {
    const link = readLink("https://app.example.test/d/acme/inbox/all/conv_01abc", here);

    expect(link).toEqual({ kind: "conversation", inApp: "/d/acme/inbox/all/conv_01abc", name: "Open conversation" });
    expect(readLink("/d/acme/help-center/art_01abc", here).kind).toBe("article");
    expect(readLink("/d/acme/changelog/cle_01abc", here).kind).toBe("changelog");
  });

  it("names a public help center page by its title and leaves the app for it", () => {
    const link = readLink("https://help.example.test/hc/acme/articles/plans-and-label-allowances", here);

    expect(link).toEqual({ kind: "public", inApp: null, name: "Plans and label allowances" });
    expect(readLink("https://help.example.test/feedback/12-dark-mode", here).name).toBe("Dark mode");
  });

  it("names any other link by its site", () => {
    expect(readLink("https://www.example.com/pricing", here)).toEqual({
      kind: "outside",
      inApp: null,
      name: "example.com",
    });
  });
});
