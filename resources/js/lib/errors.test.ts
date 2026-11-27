import { ApiError } from "./api/client";
import { readReporting, reportable } from "./errors";

function page(meta: Record<string, string>): Document {
  const doc = document.implementation.createHTMLDocument();
  for (const [name, content] of Object.entries(meta)) {
    const tag = doc.createElement("meta");
    tag.name = `dimmi-errors-${name}`;
    tag.content = content;
    doc.head.append(tag);
  }

  return doc;
}

describe("readReporting", () => {
  it("reads the tracker, environment and release the page names", () => {
    const doc = page({ dsn: "https://key@errors.example.com/2", environment: "staging", release: "abc123" });

    expect(readReporting(doc)).toEqual({
      dsn: "https://key@errors.example.com/2",
      environment: "staging",
      release: "abc123",
    });
  });

  it("reports nothing when the page names no tracker", () => {
    expect(readReporting(page({ environment: "staging" }))).toBeNull();
  });
});

describe("reportable", () => {
  it("leaves the server's own answers to the server", () => {
    expect(reportable(new ApiError(500, "server_error", "Something broke"))).toBe(false);
  });

  it("reports the app's own crashes", () => {
    expect(reportable(new TypeError("x is undefined"))).toBe(true);
  });
});
