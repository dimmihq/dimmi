/*
 * A routine as the builder edits it: filled from a template for a new
 * one or from the saved routine, and turned back into what the API
 * takes. A routine is its name, what Operator should do in plain words,
 * when it runs, what it reads and who hears about it; a template only
 * fills the first three.
 */
import type { NotifyPolicy, Routine, RoutineDelivery, RoutineInput, RoutineTemplate } from "@/lib/api/routines";

import { cronOf, instantOf, type Timing, timingOf } from "./timing";

/* GitHubPick is the repositories a routine reads; none picked reads none. */
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
  prompt: string;
  timing: Timing;
  /* Empty reads every inbox. */
  inboxIds: string[];
  github: GitHubPick;
  notifyPolicy: NotifyPolicy;
  notify: Notify;
}

const noGitHub: GitHubPick = { installationId: "", repositories: [], baseBranch: "" };

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
    prompt: t.prompt,
    timing: timingOf(t.cadence, t.cron_expression, null, timezone),
    inboxIds: [],
    github: { ...noGitHub },
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
    prompt: r.prompt,
    timing: timingOf(r.cadence, r.cron_expression, r.run_at, r.timezone),
    inboxIds: [...r.inbox_ids],
    github: gh
      ? { installationId: gh.installation_id, repositories: [...gh.repositories], baseBranch: gh.base_branch ?? "" }
      : { ...noGitHub },
    notifyPolicy: r.notify_policy,
    notify: notifyOf(r.delivery),
  };
}

/*
 * inputOf is the draft as the API takes it, whole. Repositories are sent
 * only once an account and at least one of them are picked.
 */
export function inputOf(d: RoutineDraft): RoutineInput {
  const t = d.timing;
  const input: RoutineInput = {
    name: d.name.trim(),
    prompt: d.prompt.trim(),
    cadence: t.kind,
    timezone: t.timezone,
    inbox_ids: d.inboxIds,
    notify_policy: d.notifyPolicy,
    delivery: deliveryOf(d.notify),
    scope: {},
  };
  if (t.kind === "once") {
    input.run_at = t.runAt ? instantOf(t.runAt, t.timezone) : undefined;
  } else {
    input.cron_expression = cronOf(t);
  }
  if (d.github.installationId && d.github.repositories.length > 0) {
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
    out.push("Say what Operator should do.");
  }
  if (d.timing.kind === "once" && !d.timing.runAt) {
    out.push("Pick when the routine runs.");
  }
  if (d.timing.kind === "cron" && !d.timing.cron.trim()) {
    out.push("Write the cron expression.");
  }
  out.push(...notifyProblems(d.notify));
  return out;
}

/*
 * sameDraft reports whether two drafts save the same routine.
 */
export function sameDraft(a: RoutineDraft, b: RoutineDraft): boolean {
  return JSON.stringify(inputOf(a)) === JSON.stringify(inputOf(b));
}
