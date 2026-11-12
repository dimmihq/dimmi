/*
 * Which of a type's fields show for the values given, and which of those
 * a create or a resolve still needs.
 */
import type { TicketField } from "@/lib/api/tickets";

import { conditionMet, filled } from "./settings/form";

/*
 * shownFields are a type's live fields whose show-when holds for the
 * values.
 */
export function shownFields(fields: TicketField[], values: Record<string, unknown>): TicketField[] {
  return fields.filter((f) => !f.archived_at && conditionMet(f.show_when, values));
}

/*
 * missingFor lists the keys of shown fields the purpose needs but the
 * values leave empty.
 */
export function missingFor(
  fields: TicketField[],
  values: Record<string, unknown>,
  purpose: "create" | "resolve",
): string[] {
  return shownFields(fields, values)
    .filter((f) => (purpose === "create" ? f.required_create : f.required_resolve) && !filled(values[f.key]))
    .map((f) => f.key);
}
