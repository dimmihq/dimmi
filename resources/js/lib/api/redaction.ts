/*
 * The workspace's redaction: the built-in rules for card and social
 * security numbers, and its own patterns in the order they apply. Every
 * change takes settings:write and redaction:manage, which the server
 * checks. Trying a text stores nothing.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export interface RedactionRule {
  id: string;
  object: "redaction_rule";
  name: string;
  pattern: string;
  replacement: string;
  enabled: boolean;
  position: number;
}

export interface BuiltIns {
  card_numbers: boolean;
  us_ssn: boolean;
}

export interface Redaction {
  object: "redaction";
  built_ins: BuiltIns;
  rules: RedactionRule[];
}

export interface RedactionHit {
  rule: string;
  count: number;
}

export interface RedactionTest {
  text: string;
  redactions: RedactionHit[];
}

export interface RuleFields {
  name?: string;
  pattern?: string;
  replacement?: string;
  enabled?: boolean;
}

export const redactionKey = (workspace: string) => ["workspace", workspace, "redaction"] as const;

export function redactionQuery(workspace: string) {
  return queryOptions({
    queryKey: redactionKey(workspace),
    queryFn: () => request<Redaction>("GET", "/redaction-rules", undefined, { workspace }),
  });
}

export function updateRedaction(
  workspace: string,
  changes: Partial<BuiltIns> & { order?: string[] },
): Promise<Redaction> {
  return request<Redaction>("PATCH", "/redaction-rules", changes, { workspace });
}

export function createRedactionRule(workspace: string, fields: RuleFields): Promise<RedactionRule> {
  return request<RedactionRule>("POST", "/redaction-rules", fields, { workspace });
}

export function updateRedactionRule(workspace: string, id: string, fields: RuleFields): Promise<RedactionRule> {
  return request<RedactionRule>("PATCH", `/redaction-rules/${encodeURIComponent(id)}`, fields, { workspace });
}

export function deleteRedactionRule(workspace: string, id: string): Promise<unknown> {
  return request<unknown>("DELETE", `/redaction-rules/${encodeURIComponent(id)}`, undefined, { workspace });
}

export function tryRedaction(workspace: string, text: string): Promise<RedactionTest> {
  return request<RedactionTest>("POST", "/redaction-tests", { text }, { workspace });
}
