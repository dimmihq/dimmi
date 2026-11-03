import { fireEvent, screen, waitFor } from "@testing-library/react";

import { answer, renderWithQueries } from "./test-utils";
import { TwoFactorForm } from "./two-factor-form";

afterEach(() => {
  vi.unstubAllGlobals();
});

function answerWithRecoveryCode(code: string) {
  fireEvent.click(screen.getByRole("button", { name: "Use a recovery code instead" }));
  fireEvent.change(screen.getByLabelText(/recovery code/i), { target: { value: code } });
  fireEvent.click(screen.getByRole("button", { name: "Verify" }));
}

describe("TwoFactorForm", () => {
  it("signs in with a recovery code instead of the authenticator", async () => {
    const me = { object: "me", user: { name: "Ada" }, workspaces: [] };
    const fetch = answer([201, { data: me }]);
    const onSignedIn = vi.fn();
    renderWithQueries(<TwoFactorForm methods={null} onSignedIn={onSignedIn} onExpired={vi.fn()} />);

    answerWithRecoveryCode("7k2m9-x4q8z");

    await waitFor(() => expect(onSignedIn).toHaveBeenCalled());
    expect(JSON.parse(fetch.mock.calls[0]?.[1].body)).toEqual({
      grant: "two_factor",
      method: "recovery_code",
      code: "7k2m9-x4q8z",
    });
  });

  it("shows a wrong code under the field", async () => {
    answer([
      422,
      { code: "invalid_code", message: "The code is invalid.", errors: { code: ["The code is invalid."] } },
    ]);
    renderWithQueries(<TwoFactorForm methods={null} onSignedIn={vi.fn()} onExpired={vi.fn()} />);

    answerWithRecoveryCode("7k2m9-x4q8z");

    expect(await screen.findByText("The code is invalid.")).toBeTruthy();
  });

  it("starts over when the challenge expired", async () => {
    answer([401, { code: "two_factor_challenge_expired", message: "Sign in again." }]);
    const onExpired = vi.fn();
    renderWithQueries(<TwoFactorForm methods={null} onSignedIn={vi.fn()} onExpired={onExpired} />);

    answerWithRecoveryCode("7k2m9-x4q8z");

    await waitFor(() => expect(onExpired).toHaveBeenCalled());
  });

  it("explains a challenge that offers no way to finish", () => {
    renderWithQueries(<TwoFactorForm methods={[]} onSignedIn={vi.fn()} onExpired={vi.fn()} />);

    expect(screen.getByText("No way to finish signing in")).toBeTruthy();
  });

  it("hides the recovery fallback when the challenge does not offer it", () => {
    renderWithQueries(
      <TwoFactorForm
        methods={[{ method: "totp", factor_id: "fac_1", name: "Authenticator app", preferred: true }]}
        onSignedIn={vi.fn()}
        onExpired={vi.fn()}
      />,
    );

    expect(screen.queryByRole("button", { name: "Use a recovery code instead" })).toBeNull();
  });
});
