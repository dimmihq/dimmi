import { oauthError } from "./oauth-errors";

describe("oauthError", () => {
  it("explains the codes the provider callback sends", () => {
    expect(oauthError("oauth_denied")).toBe("You cancelled the sign-in at the provider.");
  });

  it("gives a general message for a code it does not know, and none for no code", () => {
    expect(oauthError("something_new")).toBe("Signing in did not work. Try again.");
    expect(oauthError(undefined)).toBeNull();
  });
});
