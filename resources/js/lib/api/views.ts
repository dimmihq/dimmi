/*
 * The saved views of the inbox: the query that lists the ones the person
 * sees and the calls that create, change, order and delete them. A view
 * describes its conversations as groups of conditions, kept as the API
 * stores them.
 */
import { queryOptions } from "@tanstack/react-query";

import { realtimeKeys } from "@/lib/realtime/keys";

import { request } from "./client";
import type { ConversationSort, StatusFilter } from "./inbox";

export interface ViewCondition {
  field: string;
  op: string;
  value?: unknown;
}

export interface ViewGroup {
  op: "and" | "or";
  conditions: ViewCondition[];
}

export interface ViewDefinition {
  op: "and" | "or";
  groups: ViewGroup[];
}

export interface SavedView {
  id: string;
  name: string;
  icon: string | null;
  scope: "shared" | "personal";
  owner_id: string | null;
  definition: ViewDefinition;
  sort: ConversationSort | null;
  status_filter: StatusFilter | null;
  position: number;
  warnings: { path: string; code: string }[];
}

export interface ViewFields {
  name?: string;
  icon?: string;
  scope?: SavedView["scope"];
  definition?: ViewDefinition;
  sort?: string;
  status_filter?: string;
}

export function viewsQuery(workspace: string) {
  return queryOptions({
    queryKey: realtimeKeys.views(workspace),
    queryFn: () => request<SavedView[]>("GET", "/conversation-views", undefined, { workspace }),
  });
}

export function createView(workspace: string, fields: ViewFields): Promise<SavedView> {
  return request<SavedView>("POST", "/conversation-views", fields, { workspace });
}

export function updateView(workspace: string, id: string, fields: ViewFields): Promise<SavedView> {
  return request<SavedView>("PATCH", `/conversation-views/${encodeURIComponent(id)}`, fields, { workspace });
}

export function deleteView(workspace: string, id: string): Promise<void> {
  return request<void>("DELETE", `/conversation-views/${encodeURIComponent(id)}`, undefined, { workspace });
}
