import { fireEvent, screen } from "@testing-library/react";

import { BrokenLink, ForgotPasswordForm } from "./password-reset-forms";
import { answer, renderWithQueries } from "./test-utils";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("ForgotPasswordForm", () => {
  it("swaps the form for a status naming the address, and mails again on request", async () => {
    const fetch = answer([202, { data: { message: "On its way." } }], [202, { data: { message: "On its way." } }]);
    renderWithQueries(<ForgotPasswordForm />);

    fireEvent.change(screen.getByLabelText(/email/i), { target: { value: "ada@example.com" } });
    fireEvent.click(screen.getByRole("button", { name: "Send reset link" }));

    expect(await screen.findByRole("heading", { name: "Check your inbox" })).toBeTruthy();
    expect(screen.getByRole("status").textContent).toContain("ada@example.com");
    expect(screen.queryByLabelText(/email/i)).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Send again" }));

    expect(await screen.findByText("We sent it again.")).toBeTruthy();
    expect(fetch).toHaveBeenCalledTimes(2);
  });

  it("shows why sending again failed", async () => {
    answer(
      [202, { data: { message: "On its way." } }],
      [429, { code: "too_many_attempts", message: "Too many attempts. Try again in 60 seconds." }],
    );
    renderWithQueries(<ForgotPasswordForm />);
    fireEvent.change(screen.getByLabelText(/email/i), { target: { value: "ada@example.com" } });
    fireEvent.click(screen.getByRole("button", { name: "Send reset link" }));
    await screen.findByRole("heading", { name: "Check your inbox" });

    fireEvent.click(screen.getByRole("button", { name: "Send again" }));

    expect((await screen.findByRole("alert")).textContent).toBe("Too many attempts. Try again in 60 seconds.");
  });
});

describe("BrokenLink", () => {
  it("reports the incomplete link as a failure", () => {
    renderWithQueries(<BrokenLink />);

    expect(screen.getByRole("alert").textContent).toContain("This link is incomplete");
  });
});
