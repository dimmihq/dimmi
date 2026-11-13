import { describe, expect, it } from "vitest";

import type { TicketEvent } from "@/lib/api/tickets";

import { foldLinks } from "./ticket-thread";

function event(id: string, kind: string): TicketEvent {
  return {
    object: "ticket_event",
    id,
    ticket_id: "tkt_1",
    kind,
    actor_type: "member",
    actor_id: null,
    data: {},
    created_at: id,
  };
}

describe("foldLinks", () => {
  it("folds each run of back-to-back links and keeps everything else in place", () => {
    const events = [
      event("1", "created"),
      event("2", "link"),
      event("3", "link"),
      event("4", "note"),
      event("5", "link"),
    ];
    const folded = foldLinks(events, (e) => (e.kind === "link" ? e : null));

    expect(folded.map((item) => (Array.isArray(item) ? item.map((e) => e.id) : item.id))).toEqual([
      "1",
      ["2", "3"],
      "4",
      ["5"],
    ]);
  });
});
