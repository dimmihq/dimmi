/*
 * Which conversations a page of the inbox lists. The address names the
 * view: a fixed one such as mine, or the public id of a team, a teammate,
 * an inbox or a saved view, or a ticket view (which lists tickets, not
 * conversations). Snoozed is the person's own conversations
 * that sleep. Everything that reads the address goes through viewParams,
 * so the list, its counts and the sidebar agree on what a view is.
 */
import { isTicketView } from "@/features/tickets/views";
import { conversationSorts } from "@/lib/api/inbox";

export type ViewKind = "fixed" | "team" | "member" | "inbox" | "custom" | "tickets";

export interface ViewParams {
  kind: ViewKind;
  /*
   * filter is what the list sends as filter[view].
   */
  filter: string;
  /*
   * status is the status the view always shows, when it has one of its
   * own.
   */
  status?: "snoozed";
}

const fixed = ["mine", "unassigned", "all", "mentions", "spam", "deleted"] as const;

export type FixedView = (typeof fixed)[number] | "snoozed";

const prefixes: [string, ViewKind, (id: string) => string][] = [
  ["team_", "team", (id) => `team:${id}`],
  ["mem_", "member", (id) => `member:${id}`],
  ["inbox_", "inbox", (id) => `inbox:${id}`],
  ["cvw_", "custom", (id) => id],
];

/*
 * viewParams returns what the view lists, or null for an address that
 * names no view.
 */
export function viewParams(view: string): ViewParams | null {
  if ((fixed as readonly string[]).includes(view)) {
    return { kind: "fixed", filter: view };
  }
  if (isTicketView(view)) {
    return { kind: "tickets", filter: view };
  }
  if (view === "snoozed") {
    return { kind: "fixed", filter: "mine", status: "snoozed" };
  }
  for (const [prefix, kind, filter] of prefixes) {
    if (view.startsWith(prefix) && /^[a-z]+_[0-9a-z]{26}$/.test(view)) {
      return { kind, filter: filter(view) };
    }
  }

  return null;
}

export type StatusChoice = "open" | "snoozed" | "closed" | "any";

/*
 * listStatus is the status the list asks for: the view's own when it
 * has one, otherwise the person's choice, open by default.
 */
export function listStatus(params: ViewParams, chosen: StatusChoice | undefined): StatusChoice {
  return params.status ?? chosen ?? "open";
}

/*
 * sortsFor returns the orders a list of the status can have: snooze ends
 * only while it shows snoozed conversations.
 */
export function sortsFor(status: StatusChoice) {
  return conversationSorts.filter((sort) => sort.value !== "snoozed_until" || status === "snoozed");
}
