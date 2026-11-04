import type { AuthFactor } from "@/lib/api/auth-factors";

import { isLastFactor, remainingLabel, twoFactorState, usedLabel } from "./two-factor-state";

function factor(type: AuthFactor["type"], status: AuthFactor["status"]): AuthFactor {
  return {
    id: `fac_${type}_${status}`,
    object: "auth_factor",
    type,
    name: type,
    status,
    preferred: false,
    confirmed_at: null,
    last_used_at: null,
    created_at: "2026-11-01T10:00:00Z",
  };
}

describe("twoFactorState", () => {
  it("is off while the authenticator is only pending", () => {
    expect(twoFactorState([factor("totp", "pending")]).on).toBe(false);
  });

  it("is on with an active authenticator, and finds the recovery codes", () => {
    const state = twoFactorState([factor("totp", "active"), factor("recovery_code", "active")]);

    expect(state.on).toBe(true);
    expect(state.recovery?.type).toBe("recovery_code");
  });

  it("is on with a passkey alone, and lists every active passkey", () => {
    const laptop = { ...factor("passkey", "active"), id: "fac_laptop" };
    const phone = { ...factor("passkey", "active"), id: "fac_phone" };
    const state = twoFactorState([laptop, phone, factor("passkey", "pending")]);

    expect(state.on).toBe(true);
    expect(state.authenticator).toBeUndefined();
    expect(state.passkeys.map((f) => f.id)).toEqual(["fac_laptop", "fac_phone"]);
  });
});

describe("isLastFactor", () => {
  it("is true only for the one authenticator or passkey left", () => {
    const app = factor("totp", "active");
    const key = factor("passkey", "active");

    expect(isLastFactor([app, factor("recovery_code", "active")], app)).toBe(true);
    expect(isLastFactor([app, key], app)).toBe(false);
    expect(isLastFactor([key], key)).toBe(true);
  });
});

describe("usedLabel", () => {
  const dates = new Intl.DateTimeFormat("en-US", { dateStyle: "medium", timeZone: "UTC" });

  it("says when a factor was added and last used", () => {
    const key = { ...factor("passkey", "active"), last_used_at: "2026-11-03T10:00:00Z" };

    expect(usedLabel(key, dates)).toBe("Added Nov 1, 2026, last used Nov 3, 2026");
    expect(usedLabel(factor("passkey", "active"), dates)).toBe("Added Nov 1, 2026, never used");
  });
});

describe("remainingLabel", () => {
  it("counts the codes left", () => {
    expect(remainingLabel(8, 10)).toBe("8 of 10 left");
  });

  it("warns when few or none are left", () => {
    expect(remainingLabel(2, 10)).toBe("2 of 10 left. Make new ones soon.");
    expect(remainingLabel(0, 10)).toBe("No codes left. Make new ones now.");
  });
});
