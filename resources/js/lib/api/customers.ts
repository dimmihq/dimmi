/*
 * The customers a team talks to: contacts, the companies they work at,
 * and the context the panel beside a conversation shows. Contact and
 * company queries are cached under the workspace, so a change made here
 * refreshes every screen that shows the same record.
 */
import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";

import { request, requestPage } from "./client";
import type { Conversation } from "./inbox";
import type { Tag } from "./tags";

export type Attributes = Record<string, unknown>;

export interface Location {
  country?: string;
  region?: string;
  city?: string;
}

export interface CompanySummary {
  id: string;
  object: "company";
  name: string;
  domain: string | null;
}

export interface Contact {
  id: string;
  object: "contact";
  kind: "user" | "lead";
  name: string | null;
  email: string | null;
  email_verified: boolean;
  external_id: string | null;
  phone: string | null;
  avatar_url: string | null;
  locale: string | null;
  timezone: string | null;
  custom_attributes: Attributes;
  blocked: boolean;
  notes: string | null;
  location: Location | null;
  company: CompanySummary | null;
  tags: Tag[];
  first_seen_at: string | null;
  last_seen_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface Identity {
  id: string;
  type: string;
  value: string;
  verified: boolean;
}

export interface Source {
  page_url?: string;
  browser?: string;
  os?: string;
  device?: string;
  app_version?: string;
  ip_country?: string;
}

export interface ConversationContext {
  contact: Contact;
  company: (CompanySummary & { custom_attributes: Attributes }) | null;
  identities: Identity[];
  source: Source;
  past_conversations: { data: Conversation[]; total_estimate: number };
  duplicates_count: number;
}

export interface Duplicate {
  contact: { id: string; name: string | null; email: string | null };
  reasons: string[];
}

export interface ContactChanges {
  name?: string | null;
  email?: string | null;
  phone?: string | null;
  notes?: string | null;
  company_id?: string | null;
  custom_attributes?: Record<string, unknown>;
  add_tags?: string[];
  remove_tags?: string[];
}

export const customerKeys = {
  context: (workspace: string, conversation: string) =>
    ["workspace", workspace, "conversation", conversation, "context"] as const,
  contacts: (workspace: string) => ["workspace", workspace, "contacts"] as const,
  contact: (workspace: string, id: string) => ["workspace", workspace, "contacts", id] as const,
};

export function contextQuery(workspace: string, conversation: string) {
  return queryOptions({
    queryKey: customerKeys.context(workspace, conversation),
    queryFn: () =>
      request<ConversationContext>("GET", `/conversations/${encodeURIComponent(conversation)}/context`, undefined, {
        workspace,
      }),
  });
}

export function contactConversationsQuery(workspace: string, contact: string, limit: number) {
  return queryOptions({
    queryKey: [...customerKeys.contact(workspace, contact), "conversations", limit] as const,
    queryFn: () =>
      request<Conversation[]>(
        "GET",
        `/contacts/${encodeURIComponent(contact)}/conversations?limit=${limit}`,
        undefined,
        { workspace },
      ),
  });
}

export function duplicatesQuery(workspace: string, contact: string) {
  return queryOptions({
    queryKey: [...customerKeys.contact(workspace, contact), "duplicates"] as const,
    queryFn: () =>
      request<Duplicate[]>("GET", `/contacts/${encodeURIComponent(contact)}/duplicates`, undefined, { workspace }),
  });
}

export function updateContact(workspace: string, id: string, changes: ContactChanges): Promise<Contact> {
  return request<Contact>("PATCH", `/contacts/${encodeURIComponent(id)}`, changes, { workspace });
}

export interface Company extends CompanySummary {
  external_id: string | null;
  custom_attributes: Attributes;
  notes: string | null;
  tags: Tag[];
  created_at: string;
}

export interface NewContact {
  name?: string;
  email?: string;
  phone?: string;
}

/*
 * Since is a window back from now: within the last day, week or month,
 * or longer ago than a month.
 */
export type Since = "day" | "week" | "month" | "older";

/*
 * PeopleFilters narrows the people list: q searches names, addresses and
 * phone numbers; seen and created are windows back from the moment the
 * page is fetched; attribute matches one custom attribute; the rest map
 * to the list's filters.
 */
export interface PeopleFilters {
  q?: string;
  kind?: "user" | "lead";
  tag?: string;
  company?: string;
  blocked?: boolean;
  seen?: Since;
  created?: Since;
  attribute?: { key: string; value: string };
  sort?: "last_seen_at" | "-created_at" | "name";
}

const sinceDays: Record<Since, number> = { day: 1, week: 7, month: 30, older: 30 };

/*
 * windowFilter turns a window into the after or before bound of the
 * named time filter.
 */
function windowFilter(name: string, since: Since | undefined, now: number): [string, string | undefined] {
  if (!since) {
    return [`filter[${name}_after]`, undefined];
  }

  const at = new Date(now - sinceDays[since] * 86_400_000).toISOString();

  return [`filter[${name}_${since === "older" ? "before" : "after"}]`, at];
}

/*
 * queryString writes the parameters that have a value.
 */
function queryString(entries: [string, string | undefined][]): string {
  const params = new URLSearchParams();
  for (const [key, value] of entries) {
    if (value) {
      params.set(key, value);
    }
  }

  return params.toString();
}

function peoplePath(filters: PeopleFilters, cursor: string | null): string {
  const entries: [string, string | undefined][] = [
    ["q", filters.q],
    ["filter[kind]", filters.kind],
    ["filter[tag]", filters.tag],
    ["filter[company]", filters.company],
    ["filter[blocked]", filters.blocked ? "true" : undefined],
    windowFilter("last_seen", filters.seen, Date.now()),
    windowFilter("created", filters.created, Date.now()),
    [`filter[custom.${filters.attribute?.key ?? ""}]`, filters.attribute?.value],
    ["sort", filters.sort],
    ["cursor", cursor ?? undefined],
  ];
  const query = queryString(entries);

  return `/contacts${query ? `?${query}` : ""}`;
}

export function peopleQuery(workspace: string, filters: PeopleFilters) {
  return infiniteQueryOptions({
    queryKey: [...customerKeys.contacts(workspace), "list", filters] as const,
    queryFn: ({ pageParam }) => requestPage<Contact>(peoplePath(filters, pageParam), { workspace }),
    initialPageParam: null as string | null,
    getNextPageParam: (page) => page.next,
  });
}

export function contactQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: customerKeys.contact(workspace, id),
    queryFn: () => request<Contact>("GET", `/contacts/${encodeURIComponent(id)}`, undefined, { workspace }),
  });
}

