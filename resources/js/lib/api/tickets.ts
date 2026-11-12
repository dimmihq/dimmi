/*
 * Tickets on the server: the workspace's setup (types with their fields
 * and the states each uses, and the states themselves in five fixed
 * groups), tickets with their history, notes and links, a conversation
 * turned into a ticket, back-office work escalated from one, and the
 * AI's suggested field values. A ticket carries a version; a change
 * sends it in If-Match so two people never overwrite each other, and a
 * clash comes back as a 409 the page offers to reload.
 */
import { queryOptions } from "@tanstack/react-query";

import { request, requestPage } from "./client";

export type TicketKind = "customer" | "back_office" | "tracker" | "incident";
export type StateGroup = "submitted" | "in_progress" | "waiting_customer" | "waiting_team" | "resolved";
export type TicketPriority = "none" | "low" | "medium" | "high" | "urgent";
export type FieldDataType =
  | "short_text"
  | "long_text"
  | "number"
  | "decimal"
  | "currency"
  | "boolean"
  | "date"
  | "single_choice"
  | "multiple_choice"
  | "file"
  | "member"
  | "company";
export type LinkKind = "back_office" | "tracker" | "follow_up" | "related";
export type IssueStatus = "backlog" | "unstarted" | "started" | "done" | "canceled";

/*
 * stateGroups are the five groups in the order a ticket moves through
 * them.
 */
export const stateGroups: StateGroup[] = ["submitted", "in_progress", "waiting_customer", "waiting_team", "resolved"];
export const ticketKinds: TicketKind[] = ["customer", "back_office", "tracker", "incident"];
export const ticketPriorities: TicketPriority[] = ["none", "low", "medium", "high", "urgent"];
export const fieldDataTypes: FieldDataType[] = [
  "short_text",
  "long_text",
  "number",
  "decimal",
  "currency",
  "boolean",
  "date",
  "single_choice",
  "multiple_choice",
  "file",
  "member",
  "company",
];

export interface TicketState {
  object: "ticket_state";
  id: string;
  group: StateGroup;
  name: string;
  customer_label: string;
  notify_customer: boolean;
  purpose: string | null;
  position: number;
  archived_at: string | null;
}

/*
 * ShowWhen shows a field only when another field, by key, has a value,
 * or one of the listed values.
 */
export interface ShowWhen {
  field: string;
  equals?: string[];
}

export interface TicketField {
  object: "ticket_field";
  id: string;
  type_id: string;
  key: string;
  label: string;
  description: string;
  data_type: FieldDataType;
  options: string[];
  currency: string | null;
  required_create: boolean;
  required_create_customer: boolean;
  required_resolve: boolean;
  visible_customer: boolean;
  editable_customer: boolean;
  show_when: ShowWhen | null;
  ai_hint: string | null;
  ai_auto: boolean;
  position: number;
  archived_at: string | null;
}

export interface TicketType {
  object: "ticket_type";
  id: string;
  kind: TicketKind;
  name: string;
  icon: string;
  description: string;
  inbox_id: string | null;
  team_id: string | null;
  customer_visible: boolean;
  company_sharing: boolean;
  reopen_days: number;
  auto_resolve_days: number | null;
  auto_resolve_message: string | null;
  ai_auto_fill: boolean;
  state_ids: string[];
  default_state_id: string | null;
  default_resolved_state_id: string | null;
  fields: TicketField[];
  open_tickets: number;
  total_tickets: number;
  position: number;
  archived_at: string | null;
}

export interface StateRef {
  id: string;
  name: string;
  customer_label: string;
  group: StateGroup;
}

export interface LinkedTicket {
  id: string;
  reference: string;
  title: string;
  kind: TicketKind;
  type_name: string;
  state: StateRef;
  assignee_id: string | null;
  updated_at: string;
}

