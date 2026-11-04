import type { ChallengeMethod } from "@/lib/api/auth";

import { firstMethod, offers, passkeyOptions } from "./second-factor";

const totp: ChallengeMethod = { method: "totp", factor_id: "fac_1", name: "Authenticator app", preferred: true };
const recovery: ChallengeMethod = {
  method: "recovery_code",
  factor_id: "fac_2",
  name: "Recovery codes",
  preferred: false,
};

const options = { challenge: "c2lnbiBtZQ", rpId: "localhost" };
const passkey: ChallengeMethod = { method: "passkey", factor_id: "fac_3", name: "Laptop", preferred: false, options };

describe("firstMethod", () => {
  it("asks for a preferred passkey first", () => {
    expect(firstMethod([recovery, { ...totp, preferred: false }, { ...passkey, preferred: true }])).toBe("passkey");
    expect(firstMethod([passkey, recovery])).toBe("passkey");
  });

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

describe("passkeyOptions", () => {
  it("offers a passkey only with the options to sign", () => {
    expect(passkeyOptions([totp, passkey])).toEqual(options);
    expect(offers([totp, passkey], "passkey")).toBe(true);
    expect(offers([totp, { ...passkey, options: null }], "passkey")).toBe(false);
    expect(offers(null, "passkey")).toBe(false);
  });
});
