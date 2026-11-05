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
