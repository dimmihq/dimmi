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

    expect("author" in entry && entry.failed).toBe("The address bounced.");
  });

  it("marks a delivered reply as nothing failed", () => {
    const entry = toEntry(reply({}));

    expect("author" in entry && entry.failed).toBeNull();
  });

  it("carries an action the agent asked to run", () => {
    const call = {
      id: "aax_1",
      action: { name: "refund_invoice", title: "Refund an invoice" },
      state: "pending_approval" as const,
      summary: "Refund $144.00",
      arguments: { invoice_id: "inv_1001" },
      expires_at: null,
    };
    const said = toEntry(reply({ author: { type: "agent", id: "", name: "dimmi" }, ai: { action_call: call } }));
    expect(said.kind).toBe("dimmi");
    expect("call" in said && said.call?.id).toBe("aax_1");

    const decided = toEntry(
      reply({ author: { type: "agent", id: "", name: "dimmi" }, ai: { action_call: { ...call, state: "approved" } } }),
    );
    expect("call" in decided && decided.call).toBeFalsy();

    const event = toEntry(
      reply({
        type: "event",
        body: null,
        event: {
          name: "ai.action_call_declined",
          actor: { type: "member", id: "mem_1", name: "Grace" },
          call_id: "aax_1",
          action: { id: "aac_1", title: "Refund an invoice" },
          state: "declined",
          summary: "Refund $144.00",
        },
      }),
    );
    expect(event).toMatchObject({ kind: "call", call: { state: "declined", decided_by: { name: "Grace" } } });

    const asked = toEntry(
      reply({
        type: "event",
        body: null,
        event: {
          name: "ai.action_call_requested",
          actor: { type: "agent", id: "", name: "dimmi" },
          action: { id: "aac_1", title: "Refund an invoice" },
        },
      }),
    );
    expect(asked).toMatchObject({ kind: "event", text: "dimmi asked to run Refund an invoice" });
  });
});
