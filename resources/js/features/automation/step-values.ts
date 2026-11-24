/*
 * What the extract and run action steps' editors work out from their
 * settings: the key a value's name is saved under, the path for a value
 * not found, the inputs an action takes, and where an input's value
 * comes from, read from and written to the variable the server fills.
 */
import type { Action } from "@/lib/api/ai-actions";

import { newId, type Step } from "./workflows";

/*
 * keyFor turns a value's name into the key it is saved under.
 */
export function keyFor(label: string): string {
  return label
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^[^a-z]+|_+$/g, "")
    .slice(0, 64);
}

/*
 * withNonePath gives the step its own path for a value not found, or
 * takes it away with every step down it.
 */
export function withNonePath(step: Step, on: boolean): Step {
  const others = (step.paths ?? []).filter((path) => path.outcome !== "none");
  const paths = on ? [...others, { id: newId("p_"), outcome: "none", steps: [] }] : others;

  return { ...step, paths: paths.length > 0 ? paths : undefined };
}

/*
 * inputsOf lists an action's inputs the step fills, required first, the
 * ones the admin bound left out.
 */
export function inputsOf(action: Action | undefined): { name: string; required: boolean }[] {
  const schema = action?.input_schema as { properties?: Record<string, unknown>; required?: string[] } | undefined;
  const required = schema?.required ?? [];
  const bound = Object.keys(action?.bound_arguments ?? {});

  return Object.keys(schema?.properties ?? {})
    .filter((name) => !bound.includes(name))
    .map((name) => ({ name, required: required.includes(name) }))
    .sort((a, b) => Number(b.required) - Number(a.required));
}

/*
 * details are the details an input can read, by the variable that holds
 * them.
 */
export const details: Record<string, string> = {
  "contact.email": "Contact email",
  "contact.name": "Contact name",
  "contact.first_name": "Contact first name",
  "contact.phone": "Contact phone",
  "company.name": "Company name",
  "conversation.number": "Conversation number",
  "conversation.subject": "Conversation subject",
};

/*
 * The sources an input may come from besides a detail.
 */
export const fixed = "fixed";
export const runVar = "run.vars.";
export const contactAttr = "contact.custom.";
export const aiAttr = "conversation.ai.";

/*
 * sourceOf reads a stored value as where it comes from and the name it
 * names, if any.
 */
export function sourceOf(value: string): [source: string, name: string] {
  const variable = /^\{\{\s*([a-z0-9_.]+)\s*\}\}$/.exec(value)?.[1];
  if (!variable) {
    return [fixed, value];
  }
  if (variable in details) {
    return [variable, ""];
  }
  for (const prefix of [aiAttr, runVar, contactAttr]) {
    if (variable.startsWith(prefix)) {
      const name = variable.slice(prefix.length);

      return prefix === aiAttr ? [aiAttr + name, ""] : [prefix, name];
    }
  }

  return [fixed, value];
}

/*
 * storedValue writes where a value comes from as the setting stores it.
 */
export function storedValue(source: string, name: string): string {
  if (source === fixed) {
    return name;
  }
  if (source === runVar || source === contactAttr) {
    return name.trim() ? `{{${source}${name.trim()}}}` : "";
  }

  return `{{${source}}}`;
}
