import { fireEvent, screen, waitFor } from "@testing-library/react";

import { SignupForm } from "./signup-form";
import { answer, config, renderWithQueries, terms } from "./test-utils";

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

  it("asks nothing about legal terms on a server that publishes none", () => {
    renderWithQueries(<SignupForm config={config} redirect="/d" onSignedIn={vi.fn()} />);

    expect(screen.queryByRole("checkbox")).toBeNull();
  });

  it("creates no account until the legal terms are accepted, then says they were", async () => {
    const fetch = answer([201, { data: { user: { name: "Grace" }, workspaces: [] } }]);
    renderWithQueries(<SignupForm config={{ ...config, terms }} redirect="/d" onSignedIn={vi.fn()} />);

    const box = screen.getByRole("checkbox", {
      name: "I agree to the Terms of Service, Privacy Policy and Data Processing Agreement.",
    }) as HTMLInputElement;
    expect(box.checked).toBe(false);
    expect(screen.getByRole("link", { name: "Terms of Service" }).getAttribute("href")).toBe(terms.terms_url);
    expect(screen.getByRole("link", { name: "Privacy Policy" }).getAttribute("href")).toBe(terms.privacy_url);
    expect(screen.getByRole("link", { name: "Data Processing Agreement" }).getAttribute("href")).toBe(
      terms.data_processing_url,
    );
    expect(screen.getByRole("link", { name: "Terms of Service" }).getAttribute("target")).toBe("_blank");

    fill(/full name/i, "Grace Hopper");
    fill(/email/i, "grace@example.com");
    fill(/password/i, "analytical-engine-1843");
    fireEvent.click(screen.getByRole("button", { name: "Create account" }));

    expect(await screen.findByText("Tick the box to agree before you create your account.")).toBeTruthy();
    expect(fetch).not.toHaveBeenCalled();

    fireEvent.click(box);
    expect(screen.queryByText("Tick the box to agree before you create your account.")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Create account" }));

    await waitFor(() => expect(fetch).toHaveBeenCalled());
    expect(JSON.parse(String(fetch.mock.calls[0]?.[1]?.body))).toMatchObject({ terms_accepted: true });
  });

  it("shows under the box what the server says when it refuses an account for want of the terms", async () => {
    answer([
      422,
      {
        code: "validation_failed",
        message: "Accept the terms to create your account.",
        errors: { terms_accepted: ["Accept the terms to create your account."] },
      },
    ]);
    renderWithQueries(<SignupForm config={{ ...config, terms }} redirect="/d" onSignedIn={vi.fn()} />);

    fill(/full name/i, "Grace Hopper");
    fill(/email/i, "grace@example.com");
    fill(/password/i, "analytical-engine-1843");
    fireEvent.click(screen.getByRole("checkbox"));
    fireEvent.click(screen.getByRole("button", { name: "Create account" }));

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toBe("Accept the terms to create your account.");
    expect(screen.getAllByText("Accept the terms to create your account.")).toHaveLength(1);
  });

  it("holds a provider sign-up until the legal terms are accepted, then carries the acceptance", () => {
    renderWithQueries(<SignupForm config={{ ...config, terms }} redirect="/d" onSignedIn={vi.fn()} />);

    const github = screen.getByRole("link", { name: "Sign up with GitHub" });
    expect(github.getAttribute("href")).not.toContain("terms_accepted");
    expect(fireEvent.click(github)).toBe(false);
    expect(screen.getByText("Tick the box to agree before you create your account.")).toBeTruthy();

    fireEvent.click(screen.getByRole("checkbox"));

    expect(screen.getByRole("link", { name: "Sign up with GitHub" }).getAttribute("href")).toContain(
      "terms_accepted=true",
    );
  });

  it("waits for the captcha's answer and sends it with the form", async () => {
    let solve: (token: string) => void = () => undefined;
    vi.stubGlobal("turnstile", {
      render: (_element: HTMLElement, options: { callback: (token: string) => void }) => {
        solve = options.callback;
        return "widget";
      },
      remove: vi.fn(),
    });
    const fetch = answer([201, { data: { user: { name: "Grace" }, workspaces: [] } }]);
    const captcha = { provider: "turnstile" as const, site_key: "site" };
    renderWithQueries(<SignupForm config={{ ...config, captcha }} redirect="/d" onSignedIn={vi.fn()} />);

    fill(/full name/i, "Grace Hopper");
    fill(/email/i, "grace@example.com");
    fill(/password/i, "analytical-engine-1843");
    const create = screen.getByRole("button", { name: "Create account" }) as HTMLButtonElement;
    expect(create.disabled).toBe(true);

    await waitFor(() => expect(screen.getByRole("group", { name: "Check that you are not a robot" })).toBeTruthy());
    await waitFor(() => solve("answer"));
    await waitFor(() => expect(create.disabled).toBe(false));
    fireEvent.click(create);

    await waitFor(() => expect(fetch).toHaveBeenCalled());
    expect(JSON.parse(String(fetch.mock.calls[0]?.[1]?.body))).toMatchObject({ captcha_token: "answer" });
  });
});
