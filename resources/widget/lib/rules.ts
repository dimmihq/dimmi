/*
 * The messenger's rules as a browser sees them. The server decides every
 * rule with what it knows and sends a yes, a no, or the page and device
 * conditions still to check; nothing else about the rule ever arrives.
 * Those conditions are checked here on every page, with no request.
 * Text compares without regard to case; a page path ignores a trailing
 * slash; "matches" takes * for any characters but a slash and ** for
 * any characters. An operator this messenger does not know fails the
 * condition and says so once, so an older messenger never shows what a
 * newer rule would hide by mistake.
 */
import { report } from "./report";

export interface Condition {
  field: string;
  op: string;
  value?: unknown;
}

export type Rule = { all: Rule[] } | { any: Rule[] } | { not: Rule } | Condition;

/*
 * Decision is what the server decided: true, false, or the rule left.
 */
export type Decision = boolean | Rule;

export interface Surface {
  launcher?: Decision;
  start?: boolean;
  tabs?: Record<string, Decision>;
}

/*
 * Visit is what the page and device say about the visitor right now.
 */
export interface Visit {
  url: string;
  referrer?: string;
  width?: number;
  language?: string;
}

const warned = new Set<string>();

function fold(s: unknown): string {
  return String(s ?? "")
    .normalize("NFC")
    .toLowerCase();
}

function device(width: number | undefined): string | undefined {
  if (width === undefined) {
    return undefined;
  }
  if (width < 640) {
    return "mobile";
  }

  return width < 1024 ? "tablet" : "desktop";
}

/*
 * facts reads the fields a rule may ask about from a visit.
 */
export function facts(visit: Visit): Record<string, string | undefined> {
  let url: URL | undefined;
  try {
    url = new URL(visit.url);
  } catch {
    url = undefined;
  }
  const path = url ? url.pathname.replace(/(.)\/$/, "$1") : undefined;

  return {
    "page.url": url ? `${url.origin}${url.pathname}${url.search}` : undefined,
    "page.path": path,
    "page.query": url ? url.search.replace(/^\?/, "") : undefined,
    "page.host": url?.host,
    "page.referrer": visit.referrer || undefined,
    "visitor.device": device(visit.width),
    "visitor.locale": visit.language,
  };
}

/*
 * glob turns a "matches" pattern into a regular expression.
 */
function glob(pattern: string): RegExp {
  const source = pattern
    .split("**")
    .map((part) =>
      part
        .split("*")
        .map((text) => text.replace(/[.+?^${}()|[\]\\]/g, "\\$&"))
        .join("[^/]*"),
    )
    .join(".*");

  return new RegExp(`^${source}$`, "i");
}

function list(value: unknown): string[] {
  return Array.isArray(value) ? value.map(fold) : [fold(value)];
}

/*
 * compare decides one condition with the value the visit gives.
 */
function compare(c: Condition, actual: string | undefined): boolean {
  if (c.op === "exists") {
    return actual !== undefined;
  }
  if (c.op === "not_exists") {
    return actual === undefined;
  }
  const negative = ["neq", "not_in", "not_contains", "not_contains_any"].includes(c.op);
  if (actual === undefined) {
    return negative;
  }
  const a = fold(actual);
  const v = fold(c.value);
  const values = list(c.value);
  const ops: Record<string, () => boolean> = {
    eq: () => a === v || (c.field === "page.path" && a === v.replace(/(.)\/$/, "$1")),
    neq: () => a !== v,
    in: () => values.includes(a),
    not_in: () => !values.includes(a),
    contains: () => a.includes(v),
    not_contains: () => !a.includes(v),
    starts_with: () => a.startsWith(v),
    ends_with: () => a.endsWith(v),
    contains_any: () => values.some((w) => a.includes(w)),
    not_contains_any: () => !values.some((w) => a.includes(w)),
    matches: () => glob(String(c.value ?? "")).test(actual),
  };
  const op = ops[c.op];
  if (!op) {
    if (!warned.has(c.op)) {
      warned.add(c.op);
      report("error", "rule_unknown", `this messenger does not know the rule "${c.op}"; update it.`);
    }
    return false;
  }

  return op();
}

/*
 * matches decides a rule for a visit.
 */
export function matches(rule: Rule, visit: Visit): boolean {
  if ("all" in rule) {
    return rule.all.every((r) => matches(r, visit));
  }
  if ("any" in rule) {
    return rule.any.some((r) => matches(r, visit));
  }
  if ("not" in rule) {
    return !matches(rule.not, visit);
  }

  return compare(rule, facts(visit)[rule.field]);
}

/*
 * decide turns a decision into yes or no for this visit. No decision
 * means no rule, which means yes.
 */
export function decide(decision: Decision | undefined, visit: Visit): boolean {
  if (decision === undefined || typeof decision === "boolean") {
    return decision ?? true;
  }

  return matches(decision, visit);
}
