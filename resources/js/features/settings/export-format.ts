/*
 * How the Exports page describes an export: its format in words, where
 * it stands, and what it covers.
 */
import type { DataExport } from "@/lib/api/data-exports";

const formatNames: Record<DataExport["format"], string> = {
  txt: "Transcripts",
  csv: "Spreadsheet",
  json: "Data",
  pdf: "PDF",
};

export const statusNames: Record<DataExport["status"], string> = {
  pending: "Preparing",
  running: "Preparing",
  ready: "Ready",
  failed: "Failed",
  expired: "Expired",
};

export function building(exports: DataExport[] | undefined): boolean {
  return (exports ?? []).some((e) => e.status === "pending" || e.status === "running");
}

/*
 * exportDetail says what an export covers, when it was asked for and,
 * once ready, until when it downloads.
 */
export function exportDetail(e: DataExport): string {
  const what = `${formatNames[e.format]} of ${e.kind === "contacts" ? "customers" : "conversations"}`;
  const rows = e.row_count === null ? "" : `, ${e.row_count} ${e.row_count === 1 ? "row" : "rows"}`;
  const asked = new Date(e.created_at).toLocaleString();
  const until = e.status === "ready" && e.expires_at ? ` · until ${new Date(e.expires_at).toLocaleDateString()}` : "";

  return `${what}${rows} · ${asked}${until}`;
}
