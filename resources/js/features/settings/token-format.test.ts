import type { PersonalAccessToken } from "@/lib/api/tokens";

import { abilityLabel, abilitySummary, expiryFor, isLive, tokenDetail } from "./token-format";

const now = new Date("2026-11-02T09:00:00Z");

function token(changes: Partial<PersonalAccessToken> = {}): PersonalAccessToken {
  return {
    id: "tok_1",
    object: "personal_access_token",
    name: "CI",
    prefix: "dmi_live_Ab12",
    abilities: ["conversations:read"],
    expires_at: null,
    last_used_at: null,
    last_used_ip: null,
    revoked_at: null,
    member: { id: "mem_1", name: "Ada" },
    created_at: "2026-11-01T10:00:00Z",
    ...changes,
  };
}

describe("abilityLabel", () => {
  it("names a resource and what may be done with it", () => {
    expect(abilityLabel("conversations:read")).toBe("Conversations: read");
    expect(abilityLabel("ai:keys")).toBe("AI: keys");
    expect(abilityLabel("*")).toBe("Everything your role allows");
  });
});

describe("abilitySummary", () => {
  it("names two abilities and counts the rest", () => {
    expect(abilitySummary(["members:read"])).toBe("Members: read");
    expect(abilitySummary(["a:read", "b:read", "c:read", "d:read"])).toBe("A: read, B: read and 2 more");
    expect(abilitySummary(["*"])).toBe("Everything your role allows");
  });
});

describe("expiryFor", () => {
  it("counts days from now, or never", () => {
    expect(expiryFor("30 days", now)).toBe("2026-12-02T09:00:00.000Z");
    expect(expiryFor("Never", now)).toBeNull();
  });
});

describe("tokenDetail", () => {
  it("says what a token may do, its use and its end", () => {
    expect(tokenDetail(token(), now)).toBe("dmi_live_Ab12… · Conversations: read · Never used · Never expires");
    expect(tokenDetail(token({ expires_at: "2026-11-01T00:00:00Z" }), now)).toContain("Expired Nov 1, 2026");
    expect(tokenDetail(token({ revoked_at: "2026-11-02T08:00:00Z" }), now)).toContain("Revoked Nov 2, 2026");
    expect(tokenDetail(token({ last_used_at: "2026-11-02T08:00:00Z" }), now)).toContain("Last used Nov 2, 2026");
  });
});

describe("isLive", () => {
  it("is false once revoked or expired", () => {
    expect(isLive(token(), now)).toBe(true);
    expect(isLive(token({ revoked_at: "2026-11-02T08:00:00Z" }), now)).toBe(false);
    expect(isLive(token({ expires_at: "2026-11-02T09:00:00Z" }), now)).toBe(false);
  });
});
