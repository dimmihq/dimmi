/*
 * The series as the builder edits it: steps and the paths between them,
 * kept in the same JSON the server stores. Adding a step hangs it off a
 * path and carries on with whatever that path led to; removing one joins
 * the step before it to the step after it. Step and path ids are made
 * once and never change, so people already on the journey follow edits.
 */
import type { NodeKind, NodeSettings, SeriesEdge, SeriesNode, SeriesSpec, SeriesSubject } from "@/lib/api/series";

export const messageKinds: NodeKind[] = ["email", "chat", "post", "slack"];

export function isMessage(kind: NodeKind): boolean {
  return messageKinds.includes(kind);
}

/*
 * pathsOf lists the paths a step can leave by, with the label the
 * canvas writes on each. The first is the one a new step follows.
 */
export function pathsOf(node: SeriesNode): { id: string; label: string }[] {
  const s = node.settings ?? {};
  switch (node.kind) {
    case "exit":
      return [];
    case "wait_for":
      return [
        { id: "matched", label: "It happened" },
        { id: "timed_out", label: "Time's up" },
      ];
    case "condition":
      return [...(s.paths ?? []).map((p) => ({ id: p.id, label: p.label })), { id: "else", label: "Everyone else" }];
    case "split":
      return (s.variants ?? []).map((v) => ({ id: v.id, label: `${v.label} · ${v.weight}%` }));
    case "ai_path":
      return [...(s.paths ?? []).map((p) => ({ id: p.id, label: p.label })), { id: "fallback", label: "Fallback" }];
    default:
      return [{ id: "next", label: "" }];
  }
}

/*
 * optionalPaths are paths a step has that the canvas only draws once
 * something hangs off them: a message's could-not-send path, a wait's
 * no-date path.
 */
export function optionalPaths(node: SeriesNode): { id: string; label: string }[] {
  if (isMessage(node.kind)) {
    return [{ id: "failed", label: "Could not send" }];
  }
  if (node.kind === "wait" && node.settings?.mode === "attribute") {
    return [{ id: "no_date", label: "No date" }];
  }

  return [];
}

export function targetOf(spec: SeriesSpec, from: string, path: string): string | undefined {
  return spec.edges.find((e) => e.from === from && (e.path || "next") === path)?.to;
}

/*
 * newID makes a step or path id no other one in the series has.
 */
export function newID(spec: SeriesSpec, prefix: string): string {
  const taken = new Set(spec.nodes.map((n) => n.id));
  for (const n of spec.nodes) {
    for (const p of [...(n.settings?.paths ?? []), ...(n.settings?.variants ?? [])]) {
      taken.add(p.id);
    }
  }
  let i = 1;
  while (taken.has(`${prefix}-${i}`)) {
    i++;
  }

  return `${prefix}-${i}`;
}

/*
 * defaults is what a new step of each kind starts with.
 */
export function defaults(kind: NodeKind, spec: SeriesSpec): NodeSettings {
  switch (kind) {
    case "email":
      return { subject: "", body: 'Hi {{ contact.first_name | default: "there" }},\n\n' };
    case "chat":
    case "post":
    case "slack":
      return { body: "" };
    case "wait":
      return { mode: "duration", amount: 2, unit: "days" };
    case "wait_for":
      return { event: "", amount: 3, unit: "days" };
    case "condition":
      return { paths: [{ id: newID(spec, "yes"), label: "Matches", rules: undefined }] };
    case "split": {
      const a = newID(spec, "a");
      return {
        variants: [
          { id: a, label: "A", weight: 50 },
          { id: `${a}-b`, label: "B", weight: 50 },
        ],
      };
    }
    case "ai_path":
      return {
        prompt: "",
        paths: [
          { id: newID(spec, "fit"), label: "Good fit", description: "" },
          { id: `${newID(spec, "fit")}-other`, label: "Needs help", description: "" },
        ],
      };
    case "webhook":
      return { url: "https://" };
    default:
      return {};
  }
}

/*
 * addStep hangs a new step off a path of a step. What the path led to
 * moves under the new step's first path, so nothing is lost.
 */
