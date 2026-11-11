/*
 * Knowledge sources: where dimmi reads knowledge from besides the help
 * center. Each kind of source is a driver the server describes, with the
 * JSON Schema of its settings, so a new kind needs no new screen. A
 * source syncs a read-only copy or imports articles once; its runs and
 * documents explain what happened. Secrets are never sent back: the
 * server only says which are set.
 */
import { queryOptions } from "@tanstack/react-query";

import { type Page, request, requestPage } from "./client";

export type SourceMode = "sync" | "import";
export type SourceTarget = "agent" | "operator" | "help_center";
export type SourceSchedule = "manual" | "hourly" | "daily" | "weekly";

/*
 * SchemaProperty is one setting of a driver, as its JSON Schema says:
 * x-secret marks a key never shown again, x-placeholder an example,
 * x-enum-labels the words for the enum's values, and x-depends the
 * settings an x-choice setting's values are read with.
 */
export interface SchemaProperty {
  type: "string" | "integer" | "boolean" | "array";
  title?: string;
  description?: string;
  format?: string;
  default?: unknown;
  enum?: string[];
  minimum?: number;
  maximum?: number;
  items?: { type: string };
  "x-secret"?: boolean;
  "x-placeholder"?: string;
  "x-enum-labels"?: string[];
  "x-account"?: string;
  "x-depends"?: string[];
  "x-patterns"?: boolean;
}

/*
 * SettingOption is one value a setting picked from the service can take.
 */
export interface SettingOption {
  value: string;
  label: string;
  detail?: string;
  default?: boolean;
}

export interface SourceSchema {
  type: "object";
  required?: string[];
  properties: Record<string, SchemaProperty>;
}

export interface SourceType {
  type: string;
  name: string;
  summary: string;
  logo: string;
  auth: "none" | "oauth";
  schema: SourceSchema;
  modes: SourceMode[];
  targets: SourceTarget[];
  schedules: SourceSchedule[];
  integration?: string;
}

/*
 * TreeNode is one item a source can choose from: a collection or a
 * page with items under it, or a document.
 */
export interface TreeNode {
  id: string;
  title: string;
  kind: string;
  children: TreeNode[];
}

export type RunStatus = "queued" | "running" | "succeeded" | "partial" | "failed" | "canceled";

export interface SyncRun {
  id: string;
  source_id: string;
  trigger: string;
  status: RunStatus;
  full_run: boolean;
  stats: Record<string, number>;
  error_code: string | null;
  error: string | null;
  started_at: string | null;
  finished_at: string | null;
  created_at: string;
}

export interface KnowledgeSource {
  id: string;
  type: string;
  name: string;
  mode: SourceMode;
  targets: SourceTarget[];
  help_center_id: string | null;
  settings: Record<string, unknown>;
  secrets_set: string[];
  include: string[];
  exclude: string[];
  status: "idle" | "queued" | "syncing" | "error" | "paused";
  schedule: SourceSchedule;
  next_sync_at: string | null;
  last_synced_at: string | null;
  last_success_at: string | null;
  last_error_code: string | null;
  last_error: string | null;
  document_count: number;
  counts: Record<string, number>;
  last_run: SyncRun | null;
  created_at: string;
}

export type DocumentStatus = "pending" | "active" | "excluded" | "error" | "removed";

export interface KnowledgeDocument {
  id: string;
  external_id: string;
  url: string | null;
  path: string;
  title: string;
  locale: string;
  status: DocumentStatus;
  excluded_by: string | null;
  error_code: string | null;
  error: string | null;
  chunk_count: number;
  linked_article_id: string | null;
  fetched_at: string | null;
  updated_at: string;
}

export interface SourcePreview {
  account: string;
  estimate: number;
  listed: number;
  matched: number;
  more: boolean;
  sample: { title: string; path: string; url: string }[];
}

/*
 * SourceInput is a source to add or a change to one. A secret left out
 * of settings on a change keeps the stored one.
 */
export interface SourceInput {
  type?: string;
  name?: string;
  mode?: SourceMode;
  targets?: SourceTarget[];
  help_center_id?: string | null;
  settings?: Record<string, unknown>;
  include?: string[];
  exclude?: string[];
  schedule?: SourceSchedule;
  sync_now?: boolean;
}

const sources = "/knowledge/sources";
const one = (id: string) => `${sources}/${encodeURIComponent(id)}`;

export function sourceTypesQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "knowledge", "types"],
    queryFn: () => request<SourceType[]>("GET", "/knowledge/source-types", undefined, { workspace }),
    staleTime: Number.POSITIVE_INFINITY,
  });
}

/*
 * sourcesQuery lists the sources, polling while any of them syncs so the
 * list shows a run end without a reload.
 */
export function sourcesQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "knowledge", "sources"],
    queryFn: () => request<KnowledgeSource[]>("GET", sources, undefined, { workspace }),
    refetchInterval: (query) => (query.state.data?.some((s) => s.status === "syncing") ? 2000 : false),
  });
}

