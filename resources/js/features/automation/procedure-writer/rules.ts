/*
 * How a rule reads in the rule builder: its comparisons in words, and a
 * value typed as text turned into the number, true or false, or list
 * the rule compares with, and back.
 */
import type { ConditionOperator } from "@/lib/api/procedures";

import type { Choices } from "../choices";

export const operatorChoices: Choices = [
  ["eq", "is"],
  ["neq", "is not"],
  ["gt", "is more than"],
  ["gte", "is at least"],
  ["lt", "is less than"],
  ["lte", "is at most"],
  ["in", "is one of"],
  ["not_in", "is none of"],
  ["contains", "contains"],
  ["exists", "is set"],
  ["not_exists", "is not set"],
];

export const modeChoices: Choices = [
  ["all", "Match all"],
  ["any", "Match any"],
];

/*
 * writeValue shows a rule's value as text.
 */
export function writeValue(value: unknown): string {
  if (Array.isArray(value)) {
    return value.join(", ");
  }
  return value === undefined || value === null ? "" : String(value);
}

/*
 * FieldType is the kind of value a field holds, which decides the
 * comparisons a rule offers for it and how its value is entered.
 */
export type FieldType = "boolean" | "number" | "choice" | "date" | "text";

const allowed: Record<FieldType, ConditionOperator[]> = {
  boolean: ["eq", "neq", "exists", "not_exists"],
  number: ["eq", "neq", "gt", "gte", "lt", "lte", "exists", "not_exists"],
  choice: ["eq", "neq", "in", "not_in", "exists", "not_exists"],
  date: ["eq", "neq", "gt", "gte", "lt", "lte", "exists", "not_exists"],
  text: ["eq", "neq", "contains", "in", "not_in", "exists", "not_exists"],
};

/*
 * operatorsFor lists the comparisons that make sense for a field type,
 * in the order the picker shows them.
 */
export function operatorsFor(type: FieldType): Choices {
  return operatorChoices.filter(([id]) => allowed[type].includes(id as ConditionOperator));
}

/*
 * fitOperator keeps a rule's comparison when the field's type allows it,
 * and otherwise falls back to "is".
 */
export function fitOperator(type: FieldType, operator: ConditionOperator): ConditionOperator {
  return allowed[type].includes(operator) ? operator : "eq";
}

/*
 * readTyped turns what was entered into the value a rule compares with,
 * by the field's type: a number stays a number, a yes or no is true or
 * false, text stays text even when it looks like a number, and one of
 * and none of take a list.
 */
export function readTyped(type: FieldType, operator: ConditionOperator, text: string): unknown {
  if (operator === "in" || operator === "not_in") {
    return text
      .split(",")
      .map((part) => part.trim())
      .filter(Boolean);
  }
  const trimmed = text.trim();
  if (type === "boolean") {
    return trimmed === "true";
  }
  if (type === "number") {
    return trimmed !== "" && !Number.isNaN(Number(trimmed)) ? Number(trimmed) : trimmed;
  }

  return type === "text" ? text : trimmed;
}

/*
 * typeOfValue guesses a field's type from a value it held, such as an
 * action's last test result.
 */
export function typeOfValue(value: unknown): FieldType {
  if (typeof value === "boolean") {
    return "boolean";
  }
  if (typeof value === "number") {
    return "number";
  }

  return "text";
}
