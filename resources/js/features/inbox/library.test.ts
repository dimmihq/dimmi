import { search } from "./library";

describe("search", () => {
  it("lists every item of a kind for an empty query", () => {
    expect(search("reply", "").map((item) => item.title)).toEqual([
      "Send a sign-in code",
      "Ask for the mail app",
      "Closing the loop",
    ]);
  });

  it("matches the title or the detail, ignoring case", () => {
    expect(search("article", "SECURITY").map((item) => item.title)).toEqual(["Why reset links expire"]);
    expect(search("issue", "#398").map((item) => item.id)).toEqual(["i2"]);
  });

  it("keeps to the kind asked for", () => {
    expect(search("changelog", "sign-in").every((item) => item.kind === "changelog")).toBe(true);
  });
});
