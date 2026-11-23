/*
 * How a routine's run reads in a list or a header.
 */
import type { RoutineRun } from "@/lib/api/routines";

/*
 * runLine says what a run did, in a few words.
 */
export function runLine(r: RoutineRun): string {
  switch (r.status) {
    case "queued":
      return "Waiting to start";
    case "running":
      return "Running now";
    case "skipped":
      return `Skipped: ${(r.skip_reason ?? "").replaceAll("_", " ")}`;
    case "failed":
      return `Failed: ${r.error_message ?? r.error_code ?? "unknown error"}`;
  }
  if (r.outcome === "findings") {
    return r.changesets_count === 1 ? "1 suggested change" : `${r.changesets_count} suggested changes`;
  }
  return r.noop_reason ? `Nothing to change: ${r.noop_reason}` : "Nothing to change";
}

/*
 * tookOf says how long a finished run took, empty while it has not
 * finished.
 */
export function tookOf(r: Pick<RoutineRun, "started_at" | "completed_at">): string {
  if (!r.started_at || !r.completed_at) {
    return "";
  }
  const seconds = Math.max(1, Math.round((Date.parse(r.completed_at) - Date.parse(r.started_at)) / 1000));
  if (seconds < 60) {
    return `took ${seconds} s`;
  }
  const rest = seconds % 60;

  return rest ? `took ${Math.floor(seconds / 60)} min ${rest} s` : `took ${Math.floor(seconds / 60)} min`;
}

export const triggers = { scheduled: "On schedule", manual: "Run now", dry_run: "Dry run" } as const;

function counted(n: number, one: string, many: string): string {
  return n === 0 ? "" : `${n.toLocaleString()} ${n === 1 ? one : many}`;
}

/*
 * readLine says how much a run read, empty when it opened nothing one
 * by one.
 */
export function readLine(r: Pick<RoutineRun, "read">): string {
  const parts = [
    counted(r.read.articles, "article", "articles"),
    counted(r.read.conversations, "conversation", "conversations"),
    counted(r.read.procedures, "procedure", "procedures"),
  ].filter(Boolean);
  if (parts.length === 0) {
    return "";
  }
  const last = parts.pop();

  return `read ${parts.length > 0 ? `${parts.join(", ")} and ${last}` : last}`;
}

/*
 * How a run ended, for filtering the list.
 */
export type Ended = "all" | "suggested" | "nothing" | "failed";

export const endings: { id: Ended; label: string }[] = [
  { id: "all", label: "All" },
  { id: "suggested", label: "Suggested changes" },
  { id: "nothing", label: "Nothing to change" },
  { id: "failed", label: "Failed" },
];

function endedAs(r: RoutineRun): Ended | null {
  if (r.status === "failed" || r.status === "skipped") {
    return "failed";
  }
  if (r.status !== "completed") {
    return null;
  }
  return r.outcome === "findings" ? "suggested" : "nothing";
}

/*
 * shownRuns are the runs a filter keeps. A run still going shows under
 * every filter, so starting one never looks like nothing happened.
 */
export function shownRuns(runs: RoutineRun[], ended: Ended, dry: boolean): RoutineRun[] {
  return runs.filter((r) => {
    if (!dry && r.trigger === "dry_run" && endedAs(r) !== null) {
      return false;
    }
    return ended === "all" || endedAs(r) === null || endedAs(r) === ended;
  });
}

/*
 * lastLine says what the last real run came to, and what of it still
 * waits; empty before the first one finishes.
 */
export function lastLine(runs: RoutineRun[]): string {
  const last = runs.find((r) => r.trigger !== "dry_run" && (r.status === "completed" || r.status === "failed"));
  if (!last) {
    return "";
  }
  if (last.status === "failed") {
    return "The last run failed.";
  }
  if (last.outcome !== "findings") {
    return "The last run found nothing to change.";
  }
  const made = last.changesets_count === 1 ? "1 change" : `${last.changesets_count} changes`;
  const waiting = last.suggestions_waiting > 0 ? `, ${last.suggestions_waiting} waiting for review` : ", all reviewed";

  return `The last run suggested ${made}${waiting}.`;
}
