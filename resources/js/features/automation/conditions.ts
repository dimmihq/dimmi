/*
 * The conditions a branch path can test, in the builder's words. A
 * condition compares one field with a value; the builder offers the
 * fields teams branch on most and the comparisons each kind of field
 * takes. Conditions written through the API may group several rules;
 * the builder shows those as they are and leaves them alone.
 */
import type { Condition } from "@/lib/api/workflows";

import type { TriggerKind } from "./triggers";

type FieldType = "text" | "choice" | "yes_no";

/*
 * ConditionField is a field offered in conditions. Options lists the
 * values of a choice field; triggers limits a field of the event to the
 * triggers that carry it.
 */
export interface ConditionField {
  name: string;
  type: FieldType;
  options?: Record<string, string>;
  triggers?: TriggerKind[];
}

const messageTriggers: TriggerKind[] = [
  "conversation.first_message",
  "conversation.customer_reopened",
  "message.customer",
  "message.teammate",
];

export const conditionFields: Record<string, ConditionField> = {
  "message.body": { name: "Message text", type: "text", triggers: messageTriggers },
  "note.body": { name: "Note text", type: "text", triggers: ["note.created"] },
  "trigger.to": {
    name: "New status",
    type: "choice",
    options: { open: "Open", snoozed: "Snoozed", closed: "Closed" },
    triggers: ["conversation.status_changed"],
  },
  "conversation.channel_type": {
    name: "Channel",
    type: "choice",
    options: { widget: "Messenger", email: "Email" },
  },
  "conversation.priority": {
    name: "Priority",
    type: "choice",
    options: { none: "None", low: "Low", medium: "Medium", high: "High", urgent: "Urgent" },
  },
  "conversation.subject": { name: "Subject", type: "text" },
  "office_hours.open": { name: "Team in office hours", type: "yes_no" },
  "contact.email": { name: "Contact email", type: "text" },
  "contact.email_domain": { name: "Contact email domain", type: "text" },
  "contact.name": { name: "Contact name", type: "text" },
  "contact.kind": { name: "Contact type", type: "choice", options: { user: "User", lead: "Lead" } },
  "company.name": { name: "Company name", type: "text" },
};

const textOps: Record<string, string> = {
  contains_any: "contains any of",
  not_contains_any: "contains none of",
  eq: "is",
  neq: "is not",
  starts_with: "starts with",
  ends_with: "ends with",
  exists: "is known",
  not_exists: "is unknown",
};

const yesNoOps: Record<string, string> = {
  is_true: "is yes",
  is_false: "is no",
};

const choiceOps: Record<string, string> = {
  in: "is any of",
  not_in: "is none of",
  exists: "is known",
  not_exists: "is unknown",
};

/*
 * listOps take a list of values, typed as words separated by commas.
 */
const listOps = new Set(["contains_any", "not_contains_any", "in", "not_in"]);
const bareOps = new Set(["exists", "not_exists", "is_true", "is_false"]);

/*
 * fieldsFor lists the fields offered under a trigger.
 */
export function fieldsFor(trigger: TriggerKind): Record<string, string> {
  const out: Record<string, string> = {};
  for (const [key, field] of Object.entries(conditionFields)) {
    if (!field.triggers || field.triggers.includes(trigger)) {
      out[key] = field.name;
    }
  }

  return out;
}

export function opsFor(field: string): Record<string, string> {
  const type = conditionFields[field]?.type;
  if (type === "yes_no") {
    return yesNoOps;
  }

  return type === "choice" ? choiceOps : textOps;
}

export function needsValue(op: string): boolean {
  return !bareOps.has(op);
}

/*
 * isSimple reports whether the builder can edit a condition: nothing
 * yet, or one field compared with a value.
 */
export function isSimple(condition: Condition | undefined): boolean {
  return !condition || (!condition.all && !condition.any && !condition.not);
}

/*
 * valueText is a condition's value as it is typed: a list as words
 * separated by commas.
 */
export function valueText(condition: Condition | undefined): string {
  const value = condition?.value;
  if (Array.isArray(value)) {
    return value.join(", ");
  }

  return typeof value === "string" ? value : "";
}

/*
 * valueList is the choices of a choice condition that are picked.
 */
export function valueList(condition: Condition | undefined): string[] {
  const value = condition?.value;
  if (Array.isArray(value)) {
    return value.filter((item): item is string => typeof item === "string");
  }

  return typeof value === "string" && value ? [value] : [];
}

/*
 * conditionOf builds a condition from what is picked and typed, the
 * value in the shape its comparison takes.
 */
export function conditionOf(field: string, op: string, typed: string | string[]): Condition {
  if (!needsValue(op)) {
    return { field, op };
  }
  const words = Array.isArray(typed)
    ? typed
    : typed
        .split(",")
        .map((word) => word.trim())
        .filter(Boolean);

  return { field, op, value: listOps.has(op) ? words : (words[0] ?? "") };
}

/*
 * changeField moves a condition to another field, keeping the comparison
 * when the new field takes it and the value when it still fits.
 */
export function changeField(condition: Condition | undefined, field: string): Condition {
  const ops = opsFor(field);
  const op = condition?.op && condition.op in ops ? condition.op : (Object.keys(ops)[0] ?? "eq");
  const kept = conditionFields[field]?.type === conditionFields[condition?.field ?? ""]?.type;

  return conditionOf(field, op, kept ? valueList(condition) : []);
}

/*
 * changeOp changes a condition's comparison, keeping its value.
 */
export function changeOp(condition: Condition | undefined, op: string): Condition {
  return conditionOf(condition?.field ?? "message.body", op, valueList(condition));
}

/*
 * describeCondition says a condition in words, for the path's line.
 */
export function describeCondition(condition: Condition | undefined): string {
  if (!condition?.field || !isSimple(condition)) {
    return condition ? "Several conditions" : "No condition yet";
  }
  const field = conditionFields[condition.field];
  const op = opsFor(condition.field)[condition.op ?? ""] ?? condition.op ?? "";
  const values = valueList(condition).map((value) => field?.options?.[value] ?? value);

  return [field?.name ?? condition.field, op, needsValue(condition.op ?? "") ? values.join(", ") : ""]
    .filter(Boolean)
    .join(" ");
}