/*
 * KnowledgeUsage is the month's knowledge usage: what each source cost
 * to embed and to crawl, in millionths of a dollar.
 */
export interface KnowledgeUsageLine {
  source_id: string | null;
  source_name: string;
  source_type: string;
  kind: "embedding" | "crawl";
  input_tokens: number;
  estimated_tokens: number;
  browser_ms: number;
  requests: number;
  cost_micros: number;
  unpriced: boolean;
}

export interface KnowledgeUsage {
  period_start: string;
  cost_micros: number;
  unpriced: boolean;
  lines: KnowledgeUsageLine[];
}

export function knowledgeUsageQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "knowledge", "usage"],
    queryFn: () => request<KnowledgeUsage>("GET", "/knowledge/usage", undefined, { workspace }),
  });
}

export function sourceQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "knowledge", "sources", id],
    queryFn: () => request<KnowledgeSource>("GET", one(id), undefined, { workspace }),
    refetchInterval: (query) => (query.state.data?.status === "syncing" ? 2000 : false),
  });
}

export function runsQuery(workspace: string, id: string, syncing: boolean) {
  return queryOptions({
    queryKey: ["workspace", workspace, "knowledge", "sources", id, "runs"],
    queryFn: () => request<SyncRun[]>("GET", `${one(id)}/syncs`, undefined, { workspace }),
    refetchInterval: syncing ? 2000 : false,
  });
}

export function documentsQuery(
  workspace: string,
  id: string,
  status: DocumentStatus | "",
  search: string,
  syncing = false,
) {
  const params = new URLSearchParams({ limit: "100" });
  if (status) {
    params.set("status", status);
  }
  if (search) {
    params.set("q", search);
  }

  return queryOptions({
    queryKey: ["workspace", workspace, "knowledge", "sources", id, "documents", status, search],
    queryFn: (): Promise<Page<KnowledgeDocument>> =>
      requestPage<KnowledgeDocument>(`${one(id)}/documents?${params}`, { workspace }),
    refetchInterval: syncing ? 3000 : false,
  });
}

export function previewSource(workspace: string, input: SourceInput): Promise<SourcePreview> {
  return request<SourcePreview>("POST", `${sources}/previews`, input, { workspace });
}

export function createSource(workspace: string, input: SourceInput): Promise<KnowledgeSource> {
  return request<KnowledgeSource>("POST", sources, input, { workspace });
}

export function updateSource(workspace: string, id: string, input: SourceInput): Promise<KnowledgeSource> {
  return request<KnowledgeSource>("PATCH", one(id), input, { workspace });
}

export function deleteSource(workspace: string, id: string): Promise<void> {
  return request<void>("DELETE", one(id), undefined, { workspace });
}

export function syncSource(workspace: string, id: string): Promise<SyncRun> {
  return request<SyncRun>("POST", `${one(id)}/syncs`, undefined, { workspace });
}

export function cancelRun(workspace: string, id: string, run: string): Promise<SyncRun> {
  return request<SyncRun>("DELETE", `${one(id)}/syncs/${encodeURIComponent(run)}`, undefined, { workspace });
}

export function resyncDocument(workspace: string, id: string, doc: string): Promise<SyncRun> {
  return request<SyncRun>("POST", `${one(id)}/documents/${encodeURIComponent(doc)}/syncs`, undefined, { workspace });
}

export function excludeDocument(workspace: string, id: string, doc: string): Promise<void> {
  return request<void>("POST", `${one(id)}/documents/${encodeURIComponent(doc)}/exclusion`, undefined, {
    workspace,
  });
}

export function includeDocument(workspace: string, id: string, doc: string): Promise<SyncRun> {
  return request<SyncRun>("DELETE", `${one(id)}/documents/${encodeURIComponent(doc)}/exclusion`, undefined, {
    workspace,
  });
}

export function convertSource(workspace: string, id: string, helpCenter: string): Promise<void> {
  return request<void>("POST", `${one(id)}/imports`, { help_center_id: helpCenter }, { workspace });
}

export function settingOptions(
  workspace: string,
  input: { type: string; settings: Record<string, unknown>; field: string },
): Promise<SettingOption[]> {
  return request<{ options: SettingOption[] }>("POST", `${sources}/options`, input, { workspace }).then(
    (o) => o.options,
  );
}

export function previewTree(workspace: string, input: SourceInput): Promise<TreeNode[]> {
  return request<{ nodes: TreeNode[] }>("POST", `${sources}/trees`, input, { workspace }).then((t) => t.nodes);
}

export function sourceTreeQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "knowledge", "sources", id, "tree"],
    queryFn: () =>
      request<{ nodes: TreeNode[] }>("GET", `${one(id)}/tree`, undefined, { workspace }).then((t) => t.nodes),
  });
}

export function uploadArchive(workspace: string, file: File): Promise<{ key: string; name: string }> {
  const form = new FormData();
  form.append("file", file);

  return request<{ key: string; name: string }>("POST", "/knowledge/uploads", form, { workspace });
}