export function addStep(spec: SeriesSpec, from: string, path: string, kind: NodeKind): [SeriesSpec, string] {
  const id = newID(spec, kind.replace("_", "-"));
  const node: SeriesNode = { id, kind, settings: defaults(kind, spec) };
  const before = targetOf(spec, from, path);
  const edges: SeriesEdge[] = spec.edges.filter((e) => !(e.from === from && (e.path || "next") === path));
  edges.push({ from, path, to: id });

  const first = pathsOf(node)[0];
  if (before && first) {
    edges.push({ from: id, path: first.id, to: before });
  }

  return [{ ...spec, nodes: [...spec.nodes, node], edges }, id];
}

/*
 * removeStep takes a step out, joining the path that led to it to the
 * step its first path led to, and drops whatever no longer connects.
 */
export function removeStep(spec: SeriesSpec, id: string): SeriesSpec {
  const node = spec.nodes.find((n) => n.id === id);
  if (!node || node.kind === "entry") {
    return spec;
  }
  const first = pathsOf(node)[0];
  const after = first ? targetOf(spec, id, first.id) : undefined;
  const edges: SeriesEdge[] = [];
  for (const e of spec.edges) {
    if (e.from === id) {
      continue;
    }
    if (e.to === id) {
      if (after) {
        edges.push({ ...e, to: after });
      }
      continue;
    }
    edges.push(e);
  }

  return prune({ ...spec, nodes: spec.nodes.filter((n) => n.id !== id), edges });
}

/*
 * prune drops the steps no path reaches from the entry any more.
 */
export function prune(spec: SeriesSpec): SeriesSpec {
  const seen = new Set<string>();
  const queue = ["entry"];
  while (queue.length > 0) {
    const at = queue.shift() as string;
    if (seen.has(at)) {
      continue;
    }
    seen.add(at);
    for (const e of spec.edges) {
      if (e.from === at) {
        queue.push(e.to);
      }
    }
  }

  return {
    ...spec,
    nodes: spec.nodes.filter((n) => seen.has(n.id)),
    edges: spec.edges.filter((e) => seen.has(e.from) && seen.has(e.to)),
  };
}

/*
 * setSettings replaces a step's settings, dropping paths that lead from
 * ids the new settings no longer name.
 */
export function setSettings(spec: SeriesSpec, id: string, settings: NodeSettings): SeriesSpec {
  const nodes = spec.nodes.map((n) => (n.id === id ? { ...n, settings } : n));
  const node = nodes.find((n) => n.id === id);
  if (!node) {
    return spec;
  }
  const allowed = new Set([...pathsOf(node), ...optionalPaths(node)].map((p) => p.id));
  const edges = spec.edges.filter((e) => e.from !== id || allowed.has(e.path || "next"));

  return prune({ ...spec, nodes, edges });
}

const units: Record<string, [string, string]> = {
  minutes: ["minute", "minutes"],
  hours: ["hour", "hours"],
  days: ["day", "days"],
  weeks: ["week", "weeks"],
};

export function duration(amount = 0, unit = "days"): string {
  const [one, many] = units[unit] ?? ["day", "days"];
  return `${amount} ${amount === 1 ? one : many}`;
}

/*
 * summary says in one line what a step does, for its card.
 */
export function summary(node: SeriesNode, spec: SeriesSpec): string {
  if (node.kind === "entry") {
    return entrySummary(spec);
  }

  return (summaries[node.kind] ?? (() => "Leave the series"))(node.settings ?? {});
}

function tags(verb: string, s: NodeSettings): string {
  const n = s.tag_ids?.length ?? 0;
  return `${verb} ${n} tag${n === 1 ? "" : "s"}`;
}

function waitSummary(s: NodeSettings): string {
  if (s.mode === "time") {
    return `Wait until ${s.at ?? "09:00"} their time`;
  }
  if (s.mode === "attribute") {
    return `Wait until ${s.attribute || "a date"}${offset(s.offset_days ?? 0)}`;
  }

  return `Wait ${duration(s.amount, s.unit)}`;
}

const message = (s: NodeSettings) => firstLine(s.subject || s.body) || "Write the message";

