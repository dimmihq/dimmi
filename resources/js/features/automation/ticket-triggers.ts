/*
 * The triggers of a customer's ticket. Each runs on the ticket's
 * conversation; a ticket without a conversation, such as back-office
 * work, runs no workflow, though work linked to customers reaches their
 * conversations. Closing the loop is a prompt, as in Plain: nothing is
 * sent until a workflow's message step says what the customer is told.
 */
import {
  RiCheckDoubleLine,
  RiCodeBoxLine,
  RiFileList3Line,
  RiLinksLine,
  RiLoopRightLine,
  RiReplyAllLine,
  RiTicket2Line,
} from "@remixicon/react";

import type { Trigger } from "./triggers";

export type TicketTriggerKind =
  | "ticket.created"
  | "ticket.state_changed"
  | "ticket.field_changed"
  | "ticket.customer_replied"
  | "ticket.linked"
  | "ticket.issue_updated"
  | "ticket.close_loop";

export const ticketTriggers: Record<TicketTriggerKind, Trigger> = {
  "ticket.created": {
    group: "ticket",
    name: "Ticket is created",
    detail: "Route it, or escalate it at once.",
    icon: RiTicket2Line,
    runs: true,
    talks: ["message"],
  },
  "ticket.state_changed": {
    group: "ticket",
    name: "Ticket state changes",
    detail: "Tell the customer, or hand it on.",
    icon: RiLoopRightLine,
    runs: true,
    talks: ["message"],
  },
  "ticket.field_changed": {
    group: "ticket",
    name: "Ticket fields change",
    detail: "React to a new severity or amount.",
    icon: RiFileList3Line,
    runs: true,
    talks: [],
  },
  "ticket.customer_replied": {
    group: "ticket",
    name: "Customer replies on a ticket",
    detail: "Bring it back to the team.",
    icon: RiReplyAllLine,
    runs: true,
    talks: [],
  },
  "ticket.linked": {
    group: "ticket",
    name: "Ticket is linked",
    detail: "When it joins a tracker or other work.",
    icon: RiLinksLine,
    runs: true,
    talks: [],
  },
  "ticket.issue_updated": {
    group: "ticket",
    name: "Linked issue changes",
    detail: "Follow the engineering issue behind it.",
    icon: RiCodeBoxLine,
    runs: true,
    talks: ["message"],
  },
  "ticket.close_loop": {
    group: "ticket",
    name: "Ticket needs the loop closed",
    detail: "Tell the customer the fix shipped, then resolve.",
    icon: RiCheckDoubleLine,
    runs: true,
    talks: ["message"],
  },
};