export interface TicketLink {
  object: "ticket_link";
  id: string;
  kind: LinkKind;
  from_ticket_id: string;
  to_ticket_id: string | null;
  conversation_id: string | null;
  ticket: LinkedTicket | null;
  created_at: string;
}

export interface TicketIssue {
  object: "ticket_issue";
  id: string;
  ticket_id: string;
  provider: string;
  key: string;
  url: string;
  title: string;
  status: string;
  status_group: string;
  team_name: string;
  warning: string | null;
  sync_error: string | null;
}

export interface SLAClock {
  object: "sla_clock";
  id: string;
  target: string;
  state: string;
  started_at: string;
  due_at: string;
  warn_at: string | null;
  paused_at: string | null;
  breached_at: string | null;
  stopped_at: string | null;
}

export interface Ticket {
  object: "ticket";
  id: string;
  reference: string;
  number: number;
  kind: TicketKind;
  type_id: string;
  type: { id: string; name: string; icon: string; kind: TicketKind };
  state_id: string;
  state: StateRef;
  group: StateGroup;
  title: string;
  description: string;
  fields: Record<string, unknown>;
  suggestions: Record<string, unknown>;
  priority: TicketPriority;
  assignee_id: string | null;
  team_id: string | null;
  inbox_id: string;
  conversation_id: string | null;
  contact_id: string | null;
  company_id: string | null;
  external_id: string | null;
  close_loop: boolean;
  customer_visible: boolean;
  version: number;
  linked_conversations: number;
  issues: TicketIssue[];
  created_by_type: string;
  state_since: string;
  resolved_at: string | null;
  created_at: string;
  updated_at: string;
  deleted_at: string | null;
  links?: TicketLink[];
  sla_clocks?: SLAClock[];
  impact?: { conversations: number; customers: number; companies: number; revenue: number };
}

export interface TicketEvent {
  object: "ticket_event";
  id: string;
  ticket_id: string;
  kind: string;
  actor_type: string;
  actor_id: string | null;
  data: Record<string, unknown>;
  created_at: string;
}

/*
 * ConversationTickets is a conversation's own ticket, when it is one,
 * and the trackers and incidents it joined.
 */
export interface ConversationTickets {
  ticket: Ticket | null;
  trackers: Ticket[];
}

/*
 * TicketFilter narrows the ticket list. Fields matches field values
 * exactly, by key.
 */
export interface TicketFilter {
  type_id?: string;
  kind?: string;
  state_id?: string;
  group?: string;
  active?: boolean;
  assignee_id?: string;
  team_id?: string;
  company_id?: string;
  conversation_id?: string;
  priority?: TicketPriority;
  issue_status?: IssueStatus;
  fields?: Record<string, unknown>;
  q?: string;
  cursor?: string;
  limit?: number;
}

/*
 * ticketParams writes a filter as the list's query string, leaving out
 * what it does not set.
 */
export function ticketParams(filter: TicketFilter): string {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(filter)) {
    if (value === undefined || value === "" || value === false) {
      continue;
    }
    if (key === "fields") {
      if (Object.keys(value as object).length > 0) {
        params.set("fields", JSON.stringify(value));
      }
      continue;
    }
    params.set(key, String(value));
  }

  return params.toString();
}

export const ticketKeys = {
  all: (workspace: string) => ["workspace", workspace, "tickets"] as const,
  types: (workspace: string) => ["workspace", workspace, "tickets", "types"] as const,
  states: (workspace: string) => ["workspace", workspace, "tickets", "states"] as const,
  list: (workspace: string, filter: TicketFilter) => ["workspace", workspace, "tickets", "list", filter] as const,
  one: (workspace: string, ticket: string) => ["workspace", workspace, "tickets", "one", ticket] as const,
  events: (workspace: string, ticket: string) => ["workspace", workspace, "tickets", "events", ticket] as const,
  conversation: (workspace: string, conversation: string) =>
    ["workspace", workspace, "tickets", "conversation", conversation] as const,
};

