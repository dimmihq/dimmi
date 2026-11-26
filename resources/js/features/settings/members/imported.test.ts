import { describe, expect, it } from "vitest";

import type { Member } from "@/lib/api/workspace";

import { actionLabel, enableAllText, enableText, importState, sectionLine, sourceName } from "./imported";

function member(changes: Partial<Member> & { verified?: boolean; name?: string }): Member {
  const { verified = false, name = "Amara Okafor", ...rest } = changes;

  return {
    id: "mem_1",
    role: "member",
    status: "deactivated",
    user: { id: "usr_1", name, email: "amara@example.com", email_verified: verified, avatar_url: null },
    sign_in_locked: null,
    two_factor_enabled: false,
    last_active_at: null,
    conversation_access: "all",
    conversation_access_unassigned: false,
    access_team_ids: [],
    awaiting_enable: true,
    imported_from: "intercom",
    enable_invited_at: null,
    enable_team_ids: [],
    ...rest,
  };
}

describe("imported teammates", () => {
  it("counts signing up once the address is proven", () => {
    expect(importState(member({ verified: true }))).toBe("waiting");
    expect(importState(member({}))).toBe("not_signed_up");
    expect(importState(member({ enable_invited_at: "2026-11-26T10:00:00Z" }))).toBe("invited");
  });

  it("names the tool they came from", () => {
    expect(sourceName("intercom")).toBe("Intercom");
    expect(sourceName("help_scout")).toBe("Help Scout");
    expect(sourceName(null)).toBe("another tool");
  });

  it("says how many wait", () => {
    expect(sectionLine([member({})])).toBe("1 teammate from Intercom waits for access. Their history is already here.");
    expect(sectionLine([member({}), member({ imported_from: "zendesk" })])).toBe(
      "2 teammates wait for access. Their history is already here.",
    );
  });

  it("lets a teammate who signed up in at once, with their role and teams", () => {
    const text = enableText(member({ verified: true, role: "admin" }), ["Support", "Billing"]);
    expect(text.confirm).toBe("Enable");
    expect(text.description).toBe(
      "Amara gets access straight away as an admin and joins Support and Billing. We'll email them.",
    );
  });

  it("invites a teammate who has not signed up", () => {
    const text = enableText(member({}), []);
    expect(text.confirm).toBe("Send invite");
    expect(text.description).toContain("hasn't signed up yet");
    expect(actionLabel(member({ enable_invited_at: "2026-11-26T10:00:00Z" }))).toBe("Resend");
  });

  it("sums up enabling everyone", () => {
    const text = enableAllText([member({ verified: true }), member({}), member({})]);
    expect(text.title).toBe("Enable 3 teammates?");
    expect(text.description).toBe("1 who signed up gets access now; 2 get an invitation by email.");
  });
});
