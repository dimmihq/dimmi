import type { Session } from "@/lib/api/sessions";

import { lastActive, sessionDetail } from "./session-detail";

const now = Date.parse("2026-11-03T12:00:00Z");

function session(changes: Partial<Session>): Session {
  return {
    id: "ses_1",
    object: "session",
    kind: "browser",
    current: false,
    auth_method: "password",
    auth_methods: ["password"],
    two_factor_used: false,
    authenticated_at: "2026-11-01T10:00:00Z",
    device_label: "Chrome on macOS",
    client: null,
    platform: null,
    app_version: null,
    ip: null,
    country: null,
    last_activity_at: "2026-11-03T11:59:30Z",
    created_at: "2026-11-01T10:00:00Z",
    expires_at: "2026-12-01T10:00:00Z",
    ...changes,
  };
}

describe("lastActive", () => {
  it("says just now under a minute", () => {
    expect(lastActive("2026-11-03T11:59:30Z", now)).toBe("Active just now");
  });

  it("uses the largest unit that fits", () => {
    expect(lastActive("2026-11-03T11:45:00Z", now)).toBe("Last active 15 minutes ago");
    expect(lastActive("2026-11-03T09:00:00Z", now)).toBe("Last active 3 hours ago");
    expect(lastActive("2026-11-01T12:00:00Z", now)).toBe("Last active 2 days ago");
  });
});

describe("sessionDetail", () => {
  it("marks the session in use and says where it is", () => {
    expect(sessionDetail(session({ current: true, ip: "203.0.113.9", country: "PT" }), now)).toBe(
      "This session · 203.0.113.9 · PT",
    );
  });

  it("says when another session was used", () => {
    expect(sessionDetail(session({ last_activity_at: "2026-11-02T12:00:00Z" }), now)).toBe("Last active yesterday");
  });
});
