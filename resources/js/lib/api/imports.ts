/*
 * Imports of customers and companies from a CSV. An upload is kept as a
 * draft with its header, first rows and the mapping its headers
 * suggest; a dry run counts what a mapping would do; a start runs it in
 * the background, and the import is then read again for its progress.
 * The rows that failed download as a CSV. Importing takes
 * imports:manage, which the server checks.
 */
import { queryOptions } from "@tanstack/react-query";

import { request, requestFile } from "./client";

export type ImportKind = "contacts" | "companies";
export type ImportMode = "update" | "skip";
export type ImportStatus = "draft" | "queued" | "running" | "completed" | "failed";

export interface ImportColumn {
  column: number;
  field: string;
}

export interface ImportRowError {
  row: number;
  message: string;
}

export interface Import {
  id: string;
  kind: ImportKind;
  status: ImportStatus;
  file_name: string;
  row_count: number;
  columns: string[];
  preview: string[][];
  suggested_mapping: ImportColumn[];
  mapping: ImportColumn[];
  mode: ImportMode;
  processed_rows: number;
  created_count: number;
  updated_count: number;
  skipped_count: number;
  failed_count: number;
  errors: ImportRowError[];
  error_report_url: string | null;
  error: string | null;
}

export interface ImportPreview {
  rows: number;
  created: number;
  updated: number;
  skipped: number;
  failed: number;
  errors: ImportRowError[];
}

export interface ImportPlan {
  mapping: ImportColumn[];
  mode: ImportMode;
}

export function uploadImport(workspace: string, kind: ImportKind, file: File): Promise<Import> {
  const form = new FormData();
  form.append("file", file);
  form.append("kind", kind);

  return request<Import>("POST", "/imports", form, { workspace });
}

export function importQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "imports", id],
    queryFn: () => request<Import>("GET", `/imports/${id}`, undefined, { workspace }),
  });
}

export function previewImport(workspace: string, id: string, plan: ImportPlan): Promise<ImportPreview> {
  return request<ImportPreview>("POST", `/imports/${id}/previews`, plan, { workspace });
}

export function startImport(workspace: string, id: string, plan: ImportPlan): Promise<Import> {
  return request<Import>("POST", `/imports/${id}/runs`, plan, { workspace });
}

export function downloadImportErrors(workspace: string, id: string) {
  return requestFile(`/imports/${id}/errors`, "text/csv", { workspace });
}
