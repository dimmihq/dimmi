/*
 * The ticket facts branch conditions read: the conversation's ticket
 * type, state and its group, priority, whether it waits to close the
 * loop, and the status of the issues linked to it; and what the ticket
 * triggers carry about the change. A conversation that is no ticket
 * has none of them, so "is unknown" finds plain conversations.
 */
import { stateGroups } from "@/lib/api/tickets";

import type { ConditionField } from "./conditions";

const groupNames: Record<string, string> = {
  submitted: "Submitted",
  in_progress: "In progress",
  waiting_customer: "Waiting on the customer",
  waiting_team: "Waiting on the team",
  resolved: "Resolved",
};

const groups: Record<string, string> = Object.fromEntries(stateGroups.map((group) => [group, groupNames[group]]));

const issueGroups: Record<string, string> = {
  backlog: "Backlog",
  unstarted: "To do",
  started: "In progress",
  done: "Done",
  canceled: "Canceled",
};

export const ticketConditionFields: Record<string, ConditionField> = {
  "ticket.type_id": { name: "Ticket type", type: "choice", source: "ticketTypes" },
  "ticket.state_id": { name: "Ticket state", type: "choice", source: "ticketStates" },
  "ticket.state_group": { name: "Ticket state group", type: "choice", options: groups },
  "ticket.priority": {
    name: "Ticket priority",
    type: "choice",
    options: { none: "None", low: "Low", medium: "Medium", high: "High", urgent: "Urgent" },
  },
  "ticket.close_loop": { name: "Ticket waits to close the loop", type: "yes_no" },
  "ticket.issue_status_groups": { name: "Linked issue status", type: "set", options: issueGroups },
  "trigger.to_group": {
    name: "Moved into",
    type: "choice",
    options: groups,
    triggers: ["ticket.state_changed"],
  },
  "trigger.from_group": {
    name: "Moved out of",
    type: "choice",
    options: groups,
    triggers: ["ticket.state_changed"],
  },
  "trigger.fields": { name: "Changed field keys", type: "set", triggers: ["ticket.field_changed"] },
  "trigger.link_kind": {
    name: "Link kind",
    type: "choice",
    options: { back_office: "Back-office work", tracker: "Tracker", follow_up: "Follow-up", related: "Related" },
    triggers: ["ticket.linked"],
  },
  "trigger.issue_status_group": {
    name: "Issue moved to",
    type: "choice",
    options: issueGroups,
    triggers: ["ticket.issue_updated"],
  },
};
