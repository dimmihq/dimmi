/*
 * Members' exports of conversations and customers. An export covers the
 * rows picked, or a list's filters as the list itself reads them, and is
 * built in the background; only the member who asked sees it. Each
 * format has a ceiling on how many rows it covers, which the server
 * checks when the export is asked for. An owner can also export the
 * whole workspace as one zip, which downloads from a signed link.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export type ExportKind = "conversations" | "contacts";
export type ExportFormat = "txt" | "csv" | "json";

/* WORKSPACE_EXPORT is the permission a whole-workspace export needs. */
export const WORKSPACE_EXPORT = "exports:write";

export interface DataExport {
  id: string;
  object: "data_export";
  kind: ExportKind | "conversation_facts" | "workspace";
  format: ExportFormat | "pdf" | "zip";
  status: "pending" | "running" | "ready" | "failed" | "expired";
  include_notes: boolean;
  row_count: number | null;
  warnings: string[];
  file_name: string;
  size_bytes: number | null;
  error: string | null;
  expires_at: string | null;
  url: string | null;
  created_at: string;
}

/*
 * ExportSelection is what an export covers: ids picked, or a list's
 * filters with its search.
 */
export interface ExportSelection {
  ids?: string[];
  filter?: Record<string, string>;
  q?: string;
  conditions?: string;
}

export interface ExportRequest {
  kind: ExportKind;
  format: ExportFormat;
  include_notes?: boolean;
  selection: ExportSelection;
}

export const dataExportsKey = (workspace: string) => ["workspace", workspace, "data-exports"] as const;

export function dataExportsQuery(workspace: string) {
  return queryOptions({
    queryKey: dataExportsKey(workspace),
    queryFn: () => request<DataExport[]>("GET", "/data-exports", undefined, { workspace }),
  });
}

export function requestDataExport(workspace: string, body: ExportRequest): Promise<DataExport> {
  return request<DataExport>("POST", "/data-exports", body, { workspace });
}

/*
 * requestWorkspaceExport starts an export of everything the workspace
 * owns, as one zip.
 */
export function requestWorkspaceExport(workspace: string): Promise<DataExport> {
  return request<DataExport>("POST", "/data-exports", { kind: "workspace", format: "zip" }, { workspace });
}

/*
 * dataExportLink signs a link a ready export's file downloads from for
 * a few minutes, so a large file goes straight to the browser's
 * downloads instead of through memory.
 */
export async function dataExportLink(workspace: string, id: string): Promise<string> {
  const link = await request<{ url: string }>("GET", `/data-exports/${encodeURIComponent(id)}/link`, undefined, {
    workspace,
  });

  return link.url;
}

export function deleteDataExport(workspace: string, id: string): Promise<unknown> {
  return request<unknown>("DELETE", `/data-exports/${encodeURIComponent(id)}`, undefined, { workspace });
}

/*
 * downloadDataExport fetches a ready export's file, which the workspace
 * header lets the server find.
 */
export async function downloadDataExport(workspace: string, url: string): Promise<Blob> {
  const response = await fetch(url, { headers: { "Dimmi-Workspace": workspace }, credentials: "same-origin" });
  if (!response.ok) {
    throw new Error("The export could not be downloaded.");
  }

  return response.blob();
}

/*
 * formatsOf lists the formats a kind of export comes in, with what each
 * holds.
 */
export function formatsOf(kind: ExportKind): { id: ExportFormat; label: string; hint: string }[] {
  const sheet = { id: "csv" as const, label: "Spreadsheet", hint: "One row each, up to 50,000." };
  const data = { id: "json" as const, label: "Data", hint: "Everything as JSON, up to 10,000." };
  if (kind === "contacts") {
    return [sheet, data];
  }

  return [{ id: "txt", label: "Transcripts", hint: "One text file per conversation, up to 1,000." }, sheet, data];
}
