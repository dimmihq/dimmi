import type { IdentitySecret, IdentityStats } from "@/lib/api/identity";

import { modeDetail, secretDetail, secretState, signingSnippet, statsSummary, topFailures } from "./identity-format";

const secret: IdentitySecret = {
  id: "isk_1",
  object: "identity_secret",
  name: "Production",
  algorithm: "HS256",
  source: "shared_secret",
  kid: "isk_1",
  jwks_url: null,
  issuer: null,
  audience: null,
  jwks_fetched_at: null,
  jwks_error: null,
  fingerprint: "a21b",
  status: "active",
  retires_at: null,
  revoked_at: null,
  last_used_at: null,
  created_at: "2026-11-07T10:00:00Z",
};

describe("identity secrets", () => {
  it("names a shared secret by its last characters and says it was never used", () => {
    expect(secretDetail(secret)).toBe("HS256 · …a21b · Never used");
  });

  it("names a provider key set by its host", () => {
    const provider: IdentitySecret = {
      ...secret,
      algorithm: "RS256",
      source: "jwks",
      kid: "",
      jwks_url: "https://acme.clerk.accounts.dev/.well-known/jwks.json",
    };
    expect(secretDetail(provider)).toBe("RS256 key set · acme.clerk.accounts.dev · Never used");
  });

  it("marks only secrets that stopped signing new sessions", () => {
    expect(secretState(secret)).toBeNull();
    expect(secretState({ ...secret, status: "revoked", revoked_at: "2026-11-07T10:00:00Z" })).toBe("Revoked");
    expect(secretState({ ...secret, status: "retiring", retires_at: "2026-11-14T10:00:00Z" })).toMatch(/^Works until /);
  });

  it("puts the key id in every signing snippet", () => {
    expect(signingSnippet("Next.js", "isk_1")).toContain('kid: "isk_1"');
    expect(signingSnippet("Express", "isk_1")).toContain("@dimmi/server/express");
    expect(signingSnippet("Node.js", "isk_1")).toContain('kid: "isk_1"');
    expect(signingSnippet("Python", "isk_1")).toContain('"kid": "isk_1"');
    expect(signingSnippet("Script tag", "isk_1")).toContain("data-identity-url");
  });
});

describe("sign-in mode", () => {
  const stats: IdentityStats = {
    object: "identity_stats",
    days: 7,
    totals: { verified: 90, anonymous_started: 5, token_expired: 4, token_signature_invalid: 1 },
    by_day: [],
    verified: 90,
    invalid: 5,
    anonymous: 5,
    verified_share: 0.9,
  };

  it("says what each mode does for visitors", () => {
    expect(modeDetail("required")).toMatch(/Only signed-in customers/);
    expect(modeDetail("off")).toMatch(/ignored/);
  });

  it("sums up the last days and ranks refusals", () => {
    expect(statsSummary(stats)).toBe("90% of 100 messenger starts were signed in over the last 7 days.");
    expect(statsSummary({ ...stats, verified: 0, invalid: 0, anonymous: 0 })).toBeNull();
    expect(topFailures(stats).map((f) => f.code)).toEqual(["token_expired", "token_signature_invalid"]);
  });
});
