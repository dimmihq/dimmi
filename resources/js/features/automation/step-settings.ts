/*
 * The settings of each kind of workflow step, and the sentence its card
 * shows for them. The card always says what the step will do with the
 * settings as they stand, naming teams, teammates and tags rather than
 * their ids, so a change in the inspector shows on the canvas as it is
 * made. Settings are the server's own, so what the builder saves is
 * exactly what runs.
 */
import { stepLook } from "./step-kinds";
import { ticketSentences } from "./ticket-sentences";
import { newId, type Path, type Step, type StepKind } from "./workflows";

export type Settings = Record<string, unknown>;

export const unsetSummary = "Not set up yet";

/*
 * Names turns the ids a step holds into the names its card shows.
 * Workflows and connections are read only by the steps that name them;
 * tickets names ticket types, states, trackers and SLA policies.
 */
export interface Names {
  teams: ReadonlyMap<string, string>;
  members: ReadonlyMap<string, string>;
  tags: ReadonlyMap<string, string>;
  workflows?: ReadonlyMap<string, string>;
  articles?: ReadonlyMap<string, string>;
  connections?: ReadonlyMap<string, string>;
  tickets?: ReadonlyMap<string, string>;
}

export const noNames: Names = { teams: new Map(), members: new Map(), tags: new Map() };

/*
 * durations are the lengths offered for waits, snoozes and timeouts, in
 * the server's notation.
 */
export const durations: Record<string, string> = {
  "5m": "5 minutes",
  "10m": "10 minutes",
  "30m": "30 minutes",
  "1h": "1 hour",
  "4h": "4 hours",
  "1d": "1 day",
  "3d": "3 days",
  "7d": "7 days",
};

export const priorities: Record<string, string> = {
  none: "No priority",
  low: "Low",
  medium: "Medium",
  high: "High",
  urgent: "Urgent",
};

/*
 * collectTargets are where a collected answer can be saved, and the type
 * of answer each expects.
 */
export const collectTargets: Record<string, { name: string; type: string }> = {
  "contact.email": { name: "Email", type: "email" },
  "contact.name": { name: "Name", type: "text" },
  "contact.phone": { name: "Phone", type: "phone" },
};

export const unassignTargets: Record<string, string> = {
  both: "Teammate and team",
  member: "Teammate only",
  team: "Team only",
};

export const contactFields: Record<string, string> = {
  "contact.name": "Name",
  "contact.phone": "Phone",
};

/*
 * text reads a text setting, empty when unset.
 */
export function text(settings: Settings | undefined, name: string): string {
  const value = settings?.[name];

  return typeof value === "string" ? value : "";
}

/*
 * list reads a list setting of strings, empty when unset.
 */
export function list(settings: Settings | undefined, name: string): string[] {
  const value = settings?.[name];

  return Array.isArray(value) ? value.filter((item): item is string => typeof item === "string") : [];
}

function written(step: Step): string {
  return text(step.settings, "text").trim() || unsetSummary;
}

function lasting(value: string): string {
  return durations[value] ?? value;
}

const noMap: ReadonlyMap<string, string> = new Map();

function nameOf(names: ReadonlyMap<string, string>, id: string): string {
  return names.get(id) ?? "a removed item";
}

function assigned(settings: Settings | undefined, names: Names): string {
  const mode = text(settings, "mode");
  if (mode === "member") {
    const member = text(settings, "member_id");

    return member ? `Assign to ${nameOf(names.members, member)}` : unsetSummary;
  }
  const team = text(settings, "team_id");
  if (!team) {
    return unsetSummary;
  }
  const how: Record<string, string> = {
    team_round_robin: " by round robin",
    team_balanced: ", to whoever holds the fewest",
  };

  return `Assign to ${nameOf(names.teams, team)}${how[mode] ?? ""}`;
}

function tagged(step: Step, names: Names, verb: string): string {
  const tags = list(step.settings, "tag_ids").map((id) => nameOf(names.tags, id));

  return tags.length > 0 ? `${verb} ${tags.join(", ")}` : unsetSummary;
}

function waited(step: Step, verb: string): string {
  const length = text(step.settings, "for");
  if (length) {
    const office = text(step.settings, "count") === "business" ? " of office hours" : "";

    return `${verb} for ${lasting(length)}${office}`;
  }
  const until = step.settings?.until as { kind?: string; time?: string } | undefined;
  if (until?.kind === "office_hours_open") {
    return `${verb} until office hours open`;
  }

  return until?.time ? `${verb} until ${until.time}` : unsetSummary;
}

function collected(step: Step): string {
  const question = text(step.settings, "text").trim();
  const field = text(step.settings, "field");
  const target = collectTargets[field]?.name ?? field;

  return question ? `Ask "${question}" → ${target}` : unsetSummary;
}

/*
 * replyTimeText is what a reply time step says when no message is typed.
 */
export const replyTimeText = "We typically reply {{reply_time}}.";

function called(step: Step, names: Names): string {
  const url = text(step.settings, "url").trim();
  if (!url) {
    return unsetSummary;
  }
  const method = text(step.settings, "method") || "GET";
  const connection = text(step.settings, "connection_id");
  const where = connection ? `${nameOf(names.connections ?? noMap, connection)} ${url}` : url;

  return method === "GET" ? `GET ${where}` : `${method} to ${where}`;
}