export function identitiesQuery(workspace: string, contact: string) {
  return queryOptions({
    queryKey: [...customerKeys.contact(workspace, contact), "identities"] as const,
    queryFn: () =>
      request<Identity[]>("GET", `/contacts/${encodeURIComponent(contact)}/identities`, undefined, { workspace }),
  });
}

export function detachIdentity(workspace: string, contact: string, identity: string): Promise<unknown> {
  return request<unknown>(
    "DELETE",
    `/contacts/${encodeURIComponent(contact)}/identities/${encodeURIComponent(identity)}`,
    undefined,
    { workspace },
  );
}

export function createContact(workspace: string, fields: NewContact): Promise<Contact> {
  return request<Contact>("POST", "/contacts", fields, { workspace });
}

export function companiesQuery(workspace: string, q: string) {
  return infiniteQueryOptions({
    queryKey: ["workspace", workspace, "companies", "list", q] as const,
    queryFn: ({ pageParam }) => {
      const query = queryString([
        ["q", q || undefined],
        ["cursor", pageParam ?? undefined],
      ]);

      return requestPage<Company>(`/companies${query ? `?${query}` : ""}`, { workspace });
    },
    initialPageParam: null as string | null,
    getNextPageParam: (page) => page.next,
  });
}

export function companyQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "companies", id] as const,
    queryFn: () => request<Company>("GET", `/companies/${encodeURIComponent(id)}`, undefined, { workspace }),
  });
}

export function companyPeopleQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "companies", id, "contacts"] as const,
    queryFn: () => request<Contact[]>("GET", `/companies/${encodeURIComponent(id)}/contacts`, undefined, { workspace }),
  });
}

export function companyConversationsQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "companies", id, "conversations"] as const,
    queryFn: () =>
      request<Conversation[]>("GET", `/companies/${encodeURIComponent(id)}/conversations`, undefined, { workspace }),
  });
}

export function updateCompany(
  workspace: string,
  id: string,
  changes: {
    name?: string;
    domain?: string | null;
    notes?: string | null;
    add_tags?: string[];
    remove_tags?: string[];
  },
): Promise<Company> {
  return request<Company>("PATCH", `/companies/${encodeURIComponent(id)}`, changes, { workspace });
}

export function createCompany(workspace: string, fields: { name: string; domain?: string }): Promise<Company> {
  return request<Company>("POST", "/companies", fields, { workspace });
}
