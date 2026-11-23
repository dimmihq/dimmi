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
