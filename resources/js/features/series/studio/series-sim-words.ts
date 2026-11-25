/*
 * How a series' dry simulation reads in words: what an extract or run
 * action step found or saved, the steps whose answers a teammate can set
 * in place of the AI or the action, and the walk as a journey, moment by
 * moment: what the person got, what they did, and what the series did.
 */
import { foundNote } from "@/features/automation/studio/workflow-sim-words";
import type { NodeKind, SeriesSpec } from "@/lib/api/series";
import type { SeriesSimulation } from "@/lib/api/workflow-simulations";

import { endReasonName, nodeLook } from "../looks";
import { summary } from "../model";

type Step = SeriesSimulation["steps"][number];

/*
 * Moment is one beat of the journey: a message the person got, a thing
 * they did or did not do, or something the series did, with the step
 * kind that drew it.
 */
export type Moment =
  | { kind: "message"; key: string; node: NodeKind; to?: string; subject?: string; body: string }
  | { kind: "did"; key: string; text: string; did: boolean }
  | { kind: "event"; key: string; node: NodeKind; text: string; note?: string; muted?: boolean };

const pathWords: Record<string, string> = {
  else: "everyone else",
  matched: "they did it",
  timed_out: "the wait ran out",
  failed: "it could not send",
  fallback: "the fallback",
};

const pastTense: [string, string][] = [
  ["open ", "opened "],
  ["click ", "clicked "],
  ["reply ", "replied "],
  ["do ", "did "],
];

/*
 * didWords says what the person did, or did not do, as the series asked
 * it: "open the message "Welcome"" reads "Ada opened the message
 * "Welcome"" or "Ada did not open the message "Welcome"".
 */
export function didWords(name: string, what: string, did: boolean): string {
  if (!did) {
    return `${name} did not ${what}`;
  }
  const verb = pastTense.find(([plain]) => what.startsWith(plain));

  return `${name} ${verb ? verb[1] + what.slice(verb[0].length) : what}`;
}

/* Branch steps read by their kind and the path's name, not their whole settings. */
const branching = new Set<string>(["condition", "split", "ai_path"]);

/* labelOf finds what a step's settings call one of its paths or variants. */
function labelOf(settings: Record<string, unknown> | undefined, id: string): string | undefined {
  const named = [...((settings?.paths as unknown[]) ?? []), ...((settings?.variants as unknown[]) ?? [])] as {
    id?: string;
    label?: string;
  }[];

  return named.find((p) => p.id === id)?.label || undefined;
}

/* pathOf names the path a step sent the person down, empty for the next step. */
function pathOf(step: Step, settings?: Record<string, unknown>): string {
  if (!step.path || step.path === "next") {
    return "";
  }
  if (step.kind === "run_action" && step.path === "failed") {
    return "the action failed";
  }

  return labelOf(settings, step.path) ?? pathWords[step.path] ?? String(step.detail?.label ?? step.path);
}

/* stepMoment is what one step drew, or nothing when the step needs no line. */
function stepMoment(step: Step, index: number, result: SeriesSimulation, spec?: SeriesSpec): Moment | null {
  const key = `${step.node_id}-${String(index)}`;
  const node = spec?.nodes.find((n) => n.id === step.node_id);
  const kind = step.kind as NodeKind;
  if (kind === "entry") {
    return null;
  }
  const message = result.messages.find((m) => m.node_id === step.node_id);
  if (message && !step.skipped) {
    return { kind: "message", key, node: kind, to: message.to, subject: message.subject, body: message.body };
  }
  const said =
    branching.has(kind) || !node || !spec
      ? (nodeLook[kind]?.name ?? step.kind.replaceAll("_", " "))
      : summary(node, spec);
  const path = pathOf(step, node?.settings as Record<string, unknown> | undefined);
  const note = walkedNote(step) || undefined;
  if (kind === "wait") {
    return { kind: "event", key, node: kind, text: `${said}, skipped`, muted: true };
  }

  return { kind: "event", key, node: kind, text: path ? `${said}: ${path}` : said, note, muted: step.skipped };
}

/*
 * journey reads a dry simulation as moments, in the order they happened:
 * before each step, what the person did when the series asked, then the
 * step itself, and last how the journey ended.
 */
export function journey(result: SeriesSimulation, name: string, spec?: SeriesSpec): Moment[] {
  const out: Moment[] = [];
  const decided = result.decisions ?? [];
  const asked = (at: number) =>
    decided.forEach((d, i) => {
      if (d.before === at) {
        out.push({ kind: "did", key: `did-${String(i)}`, text: didWords(name, d.what, d.did), did: d.did });
      }
    });
  result.steps.forEach((step, index) => {
    asked(index);
    const moment = stepMoment(step, index, result, spec);
    if (moment) {
      out.push(moment);
    }
  });
  asked(result.steps.length);
  const end = endReasonName[result.end] ?? result.end.replaceAll("_", " ");
  out.push({
    kind: "event",
    key: "end",
    node: "exit",
    text: result.goal_reached && result.end !== "goal" ? `${end}, and reached the goal` : end,
  });

  return out;
}

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
