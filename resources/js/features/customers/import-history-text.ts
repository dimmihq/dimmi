/*
 * The words of past imports: where each stands, what it came to, whether
 * it can still be undone, and exactly what undoing it would do, so the
 * confirmation names every removal and every change before it runs.
 */
import type { ImportSummary } from "@/lib/api/imports";
import { formatNumber } from "@/lib/i18n/format";

const statusLabels: Record<ImportSummary["status"], string> = {
  draft: "Not started",
  queued: "Queued",
  running: "Importing",
  completed: "Finished",
  failed: "Stopped",
};

/*
 * statusLabel names where an import stands, its undo first.
 */
export function statusLabel(i: ImportSummary): string {
  switch (i.rollback?.status) {
    case "queued":
    case "running":
      return "Undoing";
    case "completed":
      return "Undone";
    case "failed":
      return "Undo stopped";
    default:
      return statusLabels[i.status];
  }
}

/*
 * busy reports whether the import or its undo is still running, so the
 * list keeps following it.
 */
export function busy(i: ImportSummary): boolean {
  const running = ["queued", "running"];

  return running.includes(i.status) || running.includes(i.rollback?.status ?? "");
}

/*
 * canUndo reports whether the import can still be undone: it finished,
 * its week is not over, and no undo has run or one stopped.
 */
export function canUndo(i: ImportSummary, now: Date): boolean {
  if (i.status !== "completed" && i.status !== "failed") {
    return false;
  }
  if (i.rollback !== null && i.rollback.status !== "failed") {
    return false;
  }

  return i.rollback_until !== null && new Date(i.rollback_until) > now;
}

/*
 * countsLine says what the import came to.
 */
export function countsLine(i: ImportSummary): string {
  const parts = [
    `${formatNumber(i.created_count)} new`,
    `${formatNumber(i.updated_count)} updated`,
    i.skipped_count > 0 ? `${formatNumber(i.skipped_count)} skipped` : "",
    i.failed_count > 0 ? `${formatNumber(i.failed_count)} with problems` : "",
  ];

  return parts.filter(Boolean).join(" · ");
}

/*
 * undoneLine says what an undo came to.
 */
export function undoneLine(i: ImportSummary): string {
  const r = i.rollback;
  if (r === null) {
    return "";
  }

  return `${formatNumber(r.removed)} removed, ${formatNumber(r.reverted)} put back, ${formatNumber(r.kept)} kept`;
}

/*
 * undoPlan lists exactly what undoing the import does, one line each.
 */
export function undoPlan(i: ImportSummary, noun: string): string[] {
  const lines: string[] = [];
  if (i.created_count > 0) {
    lines.push(
      `Removes the ${formatNumber(i.created_count)} ${noun} it added. Any of them edited, tagged, ` +
        `${i.kind === "contacts" ? "or talked to" : "or given people"} since the import are kept.`,
    );
  }
  if (i.updated_count > 0) {
    lines.push(
      `Puts back the previous values on the ${formatNumber(i.updated_count)} ${noun} it changed. ` +
        "A field edited again since the import keeps its new value.",
    );
  }
  lines.push("Takes off the tags it added. Tags themselves stay in the workspace.");
  lines.push("Runs in the background. Everything kept is listed with why once it finishes.");

  return lines;
}

/*
 * keptMore says how many kept records the list does not name.
 */
export function keptMore(i: ImportSummary, shown: number): string {
  const r = i.rollback;
  if (r === null || r.kept <= shown) {
    return "";
  }

  return `and ${formatNumber(r.kept - Math.min(shown, r.kept_records.length))} more`;
}

/*
 * byLine says who ran the import and when.
 */
export function byLine(i: ImportSummary, when: string): string {
  return `${i.created_by_name ?? "A former teammate"} · ${when}`;
}
