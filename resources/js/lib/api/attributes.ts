/*
 * The custom attributes of contacts and companies: each defined with a
 * type, a label and a status. Keys sent without a definition are defined
 * from their first trusted value; values that do not fit are dropped and
 * listed as problems. Reading takes contacts:read; changing takes
 * attributes:manage, which the server checks.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export type AttributeOf = "contact" | "company";
export type AttributeType = "string" | "number" | "boolean" | "date" | "list";

export interface AttributeDefinition {
  id: string;
  of: AttributeOf;
  key: string;
  label: string;
  description: string | null;
  data_type: AttributeType;
  options: string[];
  origin: "created" | "observed";
  status: "active" | "archived";
  client_writable: boolean;
  last_value_at: string | null;
}

export interface AttributeProblem {
  of: AttributeOf;
  key: string;
  code: "type_mismatch" | "invalid_option" | "archived_write" | "key_limit_reached";
  count: number;
  sample: string | null;
  last_at: string;
}

export function attributesQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "attribute-definitions"],
    queryFn: () => request<AttributeDefinition[]>("GET", "/attribute-definitions", undefined, { workspace }),
  });
}

export function attributeProblemsQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "attribute-problems"],
    queryFn: () => request<AttributeProblem[]>("GET", "/attribute-problems", undefined, { workspace }),
  });
}

export function defineAttribute(
  workspace: string,
  definition: { object: AttributeOf; key: string; label: string; data_type: AttributeType; options?: string[] },
): Promise<AttributeDefinition> {
  return request<AttributeDefinition>("POST", "/attribute-definitions", definition, { workspace });
}

export function updateAttribute(
  workspace: string,
  id: string,
  changes: { label?: string; options?: string[]; status?: "active" | "archived"; client_writable?: boolean },
): Promise<AttributeDefinition> {
  return request<AttributeDefinition>("PATCH", `/attribute-definitions/${encodeURIComponent(id)}`, changes, {
    workspace,
  });
}

export function deleteAttribute(workspace: string, id: string): Promise<void> {
  return request<void>("DELETE", `/attribute-definitions/${encodeURIComponent(id)}`, undefined, { workspace });
}
