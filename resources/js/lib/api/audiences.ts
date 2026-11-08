/*
 * Saved audiences: named rules the workspace writes once and uses
 * wherever the product asks "who". A rule is a tree of all, any and not
 * groups over conditions; the server checks it, counts who it matches
 * and tests it on one person. Reading takes audiences:read; changing
 * takes audiences:manage, which the server checks.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export type Visitors = "everyone" | "identified" | "leads" | "anonymous" | "anonymous_and_leads";

/*
 * Condition is one comparison of a field with a value.
 */
export interface Condition {
  field: string;
  op: string;
  value?: unknown;
  within_days?: number;
}

/*
 * Rule is a node of a rule tree: a group of exactly one kind, or a
 * condition.
 */
export type Rule = { all: Rule[] } | { any: Rule[] } | { not: Rule } | Condition;

export interface AudienceRules {
  visitors?: Visitors;
  rules?: Rule;
}

export interface AudienceEstimate {
  count: number;
  pages: number;
  approximate: boolean;
  at: string;
}

export interface AudienceUser {
  object: "workflow" | "audience";
  id: string;
  name: string;
  live: boolean;
}

export interface Audience {
  id: string;
  name: string;
  description: string | null;
  rules: AudienceRules;
  last_estimate: AudienceEstimate | null;
  lock_version: number;
  used_by: AudienceUser[] | null;
  updated_at: string;
}

/*
 * RuleProblem is one reason the server refused a rule, with the path of
 * the condition it is about.
 */
export interface RuleProblem {
  code: string;
  path: string;
  field?: string;
  message: string;
}

export interface RuleWarning {
  code: string;
  path: string;
  message: string;
}

export interface AudiencePerson {
  id: string;
  name: string | null;
  email: string | null;
  kind: string;
}

export interface AudiencePreview {
  count: number;
  pages: number;
  total_contacts: number;
  approximate: boolean;
  sample: AudiencePerson[];
  warnings: RuleWarning[];
}

export interface LeafResult {
  path: string;
  field: string;
  op: string;
  value?: unknown;
  actual: unknown;
  trust: "verified" | "observed" | "claimed";
  result: "true" | "false" | "unknown";
}

export interface AudienceEvaluation {
  result: "match" | "no_match" | "depends_on_page";
  contact: AudiencePerson;
  leaves: LeafResult[];
}

export interface AudienceField {
  key: string;
  type: "string" | "number" | "bool" | "date" | "list" | "set" | "url" | "id" | "auto" | "saved_audience";
  where: "server" | "client";
  trust: "verified" | "observed" | "claimed";
  options?: string[];
  ops: string[];
  custom: boolean;
}

/*
 * BrowserContext is what a person's browser would report, typed in to
 * test them.
 */
export interface BrowserContext {
  page_url?: string;
  locale?: string;
  device?: "desktop" | "mobile" | "tablet";
  country?: string;
}

const path = (id: string) => `/audiences/${encodeURIComponent(id)}`;

export function audiencesQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "audiences"],
    queryFn: () => request<Audience[]>("GET", "/audiences", undefined, { workspace }),
  });
}

export function audienceQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "audiences", id],
    queryFn: () => request<Audience>("GET", path(id), undefined, { workspace }),
    retry: false,
  });
}

export function audienceFieldsQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "audience-fields"],
    queryFn: () => request<AudienceField[]>("GET", "/audience-fields", undefined, { workspace }),
    staleTime: 60_000,
  });
}

export function createAudience(
  workspace: string,
  audience: { name: string; description?: string; rules?: AudienceRules },
): Promise<Audience> {
  return request<Audience>("POST", "/audiences", audience, { workspace });
}

export function saveAudience(
  workspace: string,
  id: string,
  changes: { name?: string; description?: string; rules?: AudienceRules; lock_version: number },
): Promise<Audience> {
  return request<Audience>("PATCH", path(id), changes, { workspace });
}

export function deleteAudience(workspace: string, id: string): Promise<void> {
  return request<void>("DELETE", path(id), undefined, { workspace });
}

export function previewSavedAudience(workspace: string, id: string): Promise<AudiencePreview> {
  return request<AudiencePreview>("POST", `${path(id)}/previews`, undefined, { workspace });
}

export function previewAudience(workspace: string, rules: AudienceRules, sample = 10): Promise<AudiencePreview> {
  return request<AudiencePreview>("POST", "/audience-previews", { rules, sample }, { workspace });
}

export function evaluateAudience(
  workspace: string,
  rules: AudienceRules,
  contactID: string,
  context: BrowserContext,
): Promise<AudienceEvaluation> {
  return request<AudienceEvaluation>(
    "POST",
    "/audience-evaluations",
    { rules, contact_id: contactID, context },
    { workspace },
  );
}