export function ticketTypesQuery(workspace: string) {
  return queryOptions({
    queryKey: ticketKeys.types(workspace),
    queryFn: () => request<TicketType[]>("GET", "/ticket-types", undefined, { workspace }),
    enabled: workspace !== "",
  });
}

export function ticketStatesQuery(workspace: string) {
  return queryOptions({
    queryKey: ticketKeys.states(workspace),
    queryFn: () => request<TicketState[]>("GET", "/ticket-states", undefined, { workspace }),
    enabled: workspace !== "",
  });
}

export function ticketsQuery(workspace: string, filter: TicketFilter) {
  return queryOptions({
    queryKey: ticketKeys.list(workspace, filter),
    queryFn: () => requestPage<Ticket>(`/tickets?${ticketParams(filter)}`, { workspace }),
    enabled: workspace !== "",
  });
}

export function ticketQuery(workspace: string, ticket: string) {
  return queryOptions({
    queryKey: ticketKeys.one(workspace, ticket),
    queryFn: () => request<Ticket>("GET", `/tickets/${encodeURIComponent(ticket)}`, undefined, { workspace }),
    enabled: workspace !== "" && ticket !== "",
  });
}

export function ticketEventsQuery(workspace: string, ticket: string) {
  return queryOptions({
    queryKey: ticketKeys.events(workspace, ticket),
    queryFn: () => requestPage<TicketEvent>(`/tickets/${encodeURIComponent(ticket)}/events`, { workspace }),
    enabled: workspace !== "" && ticket !== "",
  });
}

export function conversationTicketsQuery(workspace: string, conversation: string) {
  return queryOptions({
    queryKey: ticketKeys.conversation(workspace, conversation),
    queryFn: () =>
      request<ConversationTickets>("GET", `/conversations/${conversation}/tickets`, undefined, { workspace }),
    enabled: workspace !== "" && conversation !== "",
  });
}

/*
 * ticketSuggestions asks the AI for a type's field values, read from the
 * conversation. They are suggestions only; nothing is saved.
 */
export function ticketSuggestions(workspace: string, conversation: string, type: string) {
  return request<Record<string, unknown>>(
    "GET",
    `/conversations/${conversation}/ticket-suggestions?type_id=${encodeURIComponent(type)}`,
    undefined,
    { workspace },
  );
}

/*
 * TypeChange is what adding or changing a type sends; kind is read only
 * when the type is added.
 */
export type TypeChange = Partial<
  Pick<
    TicketType,
    | "kind"
    | "name"
    | "icon"
    | "description"
    | "inbox_id"
    | "team_id"
    | "customer_visible"
    | "company_sharing"
    | "reopen_days"
    | "auto_resolve_days"
    | "auto_resolve_message"
    | "ai_auto_fill"
    | "state_ids"
    | "default_state_id"
    | "default_resolved_state_id"
  >
>;

export function createTicketType(workspace: string, change: TypeChange) {
  return request<TicketType>("POST", "/ticket-types", change, { workspace });
}

export function updateTicketType(workspace: string, type: string, change: TypeChange) {
  return request<TicketType>("PATCH", `/ticket-types/${type}`, change, { workspace });
}

export function archiveTicketType(workspace: string, type: string) {
  return request<void>("DELETE", `/ticket-types/${type}`, undefined, { workspace });
}

/*
 * FieldChange is what adding or changing a field sends; key and data
 * type are read only when the field is added.
 */
export type FieldChange = Partial<
  Omit<TicketField, "object" | "id" | "type_id" | "position" | "archived_at" | "show_when">
> & { show_when?: ShowWhen | null };

export function createTicketField(workspace: string, type: string, change: FieldChange) {
  return request<TicketField>("POST", `/ticket-types/${type}/fields`, change, { workspace });
}

export function updateTicketField(workspace: string, field: string, change: FieldChange) {
  return request<TicketField>("PATCH", `/ticket-fields/${field}`, change, { workspace });
}

