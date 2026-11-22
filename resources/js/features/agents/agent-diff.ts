/*
 * What differs between two copies of an agent, field by field, in the
 * words the Build tab uses. Guidance compares line by line, so a long
 * guidance shows only the lines that were added or taken out; every
 * other field shows its value before and after.
 */
import type { AgentDefinition, AgentScope, KnowledgeScope } from "@/lib/api/agents";

export type LineMark = "same" | "added" | "removed";

export interface GuidanceLine {
  mark: LineMark;
  text: string;
}

export interface FieldChange {
  key: string;
  label: string;
  before: string;
  after: string;
  /* Set for guidance: its lines, marked. */
  lines?: GuidanceLine[];
}

/* Namer reads an id as its name, undefined when it does not know it. */
export type Namer = (id: string) => string | undefined;

const words: Record<string, string> = {
  friendly: "Friendly",
  professional: "Professional",
  playful: "Playful",
  always: "Always",
  business_hours: "During business hours",
  outside_business_hours: "Outside business hours",
  skip_when_human_active: "Steps back when a teammate replies",
  always_answer: "Always answers",
  offer_handoff: "Offers the team",
  handoff: "Hands to the team",
  say_unknown: "Says it does not know",
  full: "Full",
  basic: "Basic",
  none: "None",
};

const word = (v: string) => words[v] ?? v;
const yes = (v: boolean) => (v ? "On" : "Off");

function list(values: string[], name: Namer = (id) => id): string {
  if (values.length === 0) {
    return "None";
  }
  const known = values.flatMap((id) => name(id) ?? []);
  const others = values.length - known.length;
  if (others === 0) {
    return known.join(", ");
  }
  const rest = others === 1 ? "1 other" : `${others} others`;

  return known.length === 0 ? `${others} picked` : `${known.join(", ")} and ${rest}`;
}

function scope(s: AgentScope, name: Namer): string {
  return s.mode === "all" ? "All" : list(s.ids, name);
}

function knowledge(s: KnowledgeScope, name: Namer): string {
  return s.mode === "all" ? "All" : list([...s.help_center_ids, ...s.source_ids], name);
}

/* commonRuns[i][j] is how many lines a[i:] and b[j:] share, in order. */
function commonRuns(a: string[], b: string[]): number[][] {
  const run: number[][] = Array.from({ length: a.length + 1 }, () => new Array<number>(b.length + 1).fill(0));
  for (let i = a.length - 1; i >= 0; i--) {
    for (let j = b.length - 1; j >= 0; j--) {
      run[i][j] = a[i] === b[j] ? run[i + 1][j + 1] + 1 : Math.max(run[i + 1][j], run[i][j + 1]);
    }
  }

  return run;
}

/*
 * guidanceLines marks the lines of after against before: kept, added
 * or taken out, in reading order, by their longest common run.
 */
export function guidanceLines(before: string, after: string): GuidanceLine[] {
  const a = before.split("\n");
  const b = after.split("\n");
  const run = commonRuns(a, b);
  const out: GuidanceLine[] = [];
  let [i, j] = [0, 0];
  while (i < a.length || j < b.length) {
    if (i < a.length && j < b.length && a[i] === b[j]) {
      out.push({ mark: "same", text: a[i] });
      i++;
      j++;
    } else if (i < a.length && (j >= b.length || run[i + 1][j] >= run[i][j + 1])) {
      out.push({ mark: "removed", text: a[i] });
      i++;
    } else {
      out.push({ mark: "added", text: b[j] });
      j++;
    }
  }

  return out;
}

/*
 * agentChanges lists the fields that differ from before to after, in
 * the order the Build tab shows them.
 */
export function agentChanges(before: AgentDefinition, after: AgentDefinition, name: Namer): FieldChange[] {
  const fields: [string, string, (d: AgentDefinition) => string][] = [
    ["name", "Name", (d) => d.name],
    ["tone", "Tone", (d) => word(d.tone)],
    ["guidance", "Guidance", (d) => d.guidance],
    [
      "placement",
      "Where it answers",
      (d) =>
        list([...d.placement.channel_connection_ids, ...d.placement.channel_room_ids, ...d.placement.inbox_ids], name),
    ],
    ["enabled_channels", "Channels", (d) => list(d.enabled_channels)],
    ["knowledge_scope", "Knowledge", (d) => knowledge(d.knowledge_scope, name)],
    ["procedure_scope", "Procedures", (d) => scope(d.procedure_scope, name)],
    ["action_scope", "Actions", (d) => scope(d.action_scope, name)],
    ["transfer_agent_ids", "Hands over to", (d) => list(d.transfer_agent_ids, name)],
    ["schedule", "When it answers", (d) => word(d.schedule)],
    ["assigned_behavior", "When a teammate is on it", (d) => word(d.assigned_behavior)],
    ["unknown_behavior", "When it does not know", (d) => word(d.unknown_behavior)],
    ["max_ai_replies", "Most replies", (d) => String(d.max_ai_replies)],
    ["handoff_team_id", "Hands to team", (d) => (d.handoff_team_id ? (name(d.handoff_team_id) ?? "A team") : "None")],
    ["languages", "Languages", (d) => list(d.languages)],
    ["answer_model", "Model", (d) => d.answer_model ?? "Default"],
    ["require_citations", "Cites sources", (d) => yes(d.require_citations)],
    ["pii_redaction", "Hides personal data", (d) => yes(d.pii_redaction)],
    ["link_allowlist", "Links it may share", (d) => list(d.link_allowlist)],
  ];

  return fields.flatMap(([key, label, read]) => {
    const [was, now] = [read(before), read(after)];
    if (was === now) {
      return [];
    }
    const change: FieldChange = { key, label, before: was, after: now };
    if (key === "guidance") {
      change.lines = guidanceLines(was, now);
    }
    return [change];
  });
}
