import { ApiError } from "@/lib/api/client";

import { forPreview, linkProblems } from "./links";

describe("home links while editing", () => {
  it("keeps unfinished links out of the preview and remembers each link's row", () => {
    const draft = {
      name: "Docs",
      home_links: [
        { title: "", url: "", icon: "link" as const },
        { title: "Docs", url: "https://docs.example.com", icon: "book" as const },
        { title: "Call", url: " ", icon: "calendar" as const },
      ],
    };

    expect(forPreview(draft)).toEqual({
      change: { name: "Docs", home_links: [draft.home_links[1]] },
      rows: [1],
    });
    expect(forPreview({ name: "Docs" })).toEqual({ change: { name: "Docs" }, rows: [] });
  });

  it("puts each refusal on the row it is about", () => {
    const error = new ApiError(422, "validation_failed", "Use https.", {
      "home_links.0.url": "Use https.",
      name: "Too long.",
    });

    expect(linkProblems(error, [2])).toEqual({ "home_links.2.url": "Use https." });
    expect(linkProblems(error)).toEqual({ "home_links.0.url": "Use https." });
    expect(linkProblems(new ApiError(422, "validation_failed", "Six.", { home_links: "Six." }))).toEqual({
      home_links: "Six.",
    });
    expect(linkProblems(null)).toEqual({});
  });
});
