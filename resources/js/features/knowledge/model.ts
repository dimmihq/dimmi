/*
 * How knowledge sources read on screen, in plain words: what a source
 * feeds, how it stands, when it last and next syncs, what a run did and
 * why a document was set aside. The connect form's values come from a
 * driver's schema: defaults filled in, secrets kept apart, required
 * fields checked, so any driver the server offers gets a working form.
 */
import type { ChipColor } from "@/components/base/badges/chip";
import type {
  DocumentStatus,
  KnowledgeSource,
  SchemaProperty,
  SourceSchedule,
  SourceSchema,
  SourceTarget,
  SyncRun,
  TreeNode,
} from "@/lib/api/knowledge-sources";

export const targetLabels: Record<SourceTarget, string> = {
  agent: "Agent",
  operator: "Operator",
  help_center: "Help center",
};

export const scheduleLabels: Record<SourceSchedule, string> = {
  manual: "Only when I sync",
  hourly: "Every hour",
  daily: "Every day",
  weekly: "Every week",
};

/*
 * SourceState is how a source stands, as a word and a dot's colour.
 */
export interface SourceState {
  label: string;
  tone: "green" | "orange" | "red" | "gray" | "blue";
}

export function sourceState(s: KnowledgeSource): SourceState {
  if (s.status === "syncing" || s.status === "queued") {
    return { label: "Syncing", tone: "blue" };
  }
  if (s.status === "error") {
    return { label: "Failing", tone: "red" };
  }
  if (s.last_run?.status === "partial") {
    return { label: "Needs attention", tone: "orange" };
  }
  if (!s.last_synced_at) {
    return { label: "Not synced yet", tone: "gray" };
  }

  return { label: "Live", tone: "green" };
}

/*
 * broken reports whether a source needs someone: its last sync failed
 * or half failed.
 */
export function broken(s: KnowledgeSource): boolean {
  return s.status === "error" || s.last_run?.status === "partial";
}

/*
 * ago says how long ago a moment was, in the largest unit that fits.
 */
export function ago(at: string | null, now = Date.now()): string {
  if (!at) {
    return "Never";
  }
  const seconds = Math.round((now - Date.parse(at)) / 1000);
  if (seconds < 60) {
    return "Just now";
  }

  return `${span(seconds)} ago`;
}

/*
 * inTime says how long until a moment.
 */
export function inTime(at: string | null, now = Date.now()): string {
  if (!at) {
    return "Not scheduled";
  }
  const seconds = Math.round((Date.parse(at) - now) / 1000);
  if (seconds <= 60) {
    return "Due now";
  }

  return `In ${span(seconds)}`;
}

function span(seconds: number): string {
  const units: [number, string][] = [
    [86400, "day"],
    [3600, "hour"],
    [60, "minute"],
  ];
  for (const [size, name] of units) {
    if (seconds >= size) {
      const n = Math.round(seconds / size);
      return `${n} ${name}${n === 1 ? "" : "s"}`;
    }
  }

  return `${seconds} seconds`;
}

/*
 * runSummary says what a run did in one line, its largest counters first.
 */
export function runSummary(run: SyncRun): string {
  const parts: [string, string][] = [
    ["created", "new"],
    ["updated", "updated"],
    ["unchanged", "unchanged"],
    ["removed", "removed"],
    ["excluded", "set aside"],
    ["skipped_js", "need JavaScript"],
    ["failed", "failed"],
  ];
  const said = parts.filter(([key]) => (run.stats[key] ?? 0) > 0).map(([key, word]) => `${run.stats[key]} ${word}`);

  return said.length > 0 ? said.join(" · ") : "Nothing changed";
}

export const runLabels: Record<SyncRun["status"], { label: string; color: ChipColor }> = {
  queued: { label: "Queued", color: "gray" },
  running: { label: "Running", color: "blue" },
  succeeded: { label: "Succeeded", color: "lime" },
  partial: { label: "Partial", color: "orange" },
  failed: { label: "Failed", color: "rose" },
  canceled: { label: "Canceled", color: "gray" },
};

export const documentFilters: { id: DocumentStatus | ""; label: string }[] = [
  { id: "", label: "All" },
  { id: "active", label: "Live" },
  { id: "error", label: "Failed" },
  { id: "excluded", label: "Excluded" },
];

export const documentLabels: Record<DocumentStatus, { label: string; color: ChipColor }> = {
  pending: { label: "Waiting", color: "gray" },
  active: { label: "Live", color: "lime" },
  excluded: { label: "Excluded", color: "neutral" },
  error: { label: "Failed", color: "rose" },
  removed: { label: "Removed", color: "gray" },
};

const exclusionReasons: Record<string, string> = {
  member: "Excluded by a teammate",
  robots: "Blocked by robots.txt",
  pattern: "Left out by your filters",
  js_rendered: "Needs JavaScript to show",
  duplicate: "Same page as another address",
  too_large: "Too large",
  unsupported_type: "Not a page dimmi reads",
  empty: "No text to read",
  draft: "Draft at the source",
};

export function exclusionReason(reason: string | null): string {
  return reason ? (exclusionReasons[reason] ?? reason) : "";
}

/*
 * FormValues are a connect form's values by setting name.
 */
export type FormValues = Record<string, unknown>;

/*
 * initialValues fills a schema's defaults, then the source's stored
 * settings when editing one. Secrets always start empty.
 */