export function archiveTicketField(workspace: string, field: string) {
  return request<void>("DELETE", `/ticket-fields/${field}`, undefined, { workspace });
}

export function reorderTicketFields(workspace: string, type: string, fieldIds: string[]) {
  return request<TicketField[]>("PATCH", `/ticket-types/${type}/fields`, { field_ids: fieldIds }, { workspace });
}

export type StateChange = Partial<Pick<TicketState, "group" | "name" | "customer_label" | "notify_customer">>;

export function createTicketState(workspace: string, change: StateChange) {
  return request<TicketState>("POST", "/ticket-states", change, { workspace });
}

export function updateTicketState(workspace: string, state: string, change: StateChange) {
  return request<TicketState>("PATCH", `/ticket-states/${state}`, change, { workspace });
}

export function reorderTicketStates(workspace: string, stateIds: string[]) {
  return request<TicketState[]>("PATCH", "/ticket-states", { state_ids: stateIds }, { workspace });
}

export function deleteTicketState(workspace: string, state: string, moveTo: string) {
  return request<void>("DELETE", `/ticket-states/${state}?move_to=${encodeURIComponent(moveTo)}`, undefined, {
    workspace,
  });
}

/*
 * TicketChange is what a ticket update sends: fields merge into the
 * ticket's, a null assignee or team unassigns, and accept_suggestions
 * keeps the AI's values for the keys.
 */
export interface TicketChange {
  state_id?: string;
  type_id?: string;
  title?: string;
  description?: string;
  fields?: Record<string, unknown>;
  priority?: TicketPriority;
  assignee_id?: string | null;
  team_id?: string | null;
  accept_suggestions?: string[];
}

/*
 * updateTicket changes a ticket only if it is still at the version read.
 */
export function updateTicket(workspace: string, ticket: Pick<Ticket, "id" | "version">, change: TicketChange) {
  return request<Ticket>("PATCH", `/tickets/${ticket.id}`, change, {
    workspace,
    headers: { "If-Match": `"${ticket.version}"` },
  });
}

export interface NewTicket {
  type_id: string;
  title: string;
  description?: string;
  state_id?: string;
  fields?: Record<string, unknown>;
  contact_id?: string;
  company_id?: string;
  assignee_id?: string;
  team_id?: string;
  priority?: TicketPriority;
  link_to?: { kind: LinkKind; ticket_id?: string; conversation_id?: string }[];
}

export function createTicket(workspace: string, ticket: NewTicket) {
  return request<Ticket>("POST", "/tickets", ticket, { workspace });
}

export interface Conversion {
  type_id: string;
  title?: string;
  state_id?: string;
  fields?: Record<string, unknown>;
  ai_fill?: boolean;
}

export function convertConversation(workspace: string, conversation: string, conversion: Conversion) {
  return request<Ticket>("POST", `/conversations/${conversation}/ticket`, conversion, { workspace });
}

export interface Escalation {
  type_id: string;
  title: string;
  description?: string;
  fields?: Record<string, unknown>;
  team_id?: string;
  assignee_id?: string;
}

export function escalateTicket(workspace: string, ticket: string, escalation: Escalation) {
  return request<Ticket>("POST", `/tickets/${ticket}/escalations`, escalation, { workspace });
}

export function addTicketNote(workspace: string, ticket: string, body: string) {
  return request<TicketEvent>("POST", `/tickets/${ticket}/notes`, { body }, { workspace });
}

export function addTicketLink(
  workspace: string,
  ticket: string,
  link: { kind: LinkKind; ticket_id?: string; conversation_id?: string },
) {
  return request<TicketLink>("POST", `/tickets/${ticket}/links`, link, { workspace });
}

export function removeTicketLink(workspace: string, link: string) {
  return request<void>("DELETE", `/ticket-links/${link}`, undefined, { workspace });
}
