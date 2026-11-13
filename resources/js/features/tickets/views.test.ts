import { describe, expect, it } from "vitest";

import type { TicketType } from "@/lib/api/tickets";

import { isTicketView, ticketViewCount, ticketViewFilter } from "./views";

const id = "tktt_01jb8z2k3m4n5p6q7r8s9t0vwx";

function type(kind: TicketType["kind"], open: number, tid = id): TicketType {
  return { id: tid, kind, open_tickets: open, archived_at: null } as TicketType;
}

describe("ticket views", () => {
  it("names the fixed views and type ids only", () => {
    for (const view of ["tickets", "tickets-waiting", "trackers", id]) {
      expect(isTicketView(view)).toBe(true);
    }
    for (const view of ["mine", "incidents", "tktt_", `${id}x`, "team_01jb8z2k3m4n5p6q7r8s9t0vwx"]) {
      expect(isTicketView(view)).toBe(false);
    }
  });

  it("asks for open tickets of the view", () => {
    expect(ticketViewFilter("tickets-waiting")).toEqual({ group: "waiting_team", active: true });
    expect(ticketViewFilter(id)).toEqual({ type_id: id, active: true });
  });

  it("counts open tickets from the types", () => {
    const types = [type("customer", 3), type("back_office", 2, "b"), type("tracker", 6, "t")];
    expect(ticketViewCount("tickets", types)).toBe(5);
    expect(ticketViewCount("trackers", types)).toBe(6);
    expect(ticketViewCount(id, types)).toBe(3);
    expect(ticketViewCount("tickets-waiting", types)).toBeUndefined();
  });
});
