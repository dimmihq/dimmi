import { fireEvent, render, screen } from "@testing-library/react";

import type { Attachment } from "@/lib/api/inbox";

import { MessageFiles } from "./message-files";

const base: Attachment = {
  id: "att_1",
  filename: "invoice.pdf",
  content_type: "application/pdf",
  size_bytes: 2048,
  width: null,
  height: null,
  url: "https://files.example/invoice.pdf",
  blocked: false,
  scanned: true,
};

describe("MessageFiles", () => {
  it("links a scanned file with no warning", () => {
    render(<MessageFiles files={[base]} ours={false} />);

    expect(screen.getByRole("link", { name: "Download invoice.pdf" })).toBeTruthy();
    expect(screen.queryByText(/not scanned/i)).toBeNull();
  });

  it("shows an infected file in red as a virus found, with nothing to open", () => {
    render(<MessageFiles files={[{ ...base, url: null, blocked: true, scanned: false }]} ours={false} />);

    expect(screen.getByRole("alert").textContent).toContain("Virus found");
    expect(screen.queryByRole("link")).toBeNull();
    expect(screen.queryByRole("button")).toBeNull();
  });

  it("asks before opening a file nothing scanned, and opens it only when told to", () => {
    const open = vi.spyOn(window, "open").mockReturnValue(null);
    render(<MessageFiles files={[{ ...base, scanned: false }]} ours={false} />);

    expect(screen.getByText(/Not scanned for viruses/)).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Download invoice.pdf" }));
    expect(open).not.toHaveBeenCalled();
    expect(screen.getByText(/be very careful/)).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Open anyway" }));
    expect(open).toHaveBeenCalledWith(base.url, "_blank", "noopener,noreferrer");
    open.mockRestore();
  });
});
