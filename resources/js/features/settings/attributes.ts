/*
 * Words for the attributes page: each type and problem as people say it,
 * and a definition's line under its label.
 */
import type { AttributeDefinition, AttributeProblem, AttributeType } from "@/lib/api/attributes";

export const typeNames: Record<AttributeType, string> = {
  string: "Text",
  number: "Number",
  boolean: "Yes or no",
  date: "Date",
  list: "List",
};

export const problemNames: Record<AttributeProblem["code"], string> = {
  type_mismatch: "Wrong type",
  invalid_option: "Not in the list",
  archived_write: "Archived",
  key_limit_reached: "Too many attributes",
};

/*
 * ago says how long ago a time was, roughly.
 */
export function ago(at: string, now: Date = new Date()): string {
  const minutes = Math.max(0, Math.round((now.getTime() - new Date(at).getTime()) / 60_000));
  if (minutes < 60) {
    return minutes <= 1 ? "just now" : `${minutes} minutes ago`;
  }
  const hours = Math.round(minutes / 60);
  if (hours < 24) {
    return hours === 1 ? "1 hour ago" : `${hours} hours ago`;
  }
  const days = Math.round(hours / 24);

  return days === 1 ? "yesterday" : `${days} days ago`;
}

/*
 * definitionLine is a definition's key, type and last value, and whether
 * it was seen rather than made, or archived.
 */
export function definitionLine(d: AttributeDefinition, now: Date = new Date()): string {
  const parts = [d.key, typeNames[d.data_type]];
  if (d.data_type === "list" && d.options.length > 0) {
    parts.push(d.options.join(", "));
  }
  parts.push(d.last_value_at ? `last value ${ago(d.last_value_at, now)}` : "no value yet");
  if (d.status === "archived") {
    parts.push("archived");
  } else if (d.origin === "observed") {
    parts.push("seen in data");
  }

  return parts.join(" · ");
}
