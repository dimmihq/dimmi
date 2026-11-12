/*
 * The inbox's ticket views: every open ticket, one view per type, the
 * tickets another team holds, and the open trackers and incidents. A
 * view's address is a fixed name or a type's public id, and each one
 * says which tickets the list asks for. Counts come from the types'
 * open totals, so the picker needs no extra request.
 */
import type { TicketFilter, TicketType } from "@/lib/api/tickets";
import { i18n } from "@/lib/i18n";

export const fixedTicketViews = ["tickets", "tickets-waiting", "trackers", "incidents"] as const;
export type FixedTicketView = (typeof fixedTicketViews)[number];

/*
 * isTicketView reports whether a view address names a ticket view.
 */
export function isTicketView(view: string): boolean {
  return (fixedTicketViews as readonly string[]).includes(view) || /^tktt_[0-9a-z]{26}$/.test(view);
}

/*
 * ticketViewFilter is what a ticket view lists: open tickets only.
 */
export function ticketViewFilter(view: string): TicketFilter {
  switch (view) {
    case "tickets":
      return { kind: "customer,back_office", active: true };
    case "tickets-waiting":
      return { group: "waiting_team", active: true };
    case "trackers":
      return { kind: "tracker", active: true };
    case "incidents":
      return { kind: "incident", active: true };
  }

  return { type_id: view, active: true };
}

/*
 * ticketViewName is what a ticket view is called: a fixed view's name,
 * or its type's.
 */
export function ticketViewName(view: string, types: TicketType[] | undefined): string | undefined {
  switch (view) {
    case "tickets":
      return i18n.t("tickets:views.all");
    case "tickets-waiting":
      return i18n.t("tickets:views.waiting");
    case "trackers":
      return i18n.t("tickets:views.trackers");
    case "incidents":
      return i18n.t("tickets:views.incidents");
  }

  return types?.find((t) => t.id === view)?.name;
}

/*
 * ticketViewCount is how many open tickets a view holds, as the types
 * report them; the waiting view has no total of its own.
 */
export function ticketViewCount(view: string, types: TicketType[] | undefined): number | undefined {
  if (!types || view === "tickets-waiting") {
    return undefined;
  }
  const sum = (kinds: string[]) =>
    types.filter((t) => kinds.includes(t.kind)).reduce((total, t) => total + t.open_tickets, 0);
  switch (view) {
    case "tickets":
      return sum(["customer", "back_office"]);
    case "trackers":
      return sum(["tracker"]);
    case "incidents":
      return sum(["incident"]);
  }

  return types.find((t) => t.id === view)?.open_tickets;
}

/*
 * typeViews are the types that get a view of their own: live customer
 * and back-office types.
 */
export function typeViews(types: TicketType[] | undefined): TicketType[] {
  return (types ?? []).filter((t) => !t.archived_at && (t.kind === "customer" || t.kind === "back_office"));
}
