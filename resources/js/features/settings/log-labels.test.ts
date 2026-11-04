import { type AuditEntry, auditPath } from "@/lib/api/audit";

import { actorDetail, actorName, auditEventLabel, formatTime, securityEventLabel } from "./log-labels";

function actor(type: string, via: string | null): AuditEntry["actor"] {
  return { type, id: null, label: "", via: via ? { kind: via, id: null } : null };
}

describe("log labels", () => {
  it("names known events and falls back to the server's name", () => {
    expect(auditEventLabel("member.role_changed")).toBe("Changed a role");
    expect(auditEventLabel("team.created")).toBe("team.created");
    expect(securityEventLabel("login.succeeded")).toBe("Signed in");
    expect(securityEventLabel("something.new")).toBe("something.new");
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
});
