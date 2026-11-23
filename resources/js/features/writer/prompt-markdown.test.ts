import { describe, expect, it } from "vitest";

import { promptMarkdown } from "./prompt-markdown";

describe("prompt markdown", () => {
  it("keeps underscores inside words and variables as typed", () => {
    expect(promptMarkdown("Cover {{window\\_start}} with list\\_articles.")).toBe(
      "Cover {{window_start}} with list_articles.",
    );
  });

  it("keeps the underscores that would mark emphasis escaped", () => {
    expect(promptMarkdown("a \\_lone\\_ word")).toBe("a \\_lone\\_ word");
  });
});
