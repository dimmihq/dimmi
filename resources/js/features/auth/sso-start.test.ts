import { ssoStart } from "./sso-start";

describe("ssoStart", () => {
  it("starts at the connection and lands on the page asked for", () => {
    expect(ssoStart({ redirect_url: "/auth/sso/ssoc_1/redirect", name: "Okta", enforced: true }, "/d/inbox")).toBe(
      "/auth/sso/ssoc_1/redirect?redirect=%2Fd%2Finbox",
    );
  });

  it("starts nowhere for an address without single sign-on", () => {
    expect(ssoStart({ redirect_url: null, name: null, enforced: false }, "/d")).toBeNull();
  });
});
