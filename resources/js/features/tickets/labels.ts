/*
 * The words and colours tickets show everywhere: each state group's name
 * and pill colour, each kind's name and icon, each field type's name,
 * and how a field value reads in a row. Names come from the tickets
 * message file, so every screen says the same thing.
 */
import {
  type RemixiconComponentType,
  RiAlarmWarningLine,
  RiBriefcase4Line,
  RiBugLine,
  RiCoupon3Line,
  RiCustomerService2Line,
  RiFileList3Line,
  RiFlashlightLine,
  RiQuestionLine,
  RiRefund2Line,
  RiRouteLine,
  RiStackLine,
} from "@remixicon/react";

import type { ChipColor } from "@/components/base/badges/chip";
import type {
  FieldDataType,
  IssueStatus,
  StateGroup,
  TicketEvent,
  TicketKind,
  TicketPriority,
} from "@/lib/api/tickets";
import { i18n } from "@/lib/i18n";

export function groupLabel(group: StateGroup): string {
  return i18n.t(`tickets:groups.${group}`);
}

export function groupHint(group: StateGroup): string {
  return i18n.t(`tickets:groupHints.${group}`);
}

export function kindLabel(kind: TicketKind): string {
  return i18n.t(`tickets:kinds.${kind}`);
}

export function kindHint(kind: TicketKind): string {
  return i18n.t(`tickets:kindHints.${kind}`);
}

export function dataTypeLabel(type: FieldDataType): string {
  return i18n.t(`tickets:dataTypes.${type}`);
}

export function priorityLabel(priority: TicketPriority): string {
  return i18n.t(`tickets:priorities.${priority}`);
}

export function issueStatusLabel(status: IssueStatus): string {
  return i18n.t(`tickets:issueStatuses.${status}`);
}

/*
 * groupColor is the pill colour of a state group: grey while new, blue
 * while worked, yellow while it waits on the customer, purple while
 * another team holds it, green once resolved.
 */
export const groupColor: Record<StateGroup, ChipColor> = {
  submitted: "neutral",
  in_progress: "blue",
  waiting_customer: "yellow",
  waiting_team: "purple",
  resolved: "lime",
};

/*
 * typeIcons are the icons a type can pick, by the name the API stores.
 * A name the app does not know shows its kind's icon.
 */
export const typeIcons: Record<string, RemixiconComponentType> = {
  ticket: RiCoupon3Line,
  question: RiQuestionLine,
  bug: RiBugLine,
  refund: RiRefund2Line,
  briefcase: RiBriefcase4Line,
  support: RiCustomerService2Line,
  route: RiRouteLine,
  list: RiFileList3Line,
  "alarm-warning": RiAlarmWarningLine,
  stack: RiStackLine,
  flash: RiFlashlightLine,
};

const kindIcons: Record<TicketKind, RemixiconComponentType> = {
  customer: RiCoupon3Line,
  back_office: RiBriefcase4Line,
  tracker: RiRouteLine,
};

/*
 * typeIcon returns the icon a type shows: its own when the app knows the
 * name, otherwise its kind's.
 */
export function typeIcon(icon: string, kind: TicketKind): RemixiconComponentType {
  return typeIcons[icon] ?? typeIcons[icon.replace(/^ri-|-line$/g, "")] ?? kindIcons[kind];
}

/*
 * fieldText is how a field value reads in a row or a preview.
 */
export function fieldText(value: unknown): string {
  if (value === null || value === undefined || value === "") {
    return "";
  }
  if (Array.isArray(value)) {
    return value.map(String).join(", ");
  }
  if (typeof value === "boolean") {
    return value ? "Yes" : "No";
  }
  if (typeof value === "object") {
    const named = value as { name?: string; amount?: number; currency?: string };
    if (named.amount !== undefined) {
      return `${named.amount} ${named.currency ?? ""}`.trim();
    }
    return named.name ?? JSON.stringify(value);
  }

  return String(value);
}

/*
 * dueIn says how long until a time, or how long since it passed.
 */
export function dueIn(at: string, now = Date.now()): { text: string; late: boolean } {
  const minutes = Math.round((new Date(at).getTime() - now) / 60000);
  const late = minutes < 0;
  const span = Math.abs(minutes);
  const hours = Math.floor(span / 60);
  const days = Math.floor(hours / 24);
  let text = `${span}m`;
  if (days >= 1) {
    text = `${days}d ${hours % 24}h`;
  } else if (hours >= 1) {
    text = `${hours}h ${span % 60}m`;
  }

  return { text, late };
}

/*
 * eventText says what one history entry changed.
 */
export function eventText(event: TicketEvent): string {
  const d = event.data;
  switch (event.kind) {
    case "created":
      return "Created the ticket";
    case "state":
      return `Moved from ${String(d.from_name ?? "")} to ${String(d.to_name ?? "")}`;
    case "note":
      return String(d.body ?? "");
    case "type":
      return "Changed the type";
    case "assign":
      return "Changed who holds it";
    case "link":
      return "Linked a ticket or conversation";
    case "unlink":
      return "Removed a link";
    case "close_loop":
      return "The work behind it is done: close the loop";
    case "customer_notified":
      return "Told the customer the new state";
    case "update": {
      const changes = (d.changes ?? {}) as Record<string, { to?: unknown }>;
      return `Changed ${Object.entries(changes)
        .map(([key, c]) => `${key.replace(/^fields\./, "")} to ${fieldText(c.to) || "nothing"}`)
        .join(", ")}`;
    }
  }

  return event.kind.replace(/_/g, " ");
}
