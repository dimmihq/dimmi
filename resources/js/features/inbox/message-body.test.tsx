import { render, screen, waitFor } from "@testing-library/react";

import { MessageBody } from "./message-body";
import { ThreadZone } from "./thread/thread-zone";
import { When } from "./thread/when";

describe("MessageBody", () => {
  it("highlights a fenced code block once the highlighter loads and offers to copy it", async () => {
    const { container } = render(<MessageBody text={"```js\nconst total = 42;\n```"} />);

    expect(screen.getByRole("button", { name: "Copy code" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Show code full screen" })).toBeTruthy();
    await waitFor(() => expect(container.querySelector(".hljs-keyword")?.textContent).toBe("const"));
  });

  it("opens an image from a button and shows a missing one as its words", () => {
    render(<MessageBody text={"![Receipt](https://example.com/r.png) and ![Lost]()"} />);

    expect(screen.getByRole("button", { name: "Open image Receipt" })).toBeTruthy();
    expect(screen.getByText("Lost")).toBeTruthy();
  });
});

describe("mentions", () => {
  it("shows each mention as a chip naming who, and an unknown one as a teammate", () => {
    render(
      <MessageBody
        text="Ask <@mem_abc> or <@team_xyz>, not <@mem_gone>"
        mentions={[
          { type: "member", id: "mem_abc", name: "Ada [admin]" },
          { type: "team", id: "team_xyz", name: "Billing" },
        ]}
      />,
    );

    expect(screen.getByText("@Ada admin")).toBeTruthy();
    expect(screen.getByText("@Billing")).toBeTruthy();
    expect(screen.getByText("@teammate")).toBeTruthy();
    expect(screen.queryByRole("link")).toBeNull();
  });
});

describe("When", () => {
  it("reads as the customer's clock time and names the full date", () => {
    render(
      <ThreadZone.Provider value="Asia/Tokyo">
        <When iso="2026-11-05T05:02:00Z" prefix="Seen " />
      </ThreadZone.Provider>,
    );

    const time = screen.getByRole("button");
    expect(time.textContent).toMatch(/^Seen 2:02\sPM$/);
    expect(time.getAttribute("aria-label")).toMatch(/^Seen Nov 5, 2026/);
  });
});

function shown(text: string, format?: "markdown" | "plain") {
  return render(<MessageBody text={text} format={format} />).container;
}

describe("MessageBody as text", () => {
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
