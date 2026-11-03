import { fireEvent, screen, waitFor } from "@testing-library/react";

import { LoginForm } from "./login-form";
import { answer, config, renderWithQueries } from "./test-utils";

afterEach(() => {
  vi.unstubAllGlobals();
});

function fill(label: RegExp, value: string) {
  fireEvent.change(screen.getByLabelText(label), { target: { value } });
}

describe("LoginForm", () => {
  it("signs in with a password and hands over the caller", async () => {
    const me = { user: { name: "Ada" }, workspaces: [] };
    const fetch = answer([201, { data: me }]);
    const onSignedIn = vi.fn();
    renderWithQueries(<LoginForm config={config} redirect="/d" onSignedIn={onSignedIn} />);

    fill(/email/i, "ada@example.com");
    fill(/password/i, "analytical-engine-1843");
    fireEvent.click(screen.getByRole("button", { name: "Sign in" }));

    await waitFor(() => expect(onSignedIn).toHaveBeenCalled());
    expect(onSignedIn.mock.calls[0]?.[0]).toEqual(me);
    expect(JSON.parse(fetch.mock.calls[0]?.[1].body)).toEqual({
      grant: "password",
      email: "ada@example.com",
      password: "analytical-engine-1843",
    });
  });

  it("shows the server's message beside the field it blames", async () => {
    answer([
      422,
      {
        code: "invalid_credentials",
        message: "These credentials do not match.",
        errors: { email: ["These credentials do not match."] },
      },
    ]);
    renderWithQueries(<LoginForm config={config} redirect="/d" onSignedIn={vi.fn()} />);

    fill(/email/i, "ada@example.com");
    fill(/password/i, "wrong");
    fireEvent.click(screen.getByRole("button", { name: "Sign in" }));

    expect(await screen.findByText("These credentials do not match.")).toBeTruthy();
  });

  it("shows a wrong code under the code boxes, not in an alert box", async () => {
    answer(
      [202, { data: { message: "On its way." } }],
      [
        422,
        {
          code: "invalid_code",
          message: "The code is invalid or has expired.",
          errors: { code: ["The code is invalid or has expired."] },
        },
      ],
    );
    renderWithQueries(<LoginForm config={config} redirect="/d" onSignedIn={vi.fn()} />);
    fill(/email/i, "ada@example.com");
    fireEvent.click(screen.getByRole("button", { name: "Email me a sign-in link" }));
    await screen.findByText("Check your inbox");

    for (const [index, digit] of [..."123456"].entries()) {
      fireEvent.change(screen.getByLabelText(`Digit ${index + 1} of 6`), { target: { value: digit } });
    }

    expect((await screen.findByRole("alert")).textContent).toBe("The code is invalid or has expired.");
    expect(screen.queryByText("Check your inbox")).toBeTruthy();
    expect((screen.getByLabelText("Digit 1 of 6") as HTMLInputElement).value).toBe("");
  });

  it("shows a failure that names no field under the email", async () => {
    answer([429, { code: "too_many_attempts", message: "Too many attempts. Try again in 30 seconds." }]);
    renderWithQueries(<LoginForm config={config} redirect="/d" onSignedIn={vi.fn()} />);

    fill(/email/i, "ada@example.com");
    fill(/password/i, "whatever-1");
    fireEvent.click(screen.getByRole("button", { name: "Sign in" }));

    const message = await screen.findByText("Too many attempts. Try again in 30 seconds.");
    expect(screen.getByLabelText(/email/i).getAttribute("aria-describedby")).toContain(message.id);
  });

  it("shows an error a provider sent back", () => {
    renderWithQueries(
      <LoginForm config={config} redirect="/d" initialError="You cancelled the sign-in." onSignedIn={vi.fn()} />,
    );

    expect(screen.getByRole("alert").textContent).toContain("You cancelled the sign-in.");
  });

  it("mails a sign-in link and asks for its code", async () => {
    const fetch = answer([202, { data: { message: "On its way." } }]);
    renderWithQueries(<LoginForm config={config} redirect="/d" onSignedIn={vi.fn()} />);

    fill(/email/i, "ada@example.com");
    fireEvent.click(screen.getByRole("button", { name: "Email me a sign-in link" }));

    expect(await screen.findByText("Check your inbox")).toBeTruthy();
    expect(fetch.mock.calls[0]?.[0]).toBe("/api/v1/magic-links");
  });

  it("links each provider's redirect with where to land, and hides what the server does not offer", () => {
    renderWithQueries(
      <LoginForm
        config={{ ...config, magic_link: false, signup_mode: "invite_only" }}
        redirect="/d/inbox"
        onSignedIn={vi.fn()}
      />,
    );

    const github = screen.getByRole("link", { name: "Continue with GitHub" });
    expect(github.getAttribute("href")).toBe("/auth/github/redirect?redirect=%2Fd%2Finbox");
    expect(screen.queryByRole("button", { name: "Email me a sign-in link" })).toBeNull();
    expect(screen.queryByText("Create an account")).toBeNull();
  });
});
