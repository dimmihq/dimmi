/*
 * The workspace's tags: the one vocabulary conversations, contacts and
 * companies share. Anyone who works conversations can add a tag; renaming
 * and deleting take the tags:manage permission, which the server checks.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export interface Tag {
  id: string;
  object: string;
  name: string;
  color: string | null;
  icon: string | null;
}

export interface TagCounts {
  conversations: number;
  contacts: number;
  companies: number;
}

export interface ListedTag extends Tag {
  counts?: TagCounts;
}

export function tagsQuery(workspace: string, counts = false) {
  return queryOptions({
    queryKey: ["workspace", workspace, "tags", counts],
    queryFn: () => request<ListedTag[]>("GET", `/tags${counts ? "?include=counts" : ""}`, undefined, { workspace }),
  });
}

export function createTag(
  workspace: string,
  name: string,
  appearance: { color?: string; icon?: string } = {},
): Promise<Tag> {
  return request<Tag>("POST", "/tags", { name, ...appearance }, { workspace });
}

export function updateTag(
  workspace: string,
  id: string,
  changes: { name?: string; color?: string | null; icon?: string | null },
): Promise<Tag> {
  return request<Tag>("PATCH", `/tags/${encodeURIComponent(id)}`, changes, { workspace });
}

export function deleteTag(workspace: string, id: string): Promise<unknown> {
  return request<unknown>("DELETE", `/tags/${encodeURIComponent(id)}`, undefined, { workspace });
}
