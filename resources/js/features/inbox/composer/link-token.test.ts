import { describe, expect, it } from "vitest";

import { kindOf, linkMarkdown } from "./link-token";

describe("help center links", () => {
  it("says what a link points at", () => {
    expect(kindOf("https://help.acme.test/articles/reset")).toBe("article");
    expect(kindOf("https://help.acme.test/feedback/12-dark-mode")).toBe("request");
    expect(kindOf("https://help.acme.test/changelog/june")).toBe("changelog");
  });

  it("keeps a chip as a plain Markdown link", () => {
    expect(linkMarkdown("Reset [beta]", "https://help.acme.test/articles/reset")).toBe(
      "[Reset (beta)](https://help.acme.test/articles/reset)",
    );
  });
});
