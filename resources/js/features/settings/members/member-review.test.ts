import { describe, expect, it } from "vitest";

import type { Member } from "@/lib/api/workspace";

import { activity, filterFor, filterKeyFrom, filters, lacksTwoFactor, unprotectedLine } from "./member-review";

function member(changes: Partial<Member>): Member {
  return {
    id: "mem_1",
    role: "member",
    status: "active",
    user: { id: "usr_1", name: "Ada", email: "ada@example.com", email_verified: true, avatar_url: null },
    sign_in_locked: false,
    two_factor_enabled: true,
    last_active_at: null,
    conversation_access: "all",
    conversation_access_unassigned: false,
    access_team_ids: [],
    ...changes,
  };
}

describe("member review", () => {
  it("turns the picked view into the list filter", () => {
    expect(filterFor(filters.withoutTwoFactor)).toEqual({ twoFactor: false });
    expect(filterFor(filters.everyone)).toEqual({});
  });

  it("reads the view from the address", () => {
    expect(filterKeyFrom({ two_factor: false })).toBe(filters.withoutTwoFactor);
    expect(filterKeyFrom({ two_factor: "false" })).toBe(filters.withoutTwoFactor);
    expect(filterKeyFrom({ two_factor: true })).toBe(filters.everyone);
    expect(filterKeyFrom({})).toBe(filters.everyone);
  });

  it("flags only a member the server says has no second factor", () => {
    expect(lacksTwoFactor(member({ two_factor_enabled: false }))).toBe(true);
    expect(lacksTwoFactor(member({ two_factor_enabled: true }))).toBe(false);
    expect(lacksTwoFactor(member({ two_factor_enabled: null }))).toBe(false);
  });

  it("says when the member was last active, if the caller is told", () => {
    const now = Date.parse("2026-11-04T12:00:00Z");

    expect(activity(member({ last_active_at: "2026-11-04T09:00:00Z" }), now, "UTC")?.relative).toBe(
      "Active 3 hours ago",
    );
    expect(activity(member({ last_active_at: null }), now)).toEqual({ relative: "Never active", full: "" });
    expect(activity(member({ last_active_at: null, two_factor_enabled: null }), now)).toBeNull();
  });

  it("counts the members a requirement would keep out", () => {
    expect(unprotectedLine(0)).toBeNull();
    expect(unprotectedLine(1)).toBe("1 member has no second factor yet.");
    expect(unprotectedLine(3)).toBe("3 members have no second factor yet.");
  });
});
