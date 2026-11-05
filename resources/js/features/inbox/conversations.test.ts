import { describe, expect, it } from "vitest";

import type { Message } from "@/lib/api/inbox";

import { toEntry, waitedFor } from "./conversations";

describe("waitedFor", () => {
  const now = Date.parse("2026-11-04T12:00:00Z");

  it("reads in minutes, then hours, then days", () => {
    expect(waitedFor("2026-11-04T11:59:40Z", now)).toBe("1 m");
    expect(waitedFor("2026-11-04T11:15:00Z", now)).toBe("45 m");
    expect(waitedFor("2026-11-04T09:00:00Z", now)).toBe("3 h");
    expect(waitedFor("2026-11-01T12:00:00Z", now)).toBe("3 d");
  });
});

function reply(changes: Partial<Message>): Message {
  return {
    id: "msg_1",
    conversation_id: "conv_1",
    type: "message",
    author: { type: "member", id: "mem_1", name: "Sam" },
    body: "It went out today.",
    body_format: "markdown",
    has_original: false,
    truncated: false,
    delivery_status: "sent",
    delivery_error: null,
    event: null,
    deleted_at: null,
    created_at: new Date().toISOString(),
    ...changes,
  } as Message;
}

describe("toEntry", () => {
  it("carries why a reply was not delivered", () => {
    const entry = toEntry(reply({ delivery_status: "failed", delivery_error: "The address bounced." }));

    expect(entry.kind !== "event" && entry.failed).toBe("The address bounced.");
  });

  it("marks a delivered reply as nothing failed", () => {
    const entry = toEntry(reply({}));

    expect(entry.kind !== "event" && entry.failed).toBeNull();
  });
});
