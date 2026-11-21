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
 * readValue turns what was typed into the value a rule compares with:
 * a number when it reads as one, true or false, a list for one of and
 * none of, else the text.
 */
export function readValue(operator: ConditionOperator, text: string): unknown {
  if (operator === "in" || operator === "not_in") {
    return text
      .split(",")
      .map((part) => part.trim())
      .filter(Boolean);
  }
  const trimmed = text.trim();
  if (trimmed === "true" || trimmed === "false") {
    return trimmed === "true";
  }
  if (trimmed !== "" && !Number.isNaN(Number(trimmed))) {
    return Number(trimmed);
  }
  return text;
}

/*
 * writeValue shows a rule's value as text.
 */
export function writeValue(value: unknown): string {
  if (Array.isArray(value)) {
    return value.join(", ");
  }
  return value === undefined || value === null ? "" : String(value);
}
