import { type AuditEntry, auditPath, exportPath } from "@/lib/api/audit";

import {
  actorDetail,
  actorName,
  auditEventGroups,
  auditEventLabel,
  auditEvents,
  formatTime,
  groupFilter,
  securityEventLabel,
} from "./log-labels";

function actor(type: string, via: string | null): AuditEntry["actor"] {
  return { type, id: null, label: "", via: via ? { kind: via, id: null } : null };
}

describe("log labels", () => {
  it("names known events and falls back to the server's name", () => {
    expect(auditEventLabel("member.role_changed")).toBe("Changed a role");
    expect(auditEventLabel("billing.plan_changed")).toBe("billing.plan_changed");
    expect(securityEventLabel("login.succeeded")).toBe("Signed in");
    expect(securityEventLabel("something.new")).toBe("something.new");
  });

  it("names every event the server records, colleagues' sign-ins and exports included", () => {
    for (const event of [
      "member.signed_in",
      "member.sign_in_failed",
      "member.sign_in_locked",
      "member.sessions_revoked",
      "member.two_factor_enabled",
      "member.two_factor_disabled",
      "member.auth_factor_added",
      "member.auth_factor_removed",
      "member.recovery_code_used",
      "member.password_changed",
      "member.password_reset",
      "member.email_changed",
      "member.identity_linked",
      "member.identity_unlinked",
      "audit_logs.exported",
    ]) {
      expect(auditEventLabel(event)).not.toBe(event);
    }
  });

  it("groups the events by kind, each event in one group", () => {
    expect(auditEventGroups.map((group) => group.name)).toEqual([
      "Sign-in",
      "Security settings",
      "Members and teams",
      "Tokens and apps",
      "Inbox",
      "Email",
      "Data export",
    ]);
    const grouped = auditEventGroups.flatMap((group) => Object.keys(group.events));
    expect(new Set(grouped).size).toBe(grouped.length);
    expect(Object.keys(auditEvents)).toHaveLength(grouped.length);
  });

  it("filters a group by every event in it", () => {
    const signIn = auditEventGroups.find((group) => group.id === "sign_in");

    expect(signIn && groupFilter(signIn)).toBe(
      "member.signed_in,member.sign_in_failed,member.sign_in_locked,member.sessions_revoked",
    );
  });

  it("shows a member's name without their address, and a token's name", () => {
    expect(actorName("Ada Lovelace <ada@acme.com>")).toBe("Ada Lovelace");
    expect(actorName("Ada Lovelace via Token CI (dmi_live_Ab12)")).toBe("Ada Lovelace via Token CI (dmi_live_Ab12)");
  });

  it("says how an actor reached dimmi and from where", () => {
    expect(actorDetail({ actor: actor("member", "session"), ip: "203.0.113.4" })).toBe("Browser · 203.0.113.4");
    expect(actorDetail({ actor: actor("member", "personal_token"), ip: null })).toBe("API token");
    expect(actorDetail({ actor: actor("system", null), ip: null })).toBe("Automatic");
    expect(actorDetail({ actor: actor("member", null), ip: "203.0.113.4" })).toBe("203.0.113.4");
  });

  it("writes a time with its date", () => {
    expect(formatTime("2026-11-01T10:00:00Z", "en-US")).toContain("2026");
  });
});

describe("auditPath", () => {
  it("asks for a page narrowed by event and actor after the cursor", () => {
    const path = auditPath({ event: "member.left", actor: "mem_1" }, "abc", 25);

    expect(path).toBe("/audit-logs?limit=25&filter%5Bevent%5D=member.left&filter%5Bactor%5D=mem_1&cursor=abc");
  });

  it("leaves out what is not set", () => {
    expect(auditPath({}, null, 10)).toBe("/audit-logs?limit=10");
  });

  it("narrows by time", () => {
    const path = auditPath({ from: "2026-11-01T00:00:00.000Z", to: "2026-11-02T00:00:00.000Z" }, null, 10);

    expect(path).toBe(
      "/audit-logs?limit=10&filter%5Bcreated_after%5D=2026-11-01T00%3A00%3A00.000Z" +
        "&filter%5Bcreated_before%5D=2026-11-02T00%3A00%3A00.000Z",
    );
  });
});

describe("exportPath", () => {
  it("asks for the file with the filters on screen", () => {
    expect(exportPath({ event: "member.left,member.joined", from: "2026-11-01T00:00:00.000Z" })).toBe(
      "/audit-logs?format=csv&filter%5Bevent%5D=member.left%2Cmember.joined" +
        "&filter%5Bcreated_after%5D=2026-11-01T00%3A00%3A00.000Z",
    );
  });
});
