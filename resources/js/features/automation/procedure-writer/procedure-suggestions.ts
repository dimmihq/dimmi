/*
 * What typing @ in a procedure step offers, grouped: the workspace's
 * published actions, with whether each only reads and who agrees before
 * a write runs; customer and company details, and the fields earlier
 * actions return; help articles; what dimmi can do in the inbox (tag the
 * conversation, hand it to a team, open a ticket of a type); and the
 * reusable sub-procedures. Every chip shows its current name, falling
 * back to the name it had when written.
 */
import type { Action } from "@/lib/api/ai-actions";
import type { AttributeDefinition } from "@/lib/api/attributes";
import type { BindingKind } from "@/lib/api/procedures";

import type { Chip } from "./tokens";

export interface Named {
  id: string;
  name: string;
}

/*
 * ChipSources is everything a chip can name.
 */
export interface ChipSources {
  actions: Action[];
  attributes: AttributeDefinition[];
  articles: Named[];
  teams: Named[];
  tags: Named[];
  ticketTypes: Named[];
  procedures: Named[];
}

export const noSources: ChipSources = {
  actions: [],
  attributes: [],
  articles: [],
  teams: [],
  tags: [],
  ticketTypes: [],
  procedures: [],
};

export interface ChipOption extends Chip {
  group: string;
  label: string;
  hint: string;
  first?: boolean;
}

const perGroup = 6;

/*
 * actionHint says what an action does to the account and who agrees.
 */
export function actionHint(action: Action): string {
  if (action.side_effect === "read") {
    return "Read-only lookup";
  }
  const parts = ["Changes the account"];
  if (action.confirmation === "customer") {
    parts.push("customer confirms");
  }
  if (action.approval !== "none") {
    parts.push("teammate approves");
  }
  return parts.join(" · ");
}

/*
 * Field is a value a rule or a chip can read: a customer or company
 * detail, or a field an action returned in its last test.
 */
export interface Field {
  field: string;
  label: string;
}

/*
 * fieldsOf lists every field rules can read: the conversation's own,
 * the customer's and company's details, and what each action returned
 * in its last live test.
 */
export function fieldsOf(sources: ChipSources): Field[] {
  const out: Field[] = [
    { field: "conversation.channel", label: "Channel" },
    { field: "conversation.verified", label: "Customer is signed in" },
    { field: "contact.email", label: "Customer email" },
    { field: "contact.name", label: "Customer name" },
    { field: "contact.external_id", label: "Customer user id" },
  ];
  for (const attr of sources.attributes) {
    if (attr.status === "active") {
      const owner = attr.of === "contact" ? "Customer" : "Company";
      out.push({ field: `${attr.of}.attr.${attr.key}`, label: `${owner} ${attr.label}` });
    }
  }
  for (const action of sources.actions) {
    out.push(...resultFields(action));
  }
  return out;
}

/*
 * resultFields lists the fields an action returned in its last live
 * test, one level of nesting deep.
 */
function resultFields(action: Action): Field[] {
  const sample = action.last_test_response;
  if (!sample || typeof sample !== "object" || Array.isArray(sample)) {
    return [];
  }
  const name = action.title || action.name;
  const out: Field[] = [];
  for (const [key, value] of Object.entries(sample as Record<string, unknown>)) {
    out.push({ field: `action.${action.name}.${key}`, label: `${name} → ${key}` });
    const nested = value && typeof value === "object" && !Array.isArray(value) ? Object.keys(value) : [];
    for (const inner of nested) {
      out.push({ field: `action.${action.name}.${key}.${inner}`, label: `${name} → ${key}.${inner}` });
    }
  }
  return out;
}

/*
 * allChips lists every chip in its group, in the order the list shows
 * the groups.
 */
export function allChips(sources: ChipSources): ChipOption[] {
  const named = (group: string, kind: BindingKind, list: Named[], hint: string) =>
    list.map((n) => ({ group, kind, ref: n.id, label: n.name, hint }));
  const actions = sources.actions
    .filter((a) => a.status === "published")
    .map((a) => ({
      group: "Actions",
      kind: "action" as const,
      ref: a.id,
      label: a.title || a.name,
      hint: actionHint(a),
    }));
  const fields = fieldsOf(sources).map((f) => ({
    group: "Customer & company",
    kind: "attribute" as const,
    ref: f.field,
    label: f.label,
    hint: f.field,
  }));

  return [
    ...actions,
    ...fields,
    ...named("Help articles", "article", sources.articles, "Cite this article"),
    ...named("Inbox actions", "tag", sources.tags, "Tag the conversation"),
    ...named("Inbox actions", "handoff", sources.teams, "Hand over to this team"),
    ...named("Inbox actions", "ticket_type", sources.ticketTypes, "Open a ticket of this type"),
    ...named("Sub-procedures", "sub_procedure", sources.procedures, "Follow these steps here"),
  ];
}

/*
 * findChips lists the chips whose name matches what was typed after the
 * @, a few per group, marking the first of each group for its heading.
 */
export function findChips(sources: ChipSources, query: string): ChipOption[] {
  const needle = query.trim().toLowerCase();
  const counts = new Map<string, number>();
  const out: ChipOption[] = [];
  for (const option of allChips(sources)) {
    if (needle && !option.label.toLowerCase().includes(needle) && !option.ref.toLowerCase().includes(needle)) {
      continue;
    }
    const n = counts.get(option.group) ?? 0;
    if (n >= perGroup) {
      continue;
    }
    counts.set(option.group, n + 1);
    out.push({ ...option, first: n === 0 });
  }
  return out;
}

/*
 * labelOf names a chip by what it points at now.
 */
export function labelOf(sources: ChipSources, chip: Chip): string {
  switch (chip.kind) {
    case "action": {
      const action = sources.actions.find((a) => a.id === chip.ref);
      return action ? action.title || action.name : "";
    }
    case "attribute":
      return fieldsOf(sources).find((f) => f.field === chip.ref)?.label ?? chip.ref;
    case "article":
      return sources.articles.find((a) => a.id === chip.ref)?.name ?? "";
    case "handoff":
      return sources.teams.find((t) => t.id === chip.ref)?.name ?? "";
    case "tag":
      return sources.tags.find((t) => t.id === chip.ref)?.name ?? "";
    case "ticket_type":
      return sources.ticketTypes.find((t) => t.id === chip.ref)?.name ?? "";
    case "sub_procedure":
      return sources.procedures.find((p) => p.id === chip.ref)?.name ?? "";
  }
  return "";
}
