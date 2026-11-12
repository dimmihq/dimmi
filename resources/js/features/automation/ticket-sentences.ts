/*
 * What the ticket and SLA steps' cards say, naming the type, state,
 * tracker, team or policy picked rather than its id.
 */
import type { Names, Settings } from "./step-settings";
import type { Step } from "./workflows";

const unset = "Not set up yet";

function setting(settings: Settings | undefined, name: string): string {
  const value = settings?.[name];

  return typeof value === "string" ? value : "";
}

function named(names: Names, id: string): string {
  return names.tickets?.get(id) ?? names.teams.get(id) ?? "a removed item";
}

function fieldCount(step: Step): number {
  return Array.isArray(step.settings?.fields) ? step.settings.fields.length : 0;
}

function withFields(sentence: string, step: Step): string {
  const count = fieldCount(step);
  if (count === 0) {
    return sentence;
  }

  return `${sentence}, setting ${count === 1 ? "1 field" : `${count} fields`}`;
}

export const ticketSentences: Record<string, (step: Step, names: Names) => string> = {
  create_ticket: (step, names) => {
    const type = setting(step.settings, "type_id");

    return type ? withFields(`Make it a ${named(names, type)} ticket`, step) : unset;
  },
  set_ticket_state: (step, names) => {
    const state = setting(step.settings, "state_id");

    return state ? `Move the ticket to ${named(names, state)}` : unset;
  },
  set_ticket_fields: (step) => {
    const count = fieldCount(step);

    return count === 0 ? unset : `Set ${count === 1 ? "1 ticket field" : `${count} ticket fields`}`;
  },
  escalate: (step, names) => {
    const type = setting(step.settings, "type_id");
    const team = setting(step.settings, "team_id");
    if (!type) {
      return unset;
    }

    return withFields(`Escalate as ${named(names, type)}${team ? ` to ${named(names, team)}` : ""}`, step);
  },
  add_to_tracker: (step, names) => {
    const tracker = setting(step.settings, "tracker_id");

    return tracker ? `Add to the tracker ${named(names, tracker)}` : unset;
  },
  apply_sla: (step, names) => {
    const policy = setting(step.settings, "sla_policy_id");

    return policy ? `Apply the ${named(names, policy)} SLA` : unset;
  },
};