export function initialValues(schema: SourceSchema, stored: Record<string, unknown> = {}): FormValues {
  const out: FormValues = {};
  for (const [name, prop] of Object.entries(schema.properties)) {
    if (prop["x-secret"]) {
      out[name] = "";
      continue;
    }
    out[name] = stored[name] ?? prop.default ?? emptyOf(prop);
  }

  return out;
}

function emptyOf(prop: SchemaProperty): unknown {
  switch (prop.type) {
    case "boolean":
      return false;
    case "array":
      return [];
    case "integer":
      return "";
    default:
      return "";
  }
}

/*
 * missing lists the required settings left empty. A secret already set
 * on the source counts as given.
 */
export function missing(schema: SourceSchema, values: FormValues, secretsSet: string[] = []): string[] {
  return (schema.required ?? []).filter((name) => {
    if (schema.properties[name]?.["x-secret"] && secretsSet.includes(name)) {
      return false;
    }
    const value = values[name];
    return value === undefined || value === null || (typeof value === "string" && value.trim() === "");
  });
}

/*
 * settingsOf turns form values into the settings a request sends:
 * numbers typed as numbers, blank secrets dropped so the stored ones stay.
 */
export function settingsOf(schema: SourceSchema, values: FormValues): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const [name, prop] of Object.entries(schema.properties)) {
    const value = sent(prop, values[name]);
    if (value !== undefined) {
      out[name] = value;
    }
  }

  return out;
}

/*
 * sent is one setting as a request sends it, undefined to leave it out.
 */
function sent(prop: SchemaProperty, value: unknown): unknown {
  const blank = value === undefined || (typeof value === "string" && value.trim() === "");
  if (prop.type === "boolean") {
    return value;
  }
  if (blank) {
    return undefined;
  }
  if (prop.type === "integer") {
    return Number(value);
  }
  if (Array.isArray(value)) {
    return value.map((v) => String(v).trim()).filter((v) => v !== "");
  }

  return typeof value === "string" ? value.trim() : value;
}

/*
 * patterns reads one pattern per line.
 */
export function patterns(text: string): string[] {
  return text
    .split("\n")
    .map((line) => line.trim())
    .filter((line) => line !== "");
}

/*
 * nameFor suggests a source's name from what it connects to.
 */
export function nameFor(typeName: string, values: FormValues): string {
  for (const key of ["start_url", "repository", "subdomain"]) {
    const value = values[key];
    if (typeof value === "string" && value.trim() !== "") {
      return value
        .replace(/^https?:\/\//, "")
        .replace(/\/$/, "")
        .slice(0, 80);
    }
  }

  return typeName;
}

/*
 * hasTree reports whether a kind of source lets the member tick what to
 * read from its own structure.
 */
export function hasTree(kind: { schema: SourceSchema }): boolean {
  return Object.values(kind.schema.properties).some((prop) => prop.format === "x-tree");
}

/*
 * chosen reads the ticked items of a form's values.
 */
export function chosen(values: FormValues): string[] {
  const selected = values.selected;
  return Array.isArray(selected) ? selected.map(String) : [];
}

/*
 * countChosen counts the documents the selection reads, every document
 * when nothing is ticked.
 */
export function countChosen(nodes: TreeNode[], selected: string[], inherited = false): number {
  let count = 0;
  for (const node of nodes) {
    const ticked = inherited || selected.length === 0 || selected.includes(node.id);
    if (node.children.length === 0) {
      count += ticked && node.kind !== "collection" ? 1 : 0;
    } else {
      count += (ticked && node.kind === "page" ? 1 : 0) + countChosen(node.children, selected, ticked);
    }
  }

  return count;
}

/*
 * patternTree reports whether a kind's tree ticks parts of a site, kept
 * as the source's include patterns rather than a setting.
 */
export function patternTree(kind: { schema: SourceSchema }): boolean {
  return Object.values(kind.schema.properties).some((prop) => prop.format === "x-tree" && prop["x-patterns"]);
}

/*
 * setupFields are the settings asked for after the account: everything
 * but the account itself and the tree.
 */
export function setupFields(kind: { schema: SourceSchema }): string[] {
  return Object.entries(kind.schema.properties)
    .filter(([, prop]) => prop.format !== "x-integration" && prop.format !== "x-tree")
    .map(([name]) => name);
}

export type StepId = "account" | "setup" | "content" | "use" | "review";

/*
 * stepsFor lists the steps of adding a kind of source: the account for
 * a connected service, its settings when it has any, what to read when
 * it has a tree, then what it is for and a review. The setup step is
 * named after the first thing it picks, such as "Repository".
 */
export function stepsFor(kind: { schema: SourceSchema; integration?: string }): { id: StepId; label: string }[] {
  const out: { id: StepId; label: string }[] = [];
  if (kind.integration) {
    out.push({ id: "account", label: "Account" });
  }
  const fields = setupFields(kind);
  if (fields.length > 0) {
    const first = kind.schema.properties[fields[0]];
    out.push({ id: "setup", label: first.format === "x-choice" ? (first.title ?? "Settings") : "Settings" });
  }
  if (hasTree(kind)) {
    out.push({ id: "content", label: "Content" });
  }
  out.push({ id: "use", label: "Use" }, { id: "review", label: "Review" });

  return out;
}
