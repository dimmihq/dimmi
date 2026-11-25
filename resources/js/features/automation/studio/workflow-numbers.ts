/*
 * What the studio's Analytics, Failures and History tabs read off a
 * workflow, kept apart from the views so it can be tested. Steps are
 * placed in reading order with a number that says where they sit: 2 is
 * the second step, 2a.1 the first step down the first path of step 2.
 * A step reads as its kind and, for a message, the start of its text.
 * Rates are whole percents of a total, a dash when there is no total.
 * The diff lists what changed between two copies of a workflow: the
 * trigger and the rules above the steps, then each step added, removed,
 * moved or edited, with a message's text compared line by line.
 */
import { type GuidanceLine, guidanceLines } from "@/features/agents/agent-diff";
import type { Spec, SpecStep, WorkflowGoal } from "@/lib/api/workflows";

import { stepLook } from "../step-kinds";
import { type TriggerKind, triggers } from "../triggers";
import { pathName, type StepKind } from "../workflows";

export interface PlacedStep {
  step: SpecStep;
  number: string;
  /* The step and path it sits under, as "Step 2, Billing"; null at the top. */
  under: string | null;
  depth: number;
}

const letters = "abcdefghijklmnopqrstuvwxyz";

/*
 * placeSteps lists every step of a tree in reading order with its
 * number.
 */
export function placeSteps(steps: SpecStep[], prefix = "", under: string | null = null, depth = 0): PlacedStep[] {
  return steps.flatMap((step, i) => {
    const number = `${prefix}${i + 1}`;
    const placed: PlacedStep = { step, number, under, depth };
    const inner = (step.paths ?? []).flatMap((path, p) =>
      placeSteps(
        path.steps,
        `${number}${letters[p] ?? String(p)}.`,
        `Step ${number}, ${pathName(path) || "a path"}`,
        depth + 1,
      ),
    );

    return [placed, ...inner];
  });
}

/*
 * stepLabel is how a step reads in a list: its kind and, when it says
 * something, the start of what it says.
 */
export function stepLabel(step: SpecStep | undefined, kind?: string): string {
  const name = stepLook[(step?.kind ?? kind) as StepKind]?.name ?? step?.kind ?? kind ?? "A removed step";
  const text = step?.settings?.text;
  if (typeof text !== "string" || text.trim() === "") {
    return name;
  }
  const short = text.trim().split("\n")[0];

  return `${name}: “${short.length > 48 ? `${short.slice(0, 47)}…` : short}”`;
}

/*
 * rate is a part of a total as a whole percent, a dash for no total.
 */
export function rate(part: number, total: number): string {
  if (total <= 0) {
    return "–";
  }

  return `${Math.round((part / total) * 100)}%`;
}

/*
 * lift is how many points the treated group's rate beats the control
 * group's by, signed, a dash when either group is empty.
 */
export function lift(treated: { entered: number; goal: number }, control: { entered: number; goal: number }): string {
  if (treated.entered <= 0 || control.entered <= 0) {
    return "–";
  }
  const points = Math.round(((treated.goal / treated.entered - control.goal / control.entered) * 1000) / 10);

  return `${points > 0 ? "+" : ""}${points} pts`;
}

/*
 * goalWords says a goal in a sentence.
 */
export function goalWords(
  goal: { kind: WorkflowGoal["kind"]; window?: string; min_rating?: number | null } | null | undefined,
): string {
  if (!goal) {
    return "No goal";
  }
  const within = `within ${windowWords(goal.window || "7d")}`;
  if (goal.kind === "csat") {
    return `Rated ${goal.min_rating ?? 4} or more ${within}`;
  }

  return `Resolved without a handover ${within}`;
}

/*
 * windowWords reads "7d", "12h" or "30m" as words.
 */
export function windowWords(window: string): string {
  const match = /^(\d+)([dhm])$/.exec(window.trim());
  if (!match) {
    return window;
  }
  const n = Number(match[1]);
  const unit = { d: ["day", "days"], h: ["hour", "hours"], m: ["minute", "minutes"] }[match[2] as "d" | "h" | "m"];

  return `${n} ${n === 1 ? unit[0] : unit[1]}`;
}

/*
 * daysBack is the first day of a range of the last days, as the API
 * reads days (YYYY-MM-DD, local).
 */
export function daysBack(days: number, today = new Date()): string {
  const from = new Date(today.getFullYear(), today.getMonth(), today.getDate() - (days - 1));
  const pad = (n: number) => String(n).padStart(2, "0");

  return `${from.getFullYear()}-${pad(from.getMonth() + 1)}-${pad(from.getDate())}`;
}

export type ChangeMark = "added" | "removed" | "changed" | "moved";

