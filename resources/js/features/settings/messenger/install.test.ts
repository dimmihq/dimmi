import { snippetFor } from "./install";

describe("snippetFor", () => {
  it("uses the Script component on Next.js", () => {
    const { code, hint } = snippetFor("Next.js", "https://app.example.com", "wgt_1");

    expect(code).toContain('import Script from "next/script";');
    expect(code).toContain(
      '<Script src="https://app.example.com/widget/v1/loader.js" data-app-id="wgt_1" strategy="afterInteractive" />',
    );
    expect(hint).toContain("app/layout.tsx");
  });

  it("gives the script tag everywhere else, saying where it goes", () => {
    const tag = '<script async src="https://app.example.com/widget/v1/loader.js" data-app-id="wgt_1"></script>';

    expect(snippetFor("React", "https://app.example.com", "wgt_1")).toEqual({
      code: tag,
      hint: "Paste it before the closing body tag in index.html.",
    });
    expect(snippetFor("Plain HTML", "https://app.example.com", "wgt_1").code).toBe(tag);
  });
});
