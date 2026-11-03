import type { ChallengeMethod } from "@/lib/api/auth";

import { firstMethod, offers } from "./second-factor";

const totp: ChallengeMethod = { method: "totp", factor_id: "fac_1", name: "Authenticator app", preferred: true };
const recovery: ChallengeMethod = {
  method: "recovery_code",
  factor_id: "fac_2",
  name: "Recovery codes",
  preferred: false,
};

describe("firstMethod", () => {
  it("asks for the preferred method first", () => {
    expect(firstMethod([recovery, totp])).toBe("totp");
  });

  it("asks for recovery codes when they are all the challenge offers", () => {
    expect(firstMethod([recovery])).toBe("recovery_code");
  });

  it("asks for the authenticator when the challenge is not known", () => {
    expect(firstMethod(null)).toBe("totp");
  });
});

describe("offers", () => {
  it("offers only what the challenge lists", () => {
    expect(offers([totp], "recovery_code")).toBe(false);
    expect(offers([totp, recovery], "recovery_code")).toBe(true);
  });

  it("offers both when the challenge is not known", () => {
    expect(offers(null, "recovery_code")).toBe(true);
  });
});
