import type { IdentitySecret } from "@/lib/api/identity";

import { secretDetail, secretState, signingSnippet } from "./identity-format";

const secret: IdentitySecret = {
  id: "isk_1",
  object: "identity_secret",
  name: "Production",
  algorithm: "HS256",
  kid: "isk_1",
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

  it("marks only secrets that stopped signing new sessions", () => {
    expect(secretState(secret)).toBeNull();
    expect(secretState({ ...secret, status: "revoked", revoked_at: "2026-11-07T10:00:00Z" })).toBe("Revoked");
    expect(secretState({ ...secret, status: "retiring", retires_at: "2026-11-14T10:00:00Z" })).toMatch(/^Works until /);
  });

  it("puts the key id in every signing snippet", () => {
    expect(signingSnippet("Node.js", "isk_1")).toContain('kid: "isk_1"');
    expect(signingSnippet("Python", "isk_1")).toContain('"kid": "isk_1"');
    expect(signingSnippet("Script tag", "isk_1")).toContain("data-identity-url");
  });
});
