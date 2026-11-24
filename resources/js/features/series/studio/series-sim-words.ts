/*
 * How a series' dry simulation reads in words: what an extract or run
 * action step found or saved, and the steps whose answers a teammate
 * can set in place of the AI or the action.
 */
import { foundNote } from "@/features/automation/studio/workflow-sim-words";
import type { SeriesSimulation } from "@/lib/api/workflow-simulations";

/* shownValue is a value as a short text. */
function shownValue(value: unknown): string {
  return typeof value === "string" ? value : JSON.stringify(value);
}

/* pairs reads an object's values as "key: value". */
function pairs(values: unknown): string {
  if (!values || typeof values !== "object") {
    return "";
  }

  return Object.entries(values as Record<string, unknown>)
    .map(([key, value]) => `${key}: ${shownValue(value)}`)
    .join(", ");
}

/* walkedNote says what an extract or run action step kept for the steps after it. */
export function walkedNote(step: SeriesSimulation["steps"][number]): string {
  const detail = step.detail ?? {};
  if (step.kind === "extract") {
    return foundNote(detail);
  }
  if (step.kind !== "run_action") {
    return "";
  }
  if (step.result === "action:failed") {
    return `The action answered ${shownValue(detail.status)}`;
  }
  const saved = pairs(detail.values);

  return saved ? `Saved ${saved}` : "";
}

/* mockableNodes lists the steps whose answers a dry simulation can set. */
export function mockableNodes(nodes: { id: string; kind: string }[]): { id: string; kind: "extract" | "run_action" }[] {
  return nodes.flatMap((n) => (n.kind === "extract" || n.kind === "run_action" ? [{ id: n.id, kind: n.kind }] : []));
}
