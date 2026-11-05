import { render } from "@testing-library/react";

import { MessageBody } from "./message-body";

function shown(text: string, format?: "markdown" | "plain") {
  return render(<MessageBody text={text} format={format} />).container;
}

describe("MessageBody", () => {
  it("shows plain text mail exactly as written, markup characters and all", () => {
    const text = "2*3*4 equals 24\n# 123 is my order\n    Name: John\nThe <div> tag and List<String>";
    const body = shown(text, "plain");

    expect(body.textContent).toBe(text);
    expect(body.querySelector("em, h1, code, div div")).toBeNull();
  });

  it("keeps raw html in markdown as the words it is", () => {
    const body = shown("Use the <div> tag, **really**\n\n<b>whole line</b>\n\nnext line");

    expect(body.textContent).toContain("Use the <div> tag, really");
    expect(body.textContent).toContain("<b>whole line</b>");
    expect(body.textContent).toContain("next line");
    expect(body.querySelector("b")).toBeNull();
  });

  it("keeps the number a list starts at", () => {
    const body = shown("3. third step\n4. fourth step");

    expect(body.querySelector("ol")?.getAttribute("start")).toBe("3");
  });
});
