/*
 * The settings of each kind of workflow step, and the sentence its card
 * shows for them. The card always says what the step will do with the
 * settings as they stand, so a change in the inspector shows on the
 * canvas as it is made. Steps that came with the example workspace carry
 * only their sentence; their settings are read back from it.
 */
import type { Step, StepKind } from "./workflows";

export type Fields = Record<string, string>;

export const unsetSummary = "Not set up yet";

export const options = {
  assignee: ["Support team", "Billing team", "Sam Okafor", "Lea Martin"],
  tag: ["VIP", "Bug", "Billing", "Sign-in"],
  wait: ["10 minutes", "1 hour", "1 day"],
  saveAs: ["Email", "Name", "Company", "Plan"],
};

const defaultQuestion = "What's the best email to reach you?";

function written(fields: Fields): string {
  return fields.text?.trim() || unsetSummary;
}

function assignee(name = ""): string {
  return name.endsWith(" team") ? `Assign to the ${name}` : `Assign to ${name}`;
}

/*
 * describe turns a step's settings into its card's sentence. Kinds that
 * are missing keep the sentence they have, as nothing in their settings
 * changes what the card says.
 */
export const describe: Partial<Record<StepKind, (fields: Fields) => string>> = {
  message: written,
  note: written,
  buttons: written,
  collect: (fields) => (fields.text?.trim() ? `Ask "${fields.text.trim()}" → ${fields.saveAs}` : unsetSummary),
  assign: (fields) => assignee(fields.assignee),
  tag: (fields) => `Tag ${fields.tag}`,
  wait: (fields) => (fields.stop === "off" ? `Wait ${fields.wait}, even if they reply` : `Wait ${fields.wait}`),
  close: (fields) => (fields.rating === "on" ? "Ask for a rating, then close" : "Close the conversation"),
};

/*
 * fieldsOf is what the inspector shows for a step: the settings it was
 * given, over the ones its sentence implies.
 */
export function fieldsOf(step: Pick<Step, "kind" | "summary" | "fields">): Fields {
  const text = step.summary === unsetSummary ? "" : step.summary;
  const pick = (list: string[]) => list.find((option) => step.summary.endsWith(option)) ?? list[0] ?? "";
  const implied: Partial<Record<StepKind, Fields>> = {
    message: { text },
    note: { text },
    buttons: { text },
    collect: { text: defaultQuestion, saveAs: pick(options.saveAs) },
    assign: { assignee: pick(options.assignee) },
    tag: { tag: pick(options.tag) },
    wait: { wait: pick(options.wait), stop: step.summary.endsWith("even if they reply") ? "off" : "on" },
    close: { rating: step.summary.startsWith("Ask for a rating") ? "on" : "off" },
  };

  return { ...implied[step.kind], ...step.fields };
}

/*
 * withField sets one setting of a step and rewrites its sentence to match.
 */
export function withField(step: Step, name: string, value: string): Step {
  const fields = { ...fieldsOf(step), [name]: value };

  return { ...step, fields, summary: describe[step.kind]?.(fields) ?? step.summary };
}

/*
 * withPathLabel renames one of a step's paths, as a reply button's text
 * is also the name of the path it leads down.
 */
export function withPathLabel(step: Step, path: string, label: string): Step {
  return { ...step, paths: step.paths?.map((current) => (current.id === path ? { ...current, label } : current)) };
}
