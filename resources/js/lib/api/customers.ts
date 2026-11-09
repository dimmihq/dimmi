/*
 * The customers a team talks to: contacts, the companies they work at,
 * and the context the panel beside a conversation shows. Contact and
 * company queries are cached under the workspace, so a change made here
 * refreshes every screen that shows the same record.
 */
import { infiniteQueryOptions, keepPreviousData, queryOptions } from "@tanstack/react-query";

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
  people_count?: number;
  created_at: string;
}

export interface NewContact {
  name?: string;
  email?: string;
  phone?: string;
}

/*
 * PeopleFilters narrows the people list: q searches names, addresses and
 * phone numbers; conditions are the filter chips, each a field, how it
 * compares and its value or values, checked by the server.
 */
export interface PeopleFilters {
  q?: string;
  blocked?: boolean;
  conditions?: Condition[];
  sort?: "last_seen_at" | "-created_at" | "name";
}

/*
 * Condition is one filter as the server reads it. Type is a custom
 * attribute's type; days counts back from now for the time fields.
 */
export interface Condition {
  field: string;
  op: string;
  value?: string;
  values?: string[];
  type?: string;
  days?: number;
}

/*
 * queryString writes the parameters that have a value.
 */
export function queryString(entries: [string, string | undefined][]): string {
  const params = new URLSearchParams();
  for (const [key, value] of entries) {
    if (value) {
      params.set(key, value);
    }
  }

  return params.toString();
}

/*
 * peopleFilters writes the directory's filters as the list reads them,
 * with no value for those not set.
 */
function peopleFilters(filters: PeopleFilters): [string, string | undefined][] {
  return [
    ["filter[blocked]", filters.blocked ? "true" : undefined],
    ["conditions", filters.conditions?.length ? JSON.stringify(filters.conditions) : undefined],
  ];
}

function peoplePath(filters: PeopleFilters, cursor: string | null): string {
  const entries: [string, string | undefined][] = [
    ["q", filters.q],
    ...peopleFilters(filters),
    ["sort", filters.sort],
    ["cursor", cursor ?? undefined],
  ];
  const query = queryString(entries);

  return `/contacts${query ? `?${query}` : ""}`;
}

/*
 * peoplePage is one numbered page of the directory, with how many people
 * match in all, counted up to peopleCap.
 */
export function peoplePageQuery(workspace: string, filters: PeopleFilters, page: number) {
  const entries: [string, string | undefined][] = [
    ["q", filters.q],
    ...peopleFilters(filters),
    ["sort", filters.sort],
    ["limit", String(peoplePerPage)],
    ["page", String(page)],
    ["total", "true"],
  ];

  return queryOptions({
    queryKey: [...customerKeys.contacts(workspace), "page", filters, page] as const,
    queryFn: () => requestPage<Contact>(`/contacts?${queryString(entries)}`, { workspace }),
    placeholderData: keepPreviousData,
  });
}

/*
 * peoplePerPage is how many people a directory page shows; peopleCap is
 * the most the server counts.
 */
export const peoplePerPage = 25;
export const peopleCap = 10_000;

/*
 * peopleSelection is what an export of the directory's current list
 * covers: its filters and search.
 */
export function peopleSelection(filters: PeopleFilters): {
  filter: Record<string, string>;
  q?: string;
  conditions?: string;
} {
  return {
    filter: filters.blocked ? { blocked: "true" } : {},
    ...(filters.q ? { q: filters.q } : {}),
    ...(filters.conditions?.length ? { conditions: JSON.stringify(filters.conditions) } : {}),
  };
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

/*
 * companiesPageQuery is one numbered page of the companies, with how many
 * match in all, counted up to peopleCap.
 */
export function companiesPageQuery(workspace: string, q: string, page: number) {
  const query = queryString([
    ["q", q || undefined],
    ["limit", String(peoplePerPage)],
    ["page", String(page)],
    ["total", "true"],
  ]);

  return queryOptions({
    queryKey: ["workspace", workspace, "companies", "page", q, page] as const,
    queryFn: () => requestPage<Company>(`/companies?${query}`, { workspace }),
    placeholderData: keepPreviousData,
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

export interface ContactErasure {
  id: string;
  object: "contact_erasure";
  contact_id: string;
  status: "pending" | "pending_hold" | "completed" | "canceled";
  scheduled_for: string;
  completed_at: string | null;
  counts: Record<string, number>;
  identities_conflicted?: string[];
  created_at: string;
}

export const erasureKeys = {
  all: (workspace: string) => ["workspace", workspace, "contact-erasures"] as const,
};

/*
 * erasuresPageQuery is one numbered page of the erasure requests, newest
 * first, narrowed to one status when given, with how many match in all.
 */
export function erasuresPageQuery(workspace: string, status: ContactErasure["status"] | undefined, page: number) {
  const query = queryString([
    ["filter[status]", status],
    ["limit", String(peoplePerPage)],
    ["page", String(page)],
    ["total", "true"],
  ]);

  return queryOptions({
    queryKey: [...erasureKeys.all(workspace), "page", status ?? "", page] as const,
    queryFn: () => requestPage<ContactErasure>(`/contact-erasures?${query}`, { workspace }),
    placeholderData: keepPreviousData,
  });
}

export function eraseContact(workspace: string, contact: string, immediate: boolean): Promise<ContactErasure> {
  const query = immediate ? "erase=true&immediate=true" : "erase=true";

  return request<ContactErasure>("DELETE", `/contacts/${encodeURIComponent(contact)}?${query}`, undefined, {
    workspace,
  });
}

export function cancelErasure(workspace: string, id: string): Promise<ContactErasure> {
  return request<ContactErasure>(
    "PATCH",
    `/contact-erasures/${encodeURIComponent(id)}`,
    { status: "canceled" },
    {
      workspace,
    },
  );
}

export interface ContactExport {
  id: string;
  object: "contact_export";
  contact_id: string;
  status: "pending" | "running" | "ready" | "failed" | "expired";
  include_notes: boolean;
  size_bytes: number | null;
  expires_at: string | null;
  url: string | null;
  created_at: string;
}

export function exportQuery(workspace: string, contact: string, id: string) {
  return queryOptions({
    queryKey: [...customerKeys.contact(workspace, contact), "exports", id] as const,
    queryFn: () =>
      request<ContactExport>(
        "GET",
        `/contacts/${encodeURIComponent(contact)}/exports/${encodeURIComponent(id)}`,
        undefined,
        { workspace },
      ),
  });
}

export function requestExport(workspace: string, contact: string): Promise<ContactExport> {
  return request<ContactExport>("POST", `/contacts/${encodeURIComponent(contact)}/exports`, {}, { workspace });
}

/*
 * downloadExport fetches a ready export's zip, which the workspace header
 * lets the server find.
 */
export async function downloadExport(workspace: string, url: string): Promise<Blob> {
  const response = await fetch(url, {
    headers: { Accept: "application/zip", "Dimmi-Workspace": workspace },
    credentials: "same-origin",
  });
  if (!response.ok) {
    throw new Error("The export could not be downloaded.");
  }

  return response.blob();
}
