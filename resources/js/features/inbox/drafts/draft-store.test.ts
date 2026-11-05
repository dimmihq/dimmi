import { afterEach, describe, expect, it, vi } from "vitest";

import type { Draft } from "@/lib/api/drafts";

import { copyKey, findDraft, newest, readCopy, writeCopy } from "./draft-store";

const server = (body: string, at: string): Draft => ({
  id: "drf_1",
  conversation_id: "conv_1",
  mode: "reply",
  body,
  attachment_ids: [],
  client_updated_at: at,
});

describe("draft store", () => {
  afterEach(() => {
    window.localStorage.clear();
    vi.restoreAllMocks();
  });

  it("names each copy by workspace, conversation and mode", () => {
    expect(copyKey("ws_1", "conv_1", "note")).toBe("dimmi:draft:ws_1:conv_1:note");
    expect(copyKey("ws_1", null, "new")).toBe("dimmi:draft:ws_1:new:new");
  });

  it("keeps a copy, drops an empty one and ignores what it cannot read", () => {
    const key = copyKey("ws_1", "conv_1", "reply");
    writeCopy(key, { body: "Hi", at: "2026-01-01T00:00:00Z" });
    expect(readCopy(key)).toEqual({ body: "Hi", at: "2026-01-01T00:00:00Z" });

    writeCopy(key, { body: "", at: "2026-01-01T00:00:01Z" });
    expect(readCopy(key)).toBeNull();

    window.localStorage.setItem(key, "{nope");
    expect(readCopy(key)).toBeNull();
    window.localStorage.setItem(key, JSON.stringify({ body: 3 }));
    expect(readCopy(key)).toBeNull();
    writeCopy(key, null);
    expect(window.localStorage.getItem(key)).toBeNull();
  });

  it("carries on when storage is blocked", () => {
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("blocked");
    });

    expect(readCopy("k")).toBeNull();
    expect(() => writeCopy("k", { body: "x", at: "2026-01-01T00:00:00Z" })).not.toThrow();
  });

  it("picks the newer of the copy and the server's draft", () => {
    const copy = { body: "Local", at: "2026-01-01T00:00:05Z" };

    expect(newest(copy, server("Server", "2026-01-01T00:00:09Z")).body).toBe("Server");
    expect(newest(copy, server("Server", "2026-01-01T00:00:01Z")).body).toBe("Local");
    expect(newest(copy, server("Server", "2026-01-01T00:00:05Z")).body).toBe("Local");
    expect(newest(null, server("Server", "2026-01-01T00:00:01Z")).body).toBe("Server");
    expect(newest(copy, undefined)).toBe(copy);
    expect(newest(null, undefined).body).toBe("");
  });

  it("finds the server's draft of the conversation and mode", () => {
    const drafts = [
      server("A", "2026-01-01T00:00:00Z"),
      { ...server("B", "2026-01-01T00:00:00Z"), mode: "note" as const },
    ];

    expect(findDraft(drafts, "conv_1", "note")?.body).toBe("B");
    expect(findDraft(drafts, "conv_2", "reply")).toBeUndefined();
    expect(findDraft(undefined, "conv_1", "reply")).toBeUndefined();
  });
});
