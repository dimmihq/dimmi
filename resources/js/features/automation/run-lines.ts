/*
 * How a workflow run reads in the runs view: the name of each status, the
 * colour of its chip, and a plain line for what a step found, from the
 * detail the server keeps for it. Kinds whose detail says nothing worth
 * reading get no line.
 */
import type { ChipColor } from "@/components/base/badges/chip";
import type { RunStatus, RunStep, RunStepStatus } from "@/lib/api/workflows";

export const runStatusName: Record<RunStatus, string> = {
  running: "Running",
  waiting: "Waiting",
  completed: "Completed",
  passed: "Passed over",
  exited: "Exited",
  canceled: "Canceled",
  failed: "Failed",
  expired: "Expired",
};

export const runStatusTint: Record<RunStatus, ChipColor> = {
  running: "blue",
  waiting: "yellow",
  completed: "lime",
  passed: "neutral",
  exited: "neutral",
  canceled: "neutral",
  failed: "rose",
  expired: "neutral",
};

export const stepStatusName: Record<RunStepStatus, string> = {
  completed: "Done",
  waiting: "Waiting",
  skipped: "Skipped",
  failed: "Failed",
  interrupted: "Cut short",
};

export const stepStatusTint: Record<RunStepStatus, ChipColor> = {
  completed: "lime",
  waiting: "yellow",
  skipped: "neutral",
  failed: "rose",
  interrupted: "neutral",
};

function word(detail: Record<string, unknown>, name: string): string {
  const value = detail[name];

  return typeof value === "string" ? value : "";
}

function called(detail: Record<string, unknown>): string {
  const parts: string[] = [];
  if (typeof detail.status === "number") {
    parts.push(`Answered ${detail.status}`);
  }
  const error = word(detail, "error");
  if (error) {
    parts.push(error);
  }
  const saved = Array.isArray(detail.saved) ? detail.saved.filter((item) => typeof item === "string") : [];
  if (saved.length > 0) {
    parts.push(`saved ${saved.join(", ")}`);
  }

  return parts.join(" · ");
}

function sorted(detail: Record<string, unknown>): string {
  const category = word(detail, "category");
  const confidence = word(detail, "confidence");
  const reason = word(detail, "reason");
  const into = category ? `Sorted into ${category}${confidence ? ` (${confidence} confidence)` : ""}` : "";

  return [into, reason].filter(Boolean).join(" · ");
}

function ran(detail: Record<string, unknown>, workflows: ReadonlyMap<string, string>): string {
  const id = word(detail, "workflow_id");
  const name = workflows.get(id) ?? "another workflow";
  const status = word(detail, "child_status") as RunStatus;
  const reason = word(detail, "reason");
  const ended = status ? ` · ${runStatusName[status]?.toLowerCase() ?? status}` : "";

  return [id ? `Ran ${name}${ended}` : "", reason].filter(Boolean).join(" · ");
}

function quoted(detail: Record<string, unknown>): string {
  const time = word(detail, "reply_time");
  const hours = typeof detail.open === "boolean" ? (detail.open ? "during office hours" : "outside office hours") : "";

  return [time ? `Said “${time}”` : "", hours].filter(Boolean).join(", ");
}

/*
 * detailLine is what a step found, as a line people read, or empty.
 */
export function detailLine(step: RunStep, workflows: ReadonlyMap<string, string> = new Map()): string {
  const detail = step.detail ?? {};
  switch (step.kind) {
    case "http_request":
      return called(detail);
    case "classify":
      return sorted(detail);
    case "run_workflow":
      return ran(detail, workflows);
    case "reply_time":
      return quoted(detail);
    default:
      return "";
  }
}
