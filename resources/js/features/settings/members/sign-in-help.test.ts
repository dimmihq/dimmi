import { describe, expect, it } from "vitest";

import type { Member } from "@/lib/api/workspace";

import { helpLabel, helpOffers } from "./sign-in-help";

function member(changes: Partial<Member> = {}): Member {
  return {
    id: "mem_ada",
    role: "member",
    status: "active",
    user: { id: "usr_ada", name: "Ada", email: "ada@example.com", email_verified: true, avatar_url: null },
    sign_in_locked: false,
    two_factor_enabled: true,
    last_active_at: null,
    conversation_access: "all",
    conversation_access_unassigned: false,
    access_team_ids: [],
    ...changes,
  };
}

const admin = { self: "mem_grace", write: true, owners: false, reset: true };

describe("helpOffers", () => {
  it("offers an admin a reset link for a teammate", () => {
    expect(helpOffers(member(), admin).map(helpLabel)).toEqual(["Help with two-factor"]);
  });

  it("offers to clear a lock only while one holds", () => {
    expect(helpOffers(member({ sign_in_locked: true }), admin).map(helpLabel)).toEqual([
      "Help with two-factor",
      "Clear sign-in lock",
    ]);
  });

  it("keeps owners for those who manage owners", () => {
    expect(helpOffers(member({ role: "owner" }), admin)).toEqual([]);
    expect(helpOffers(member({ role: "owner" }), { ...admin, owners: true })).toHaveLength(1);
  });

  it("offers nothing for oneself, a deactivated teammate, or without the permissions", () => {
    expect(helpOffers(member({ id: "mem_grace" }), admin)).toEqual([]);
    expect(helpOffers(member({ status: "deactivated" }), admin)).toEqual([]);
    expect(helpOffers(member({ sign_in_locked: null }), { ...admin, write: false, reset: false })).toEqual([]);
  });
});
