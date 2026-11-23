/*
 * A routine as the builder edits it: filled from a template for a new
 * one or from the saved routine, and turned back into what the API
 * takes. Money is typed in dollars and stored in micro-dollars; an
 * empty budget means the routine shares the workspace's.
 */
import type {
  NotifyPolicy,
  Routine,
  RoutineDelivery,
  RoutineInput,
  RoutineTemplate,
  RoutineToolGroup,
} from "@/lib/api/routines";

import { cronOf, instantOf, type Timing, timingOf } from "./timing";

export interface GitHubPick {
  installationId: string;
  repositories: string[];
  baseBranch: string;
}

export type NotifyChannel = "in_app" | "email" | "slack";

/*
 * Notify is who hears about a run and how: the channels, the teammates
 * told in dimmi and by email, and the Slack channels posted to.
 */
export interface Notify {
  channels: NotifyChannel[];
  people: string[];
  slack: string[];
}

export interface RoutineDraft {
  templateKey: string | null;
  name: string;
  description: string;
  prompt: string;
  timing: Timing;
  toolGroups: RoutineToolGroup[];
  permissions: string[];
  /* Empty reads every inbox. */
  inboxIds: string[];
  github: GitHubPick | null;
  /* Empty uses Operator's own model. */
  model: string;
  budgetDollars: string;
  notifyPolicy: NotifyPolicy;
  notify: Notify;
}

/*
 * notifyOf reads a saved delivery as channels, people and Slack channels.
 */
export function notifyOf(d: RoutineDelivery): Notify {
  const channels: NotifyChannel[] = [];
  if (d.in_app_member_ids.length > 0) {
    channels.push("in_app");
  }
  if (d.email_member_ids.length > 0) {
    channels.push("email");
  }
  if (d.slack_destination_ids.length > 0) {
    channels.push("slack");
  }
  return {
    channels,
    people: [...new Set([...d.in_app_member_ids, ...d.email_member_ids])],
    slack: [...d.slack_destination_ids],
  };
}

/*
 * deliveryOf is who hears about a run, as the API takes it.
 */
export function deliveryOf(n: Notify): RoutineDelivery {
  const on = (c: NotifyChannel) => n.channels.includes(c);
  return {
    in_app_member_ids: on("in_app") ? n.people : [],
    email_member_ids: on("email") ? n.people : [],
    slack_destination_ids: on("slack") ? n.slack : [],
  };
}

/*
 * draftFromTemplate is a new routine as the template fills it, told to
 * the teammate creating it in the app and by email.
 */
export function draftFromTemplate(t: RoutineTemplate, timezone: string, me: string): RoutineDraft {
  return {
    templateKey: t.key === "tpl_custom" ? null : t.key,
    name: t.key === "tpl_custom" ? "" : t.name,
    description: t.key === "tpl_custom" ? "" : t.description,
    prompt: t.prompt,
    timing: timingOf(t.cadence, t.cron_expression, null, timezone),
    toolGroups: [...t.tool_groups],
    permissions: [...t.permissions],
    inboxIds: [],
    github: t.tool_groups.includes("github") ? { installationId: "", repositories: [], baseBranch: "" } : null,
    model: "",
    budgetDollars: "",
    notifyPolicy: t.notify_policy,
    notify: { channels: ["in_app", "email"], people: [me], slack: [] },
  };
}

/*
 * draftFromRoutine is a saved routine as the builder shows it.
 */
export function draftFromRoutine(r: Routine): RoutineDraft {
  const gh = r.scope.github;
  return {
    templateKey: r.template_key,
    name: r.name,
    description: r.description,
    prompt: r.prompt,
    timing: timingOf(r.cadence, r.cron_expression, r.run_at, r.timezone),
    toolGroups: [...r.tool_groups],
    permissions: [...r.permissions],
    inboxIds: [...r.inbox_ids],
    github: gh
      ? { installationId: gh.installation_id, repositories: [...gh.repositories], baseBranch: gh.base_branch ?? "" }
      : null,
    model: r.model ?? "",
    budgetDollars: r.monthly_budget_micros === null ? "" : String(r.monthly_budget_micros / 1_000_000),
    notifyPolicy: r.notify_policy,
    notify: notifyOf(r.delivery),
  };
}

