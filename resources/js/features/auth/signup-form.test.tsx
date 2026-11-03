import { fireEvent, screen, waitFor } from "@testing-library/react";

import { SignupForm } from "./signup-form";
import { answer, config, renderWithQueries } from "./test-utils";

afterEach(() => {
  vi.unstubAllGlobals();
});

function fill(label: RegExp, value: string) {
  fireEvent.change(screen.getByLabelText(label), { target: { value } });
}

describe("SignupForm", () => {
  it("creates the account and hands over the caller", async () => {
    const fetch = answer([201, { data: { user: { name: "Grace" }, workspaces: [] } }]);
    const onSignedIn = vi.fn();
    renderWithQueries(<SignupForm config={config} redirect="/d" onSignedIn={onSignedIn} />);

    fill(/full name/i, "Grace Hopper");
    fill(/email/i, "grace@example.com");
    fill(/password/i, "analytical-engine-1843");
    fireEvent.click(screen.getByRole("button", { name: "Create account" }));

    await waitFor(() => expect(onSignedIn).toHaveBeenCalled());
    expect(fetch.mock.calls[0]?.[0]).toBe("/api/v1/users");
  });

  it("tells the minimum password length the server asks for", () => {
    renderWithQueries(
      <SignupForm config={{ ...config, password_min_length: 14 }} redirect="/d" onSignedIn={vi.fn()} />,
    );

    expect(screen.getByText("At least 14 characters.")).toBeTruthy();
  });

  it("explains that signing up is by invitation instead of offering a form", () => {
    renderWithQueries(
      <SignupForm config={{ ...config, signup_mode: "invite_only" }} redirect="/d" onSignedIn={vi.fn()} />,
    );

    expect(screen.getByText("Signing up is by invitation")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Create account" })).toBeNull();
  });
});