export interface WorkflowChange {
  key: string;
  label: string;
  mark: ChangeMark;
  before?: string;
  after?: string;
  /* Set for a message's text: its lines, marked. */
  lines?: GuidanceLine[];
}

/* stable writes a value as JSON with sorted keys, so key order never reads as a change. */
function stable(value: unknown): string {
  if (Array.isArray(value)) {
    return `[${value.map(stable).join(",")}]`;
  }
  if (value && typeof value === "object") {
    const entries = Object.entries(value as Record<string, unknown>)
      .filter(([, v]) => v !== undefined)
      .sort(([a], [b]) => a.localeCompare(b));

    return `{${entries.map(([k, v]) => `${JSON.stringify(k)}:${stable(v)}`).join(",")}}`;
  }

  return JSON.stringify(value ?? null);
}

const modes: Record<string, string> = {
  any: "Any time",
  open: "Only when open",
  closed: "Only when closed",
  every_time: "Every time",
  once_per_conversation: "Once per conversation",
  cooldown: "After a cooldown",
};

function head(spec: Spec): [string, string, string][] {
  const trigger = triggers[spec.trigger as TriggerKind]?.name ?? spec.trigger;
  const reentry = modes[spec.reentry?.mode] ?? spec.reentry?.mode ?? "Default";

  return [
    ["trigger", "Trigger", trigger],
    ["trigger_settings", "Trigger settings", stable(spec.trigger_settings ?? {})],
    ["channels", "Channels", spec.channels?.length ? spec.channels.join(", ") : "Every channel"],
    ["audience", "Audience", spec.audience ? stable(spec.audience) : "Everyone"],
    ["schedule", "Office hours", modes[spec.schedule?.mode] ?? spec.schedule?.mode ?? "Any time"],
    ["reentry", "Runs again", spec.reentry?.cooldown ? `${reentry}, ${spec.reentry.cooldown}` : reentry],
    ["exclusive", "Stops later workflows", spec.exclusive ? "On" : "Off"],
    ["goal", "Goal", goalWords(spec.goal)],
  ];
}

/* Fields whose value is JSON read as "Changed" rather than as their JSON. */
const opaque = new Set(["trigger_settings", "audience"]);

function stepShape(step: SpecStep): string {
  return stable({
    kind: step.kind,
    settings: step.settings ?? {},
    paths: (step.paths ?? []).map((p) => ({
      id: p.id,
      label: p.label,
      outcome: p.outcome,
      condition: p.condition,
      description: p.description,
    })),
  });
}

/* headChanges lists the rules above the steps that differ. */
function headChanges(before: Spec, after: Spec): WorkflowChange[] {
  const was = new Map(head(before).map(([key, , value]) => [key, value]));

  return head(after).flatMap(([key, label, value]): WorkflowChange[] => {
    const old = was.get(key) ?? "";
    if (old === value) {
      return [];
    }

    return [
      opaque.has(key) ? { key, label, mark: "changed" } : { key, label, mark: "changed", before: old, after: value },
    ];
  });
}

/* stepChanges lists what happened to one step kept from before. */
function stepChanges(prior: PlacedStep, p: PlacedStep, label: string): WorkflowChange[] {
  const out: WorkflowChange[] = [];
  if (prior.under !== p.under) {
    out.push({ key: p.step.id, label, mark: "moved", before: prior.under ?? "The top", after: p.under ?? "The top" });
  }
  if (stepShape(prior.step) === stepShape(p.step)) {
    return out;
  }
  const change: WorkflowChange = { key: `${p.step.id}:edit`, label, mark: "changed" };
  const [a, b] = [prior.step.settings?.text, p.step.settings?.text];
  if (typeof a === "string" && typeof b === "string" && a !== b) {
    change.lines = guidanceLines(a, b);
  }

  return [...out, change];
}

const titled = (p: PlacedStep) => `Step ${p.number} · ${stepLabel(p.step)}`;

/*
 * workflowChanges lists what differs from before to after: the rules
 * above the steps first, then the steps in the order after reads them,
 * and last the steps after no longer has.
 */
export function workflowChanges(before: Spec, after: Spec): WorkflowChange[] {
  const old = new Map(placeSteps(before.steps).map((p) => [p.step.id, p]));
  const placed = placeSteps(after.steps);
  const kept = new Set(placed.map((p) => p.step.id));
  const steps = placed.flatMap((p): WorkflowChange[] => {
    const prior = old.get(p.step.id);

    return prior
      ? stepChanges(prior, p, titled(p))
      : [{ key: p.step.id, label: titled(p), mark: "added", after: p.under ?? undefined }];
  });
  const removed = [...old.values()]
    .filter((p) => !kept.has(p.step.id))
    .map((p): WorkflowChange => ({ key: p.step.id, label: titled(p), mark: "removed" }));

  return [...headChanges(before, after), ...steps, ...removed];
}
