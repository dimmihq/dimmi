import { describe, expect, it } from "vitest";

import type { Integration, SlackRoom } from "@/lib/api/integrations";

import { draftOf, integrationHealth, landingOf, roomChange, roomLine } from "./integrations";

const room: SlackRoom = {
  id: "chr_1",
  object: "channel_room",
  channel_id: "chn_1",
  external_id: "C1",
  name: "#acme-support",
  status: "active",
  mode: "shared",
  trigger: "author_window",
  is_shared_external: true,
  settings: { agent_mode: "draft" },
  last_error: null,
  removed_at: null,
};

describe("roomChange", () => {
  it("sends nothing when nothing changed", () => {
    expect(roomChange(room, draftOf(room))).toBeNull();
  });

  it("sends only what changed and clears settings put back to their default", () => {
    const draft = { ...draftOf(room), trigger: "ai" as const, agent_mode: "auto" as const, agent_delay_seconds: 60 };

    expect(roomChange(room, draft)).toEqual({
      trigger: "ai",
      settings: { agent_mode: null, agent_delay_seconds: 60 },
    });
  });

  it("leaves the trigger to the server when the room becomes team only", () => {
    const draft = { ...draftOf(room), mode: "internal" as const, trigger: "every_message" as const };

    expect(roomChange(room, draft)).toEqual({ mode: "internal" });
  });
});

describe("landingOf", () => {
  it("reads a connected installation or a declined one", () => {
    expect(landingOf("?connected=itg_1")).toEqual({ kind: "connected", id: "itg_1" });
    expect(landingOf("?error=denied")).toEqual({
      kind: "error",
      message: "The connection was declined, so nothing changed.",
    });
    expect(landingOf("?error=strange")?.kind).toBe("error");
    expect(landingOf("")).toBeNull();
  });
});

describe("integrationHealth", () => {
  const base = { status: "active", features: [] } as unknown as Integration;

  it("tells working, missing permissions and broken apart", () => {
    expect(integrationHealth(base)).toBe("connected");
    expect(
      integrationHealth({
        ...base,
        features: [{ name: "operator", status: "blocked_missing_scope", missing_scopes: ["im:history"] }],
      }),
    ).toBe("needs_permissions");
    expect(integrationHealth({ ...base, status: "error" })).toBe("broken");
  });
});

describe("roomLine", () => {
  it("asks for the app to be invited back to a removed room", () => {
    expect(roomLine(room)).toBe("Shared with a customer · Each new question");
    expect(roomLine({ ...room, status: "removed" })).toContain("Invite @dimmi to the channel again");
  });
});
