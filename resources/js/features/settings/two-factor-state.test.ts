import type { AuthFactor } from "@/lib/api/auth-factors";

import { remainingLabel, twoFactorState } from "./two-factor-state";

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
