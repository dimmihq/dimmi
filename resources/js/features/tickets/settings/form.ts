/*
 * The rules the ticket settings screens share: the key a new field gets
 * from its label, which fields a customer's form shows for the answers
 * given so far, which settings of a type a draft changed, and how a
 * reorder within one state group becomes the order of every state.
 */
import type { ShowWhen, StateGroup, TicketField, TicketState, TicketType, TypeChange } from "@/lib/api/tickets";

/*
 * fieldKey turns a label into a field key: lower case, words joined by
 * underscores, at most 40 characters, starting with a letter.
 */
export function fieldKey(label: string): string {
  const key = label
    .normalize("NFKD")
    .replace(/[̀-ͯ]/g, "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^[^a-z]+|_+$/g, "")
    .slice(0, 40)
    .replace(/_+$/, "");

  return key;
}

/*
 * filled reports whether a value counts as an answer.
 */
export function filled(value: unknown): boolean {
  if (value === undefined || value === null || value === "") {
    return false;
  }

  return !(Array.isArray(value) && value.length === 0);
}

/*
 * conditionMet reports whether a field's show-when holds for the values:
 * the other field has a value, or one of the listed ones.
 */
export function conditionMet(when: ShowWhen | null, values: Record<string, unknown>): boolean {
  if (!when) {
    return true;
  }
  const value = values[when.field];
  if (!filled(value)) {
    return false;
  }
  if (!when.equals || when.equals.length === 0) {
    return true;
  }
  const given = Array.isArray(value) ? value.map(String) : [String(value)];

  return given.some((v) => when.equals?.includes(v));
}

/*
 * customerFields are the fields a customer's form shows for the values
 * given: live, visible to customers, and with their condition met.
 */
export function customerFields(fields: TicketField[], values: Record<string, unknown>): TicketField[] {
  return fields.filter((f) => !f.archived_at && f.visible_customer && conditionMet(f.show_when, values));
}

const typeKeys = [
  "name",
  "icon",
  "description",
  "inbox_id",
  "team_id",
  "customer_visible",
  "company_sharing",
  "reopen_days",
  "auto_resolve_days",
  "auto_resolve_message",
  "ai_auto_fill",
  "state_ids",
  "default_state_id",
  "default_resolved_state_id",
] as const satisfies readonly (keyof TypeChange)[];

/*
 * TypeDraft is the editable part of a type.
 */
export type TypeDraft = Pick<TicketType, (typeof typeKeys)[number]>;

export function draftOf(type: TicketType): TypeDraft {
  const out = {} as Record<string, unknown>;
  for (const key of typeKeys) {
    out[key] = type[key];
  }

  return out as TypeDraft;
}

/*
 * typeChanges returns the settings the draft changed, to send as a patch.
 */
export function typeChanges(saved: TypeDraft, draft: TypeDraft): TypeChange {
  const out: Record<string, unknown> = {};
  for (const key of typeKeys) {
    if (JSON.stringify(saved[key]) !== JSON.stringify(draft[key])) {
      out[key] = draft[key];
    }
  }

  return out as TypeChange;
}

/*
 * byGroup splits the live states into their groups, in order.
 */
export function byGroup(states: TicketState[]): Record<StateGroup, TicketState[]> {
  const out: Record<StateGroup, TicketState[]> = {
    submitted: [],
    in_progress: [],
    waiting_customer: [],
    waiting_team: [],
    resolved: [],
  };
  for (const state of [...states].sort((a, b) => a.position - b.position)) {
    if (!state.archived_at) {
      out[state.group].push(state);
    }
  }

  return out;
}

/*
 * withGroupOrder returns every live state's id in order after one group
 * takes the order given, the other groups keeping theirs.
 */
export function withGroupOrder(states: TicketState[], group: StateGroup, order: string[]): string[] {
  const groups = byGroup(states);

  return (Object.keys(groups) as StateGroup[]).flatMap((g) => (g === group ? order : groups[g].map((s) => s.id)));
}

/*
 * TypeSteps is a type's states split into their groups, each in the
 * type's own order.
 */
export type TypeSteps = Record<StateGroup, TicketState[]>;

/*
 * typeSteps splits the states a type uses into their groups, in the
 * order the type lists them.
 */
export function typeSteps(stateIds: string[], states: TicketState[]): TypeSteps {
  const out: TypeSteps = { submitted: [], in_progress: [], waiting_customer: [], waiting_team: [], resolved: [] };
  for (const id of stateIds) {
    const state = states.find((s) => s.id === id && !s.archived_at);
    if (state) {
      out[state.group].push(state);
    }
  }

  return out;
}

/*
 * stepOrder is every state id of the steps, groups in the order a ticket
 * moves through them.
 */
export function stepOrder(steps: TypeSteps): string[] {
  return (Object.keys(steps) as StateGroup[]).flatMap((g) => steps[g].map((s) => s.id));
}

/*
 * withSteps returns the draft change for new steps: the states in order,
 * new tickets starting in the first submitted state and closing the
 * conversation moving to the first resolved one.
 */
export function withSteps(
  steps: TypeSteps,
): Pick<TypeDraft, "state_ids" | "default_state_id" | "default_resolved_state_id"> {
  return {
    state_ids: stepOrder(steps),
    default_state_id: steps.submitted[0]?.id ?? null,
    default_resolved_state_id: steps.resolved[0]?.id ?? null,
  };
}

/*
 * requiredGroups always keep at least one state: a ticket has to start,
 * be worked on and end somewhere.
 */
export const requiredGroups: StateGroup[] = ["submitted", "in_progress", "resolved"];

/*
 * addable lists the live states of the group a type could add: those
 * it does not use yet.
 */
export function addable(states: TicketState[], group: StateGroup, used: string[]): TicketState[] {
  return byGroup(states)[group].filter((s) => !used.includes(s.id));
}