function sorted(step: Step): string {
  const categories = (step.paths ?? []).filter((path) => !path.outcome).map((path) => path.label?.trim() ?? "");
  const named = categories.filter(Boolean);

  return named.length > 0 ? `Sort into ${named.join(", ")}` : unsetSummary;
}

const describers: Partial<Record<StepKind, (step: Step, names: Names) => string>> = {
  message: written,
  note: written,
  article: (step, names) => {
    const article = text(step.settings, "article");

    return article ? `Send ${nameOf(names.articles ?? noMap, article)}` : unsetSummary;
  },
  buttons: written,
  collect: collected,
  reply_time: (step) => text(step.settings, "text").trim() || replyTimeText,
  branch: () => "Go down the first path that matches",
  classify: sorted,
  run_workflow: (step, names) => {
    const workflow = text(step.settings, "workflow_id");

    return workflow ? `Run ${nameOf(names.workflows ?? noMap, workflow)}` : unsetSummary;
  },
  http_request: called,
  dimmi: (step) => {
    const instructions = text(step.settings, "instructions").trim();

    return instructions
      ? `dimmi answers, following: ${instructions}`
      : "dimmi answers until it resolves, hands over or the customer goes quiet";
  },
  wait: (step) => waited(step, "Wait"),
  snooze: (step) => waited(step, "Snooze"),
  assign: (step, names) => assigned(step.settings, names),
  unassign: (step) => `Unassign the ${unassignTargets[text(step.settings, "what")]?.toLowerCase() ?? "conversation"}`,
  tag: (step, names) => tagged(step, names, "Tag"),
  untag: (step, names) => tagged(step, names, "Remove"),
  priority: (step) => `Set priority to ${priorities[text(step.settings, "value")]?.toLowerCase() ?? "high"}`,
  update_contact: (step) => {
    const count = Array.isArray(step.settings?.set) ? step.settings.set.length : 0;

    return count === 1 ? "Update 1 contact field" : `Update ${count} contact fields`;
  },
  close: (step) => {
    const closing = step.settings?.only_if_no_teammate_reply
      ? "Close if no teammate replied"
      : "Close the conversation";

    return step.settings?.ask_rating ? `${closing} and ask for a rating` : closing;
  },
};

/*
 * sentence is the sentence a step's card shows. Kinds the server does
 * not run yet say so.
 */
export function sentence(step: Step, names: Names): string {
  const kind = step.kind as StepKind;
  const describer = describers[kind] ?? ticketSentences[kind];
  if (!describer) {
    return `${stepLook[kind]?.name ?? step.kind}, not available yet`;
  }

  return describer(step, names);
}

/*
 * withSettings changes some settings of a step, dropping those set to
 * undefined, and leaves the rest as they were.
 */
export function withSettings(step: Step, changes: Settings): Step {
  const settings: Settings = { ...step.settings, ...changes };
  for (const [name, value] of Object.entries(changes)) {
    if (value === undefined) {
      delete settings[name];
    }
  }

  return { ...step, settings };
}

/*
 * withPathLabel renames one of a step's paths, as a reply button's text
 * is also the name of the path it leads down.
 */
export function withPathLabel(step: Step, path: string, label: string): Step {
  return { ...step, paths: step.paths?.map((current) => (current.id === path ? { ...current, label } : current)) };
}

/*
 * withPathChange changes one of a step's paths, such as its condition.
 */
export function withPathChange(step: Step, path: string, change: Partial<Path>): Step {
  return {
    ...step,
    paths: step.paths?.map((current) => (current.id === path ? { ...current, ...change } : current)),
  };
}

/*
 * withNewPath adds a path people name after the step's other named
 * paths, ahead of its fixed ones.
 */
export function withNewPath(step: Step, label: string): Step {
  const paths = step.paths ?? [];
  const named = paths.filter((path) => !path.outcome);
  const fixed = paths.filter((path) => path.outcome);

  return { ...step, paths: [...named, { id: newId("p_"), label, steps: [] }, ...fixed] };
}

/*
 * withoutPath removes a path and every step down it.
 */
export function withoutPath(step: Step, path: string): Step {
  return { ...step, paths: step.paths?.filter((current) => current.id !== path) };
}

/*
 * useSet makes the function an editor changes some of its step's
 * settings with.
 */
export function useSet(onEdit: (edit: (step: Step) => Step) => void) {
  return (changes: Settings) => onEdit((current) => withSettings(current, changes));
}

/*
 * withFailurePath gives a call its own path for when it fails, or takes
 * it away with every step down it.
 */
export function withFailurePath(step: Step, on: boolean): Step {
  const others = (step.paths ?? []).filter((path) => path.outcome !== "failure");
  const paths = on ? [...others, { id: newId("p_"), outcome: "failure", steps: [] }] : others;

  return { ...step, paths: paths.length > 0 ? paths : undefined };
}

/*
 * targetKinds are where a step saves what it finds: a variable of the
 * run, which later steps read as {{run.vars.name}}, or an attribute of
 * the contact, kept after the run ends.
 */
export const targetKinds: Record<string, string> = {
  "run.vars.": "Run variable",
  "contact.custom.": "Contact attribute",
};

/*
 * splitTarget reads a saved target as its kind and its name, both empty
 * when unset.
 */
export function splitTarget(target: string): [kind: string, name: string] {
  const kind = Object.keys(targetKinds).find((prefix) => target.startsWith(prefix));

  return kind ? [kind, target.slice(kind.length)] : ["", ""];
}
