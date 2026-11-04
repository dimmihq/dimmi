/*
 * The conditions the view editor offers, each a field with its
 * operators and the kind of value it takes, in the words a teammate
 * reads. The server knows more fields; the editor keeps to the ones a
 * person builds by hand and shows any other as it was saved.
 */
import type { ViewCondition, ViewDefinition } from "@/lib/api/views";

export type ValueKind = "statuses" | "level" | "member" | "team" | "inbox" | "channels" | "flag" | "text" | "duration";

export interface FieldSpec {
  field: string;
  label: string;
  ops: { op: string; label: string; kind?: ValueKind }[];
}

export const fieldSpecs: FieldSpec[] = [
  { field: "status", label: "Status", ops: [{ op: "in", label: "is", kind: "statuses" }] },
  {
    field: "priority",
    label: "Priority",
    ops: [
      { op: "gte", label: "is at least", kind: "level" },
      { op: "eq", label: "is", kind: "level" },
    ],
  },
  {
    field: "assignee",
    label: "Assignee",
    ops: [
      { op: "in", label: "is", kind: "member" },
      { op: "not_exists", label: "is nobody" },
      { op: "exists", label: "is someone" },
    ],
  },
  {
    field: "team",
    label: "Team",
    ops: [
      { op: "in", label: "is", kind: "team" },
      { op: "not_exists", label: "is no team" },
    ],
  },
  { field: "inbox", label: "Inbox", ops: [{ op: "in", label: "is", kind: "inbox" }] },
  { field: "channel", label: "Channel", ops: [{ op: "in", label: "is", kind: "channels" }] },
  { field: "unread", label: "Unread", ops: [{ op: "eq", label: "is", kind: "flag" }] },
  { field: "waiting", label: "Waiting for a reply", ops: [{ op: "eq", label: "is", kind: "flag" }] },
  { field: "subject", label: "Subject", ops: [{ op: "contains", label: "contains", kind: "text" }] },
  { field: "text", label: "Any message", ops: [{ op: "matches", label: "matches", kind: "text" }] },
  {
    field: "waiting_since",
    label: "Waiting since",
    ops: [{ op: "older_than", label: "more than", kind: "duration" }],
  },
  {
    field: "created_at",
    label: "Started",
    ops: [
      { op: "older_than", label: "more than", kind: "duration" },
      { op: "newer_than", label: "less than", kind: "duration" },
    ],
  },
];

/*
 * durations are the spans a time condition offers, as ISO 8601.
 */
export const durations = [
  { value: "PT1H", label: "1 hour ago" },
  { value: "PT4H", label: "4 hours ago" },
  { value: "P1D", label: "1 day ago" },
  { value: "P3D", label: "3 days ago" },
  { value: "P1W", label: "1 week ago" },
];

/*
 * blankCondition is the condition a new row starts with.
 */
export function blankCondition(): ViewCondition {
  return { field: "status", op: "in", value: ["open"] };
}

/*
 * blankDefinition is the definition a new view starts with: one group
 * of one condition.
 */
export function blankDefinition(): ViewDefinition {
  return { op: "and", groups: [{ op: "and", conditions: [blankCondition()] }] };
}

/*
 * opFor returns the operator spec of a condition, or the first of its
 * field.
 */
export function opFor(condition: ViewCondition) {
  const spec = fieldSpecs.find((f) => f.field === condition.field);
  return spec?.ops.find((o) => o.op === condition.op) ?? spec?.ops[0];
}

/*
 * defaultValue is the value a condition takes when its operator changes
 * to one of the kind.
 */
export function defaultValue(kind: ValueKind | undefined): unknown {
  switch (kind) {
    case "statuses":
      return ["open"];
    case "level":
      return "high";
    case "member":
      return ["me"];
    case "channels":
      return ["email"];
    case "flag":
      return true;
    case "duration":
      return "P1D";
    case "text":
      return "";
    default:
      return undefined;
  }
}

/*
 * warned reports whether a warning names the condition at the place.
 */
export function warned(warnings: { path: string }[], group: number, condition: number): boolean {
  return warnings.some((w) => w.path === `groups.${group}.conditions.${condition}`);
}

export type KeyedCondition = ViewCondition & { key: string };

export interface KeyedGroup {
  key: string;
  op: "and" | "or";
  conditions: KeyedCondition[];
}

/*
 * keyed gives each group and condition a key of its own, so the editor's
 * rows keep their place as some are removed.
 */
export function keyed(definition: ViewDefinition): { op: "and" | "or"; groups: KeyedGroup[] } {
  return {
    op: definition.op,
    groups: definition.groups.map((g) => ({
      key: crypto.randomUUID(),
      op: g.op,
      conditions: g.conditions.map((c) => ({ ...c, key: crypto.randomUUID() })),
    })),
  };
}

/*
 * unkeyed returns the definition the API stores, without the editor's
 * keys.
 */
export function unkeyed(draft: { op: "and" | "or"; groups: KeyedGroup[] }): ViewDefinition {
  return {
    op: draft.op,
    groups: draft.groups.map((g) => ({ op: g.op, conditions: g.conditions.map(({ key: _, ...c }) => c) })),
  };
}

/*
 * keyedCondition is a new blank row.
 */
export function keyedCondition(): KeyedCondition {
  return { ...blankCondition(), key: crypto.randomUUID() };
}