const summaries: Partial<Record<NodeKind, (s: NodeSettings) => string>> = {
  email: (s) => s.subject || "Write the subject",
  chat: message,
  post: message,
  slack: message,
  wait: waitSummary,
  wait_for: (s) =>
    `Wait up to ${duration(s.amount, s.unit)} for ${s.event || (s.engagement ? s.engagement.action : "a match")}`,
  condition: (s) => `Check ${(s.paths ?? []).map((p) => p.label).join(", ") || "who matches"}`,
  split: (s) => `Split ${(s.variants ?? []).map((v) => `${v.label} ${v.weight}%`).join(" · ")}`,
  ai_path: (s) => firstLine(s.prompt) || "Let the AI pick a path",
  tag: (s) => tags("Add", s),
  untag: (s) => tags("Remove", s),
  set_attribute: (s) => (s.key ? `Set ${s.key}` : "Set an attribute"),
  topic: (s) => (s.subscribed ? "Subscribe to a topic" : "Unsubscribe from a topic"),
  webhook: (s) => s.url || "Post to a webhook",
};

function entrySummary(spec: SeriesSpec): string {
  if (spec.entry.event) {
    return `Does ${spec.entry.event}`;
  }
  if (spec.entry.audience_id || spec.entry.rules) {
    return spec.entry.include_existing ? "Everyone who matches the rules" : "People who start matching the rules";
  }

  return "Added by hand or through the API";
}

function offset(days: number): string {
  if (days === 0) {
    return "";
  }

  return days > 0 ? ` + ${days} days` : ` − ${-days} days`;
}

function firstLine(text?: string): string {
  return (text ?? "").split("\n").find((line) => line.trim() !== "") ?? "";
}

/*
 * blankSpec is a new series: an entry and nothing after it.
 */
export function blankSpec(): SeriesSpec {
  return {
    entry: { reentry: { kind: "once" } },
    exit: {},
    nodes: [{ id: "entry", kind: "entry" }],
    edges: [],
  };
}

/*
 * templates are the series a new one can start from, for contacts or for
 * companies.
 */
export const templates: { id: string; name: string; subject: SeriesSubject; description: string }[] = [
  { id: "blank", name: "Start from scratch", subject: "contact", description: "An entry and nothing after it." },
  {
    id: "onboarding",
    name: "Onboarding",
    subject: "contact",
    description: "Welcome email, wait two days, nudge the ones who did not set up.",
  },
  {
    id: "trial",
    name: "Trial ending",
    subject: "contact",
    description: "Remind people three days before their trial ends, then on the day.",
  },
  {
    id: "renewal",
    name: "Account renewal",
    subject: "company",
    description: "Email the admins of each account and post in its Slack channel before renewal.",
  },
];

export function templateSpec(id: string): SeriesSpec {
  const spec = blankSpec();
  const step = (node: SeriesNode, from: string, path = "next") => {
    spec.nodes.push(node);
    spec.edges.push({ from, path, to: node.id });
  };
  if (id === "onboarding") {
    spec.goal = { event: "messenger_installed", days: 14 };
    spec.exit = { on_goal: true };
    step(
      { id: "welcome", kind: "email", settings: { subject: "Welcome aboard", body: defaults("email", spec).body } },
      "entry",
    );
    step({ id: "wait-2d", kind: "wait", settings: { mode: "duration", amount: 2, unit: "days" } }, "welcome");
    step(
      {
        id: "nudge",
        kind: "email",
        settings: { subject: "Your messenger is two minutes away", body: defaults("email", spec).body },
      },
      "wait-2d",
    );
  }
  if (id === "trial") {
    step(
      { id: "before", kind: "wait", settings: { mode: "attribute", attribute: "trial_ends_at", offset_days: -3 } },
      "entry",
    );
    step(
      {
        id: "remind",
        kind: "email",
        settings: { subject: "Your trial ends in 3 days", body: defaults("email", spec).body },
      },
      "before",
    );
  }
  if (id === "renewal") {
    step(
      {
        id: "admins",
        kind: "email",
        settings: { subject: "Your renewal is coming up", body: defaults("email", spec).body },
      },
      "entry",
    );
    step(
      { id: "channel", kind: "slack", settings: { body: "Your renewal is coming up. Questions? Reply here." } },
      "admins",
    );
  }

  return spec;
}