/*
 * budgetMicros reads the typed budget, null when empty or not a positive
 * amount.
 */
export function budgetMicros(dollars: string): number | null {
  const value = Number(dollars.trim().replace(/^\$/, ""));
  if (dollars.trim() === "" || !Number.isFinite(value) || value <= 0) {
    return null;
  }
  return Math.round(value * 1_000_000);
}

/*
 * inputOf is the draft as the API takes it, whole.
 */
export function inputOf(d: RoutineDraft): RoutineInput {
  const t = d.timing;
  const input: RoutineInput = {
    name: d.name.trim(),
    description: d.description.trim(),
    prompt: d.prompt.trim(),
    cadence: t.kind,
    timezone: t.timezone,
    tool_groups: d.toolGroups,
    permissions: d.permissions,
    inbox_ids: d.inboxIds,
    model: d.model,
    notify_policy: d.notifyPolicy,
    delivery: deliveryOf(d.notify),
  };
  if (t.kind === "once") {
    input.run_at = t.runAt ? instantOf(t.runAt, t.timezone) : undefined;
  } else {
    input.cron_expression = cronOf(t);
  }
  const budget = budgetMicros(d.budgetDollars);
  if (budget === null) {
    input.clear_budget = true;
  } else {
    input.monthly_budget_micros = budget;
  }
  if (d.github && d.toolGroups.includes("github")) {
    input.scope = {
      github: {
        installation_id: d.github.installationId,
        repositories: d.github.repositories,
        base_branch: d.github.baseBranch.trim() || undefined,
      },
    };
  }
  if (d.templateKey) {
    input.template_key = d.templateKey;
  }
  return input;
}

/*
 * createInputOf is the draft as creating a routine takes it: a new
 * routine has no budget to clear.
 */
export function createInputOf(d: RoutineDraft): RoutineInput {
  const { clear_budget: _, ...input } = inputOf(d);
  return input;
}

/*
 * notifyProblems lists what keeps who hears about a run from being whole.
 */
function notifyProblems(n: Notify): string[] {
  const out: string[] = [];
  if ((n.channels.includes("in_app") || n.channels.includes("email")) && n.people.length === 0) {
    out.push("Pick who hears about each run.");
  }
  if (n.channels.includes("slack") && n.slack.length === 0) {
    out.push("Pick the Slack channels to post to.");
  }
  return out;
}

/*
 * problems lists what keeps the draft from saving, as the builder says
 * it before the server has to.
 */
export function problems(d: RoutineDraft): string[] {
  const out: string[] = [];
  if (!d.name.trim()) {
    out.push("Give the routine a name.");
  }
  if (!d.prompt.trim()) {
    out.push("Tell Operator what to do.");
  }
  if (d.timing.kind === "once" && !d.timing.runAt) {
    out.push("Pick when the routine runs.");
  }
  if (d.timing.kind === "cron" && !d.timing.cron.trim()) {
    out.push("Write the cron expression.");
  }
  if (d.toolGroups.includes("github") && (!d.github?.installationId || d.github.repositories.length === 0)) {
    out.push("Pick the GitHub repositories the routine reads.");
  }
  out.push(...notifyProblems(d.notify));
  if (d.budgetDollars.trim() !== "" && budgetMicros(d.budgetDollars) === null) {
    out.push("A monthly budget is an amount above zero.");
  }
  return out;
}

/*
 * sameDraft reports whether two drafts save the same routine.
 */
export function sameDraft(a: RoutineDraft, b: RoutineDraft): boolean {
  return JSON.stringify(inputOf(a)) === JSON.stringify(inputOf(b));
}
