import { fireEvent, render, screen } from "@testing-library/react";

import { LinkPolicyContext } from "./link-checks";
import { SafeLink } from "./safe-link";

const settings = { enabled: true, allow_domains: [], block_domains: [], strip_inbound_email_links: false };

function show(workspace: string) {
  return render(
    <LinkPolicyContext.Provider value={{ workspace, settings, plain: false }}>
      <SafeLink href="https://example.com/page">the page</SafeLink>
    </LinkPolicyContext.Provider>,
  );
}

describe("SafeLink", () => {
  beforeEach(() => window.localStorage.clear());

  it("asks before an unknown link opens, and remembers a trusted domain", () => {
    const open = vi.spyOn(window, "open").mockReturnValue(null);
    show("w1");

    fireEvent.click(screen.getByRole("link", { name: "the page" }));
    expect(open).not.toHaveBeenCalled();
    expect(screen.getByText(/only open links you expect/)).toBeTruthy();

    fireEvent.click(screen.getByRole("checkbox", { name: "Always allow links from example.com" }));
    fireEvent.click(screen.getByRole("button", { name: "Open link" }));
    expect(open).toHaveBeenCalledWith("https://example.com/page", "_blank", "noopener,noreferrer");
    expect(window.localStorage.getItem("dimmi:links:trusted:w1")).toBe('["example.com"]');
    open.mockRestore();
  });

  it("opens a link of a trusted domain without asking", () => {
    window.localStorage.setItem("dimmi:links:trusted:w1", '["example.com"]');
    show("w1");

    expect(screen.getByRole("link", { name: "the page" }).getAttribute("href")).toBe("https://example.com/page");
  });
});
