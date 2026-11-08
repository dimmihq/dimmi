/*
 * The rule builder's model. A stored rule is a tree; the builder edits
 * the shape people actually write: who the audience starts from, a list
 * of conditions and groups matched all, any or none, and inside a group
 * a flat list of conditions. A tree deeper than that, written through
 * the API, is shown as it is and left alone.
 */
import type { AudienceField, AudienceRules, Condition, Rule, Visitors } from "@/lib/api/audiences";

export type Match = "all" | "any" | "none";

export interface ConditionItem {
  kind: "condition";
  id: string;
  condition: Condition;
}

export interface GroupItem {
  kind: "group";
  id: string;
  match: Match;
  conditions: ConditionItem[];
}

export type Item = ConditionItem | GroupItem;

/*
 * Form is an audience as the builder edits it.
 */
export interface Form {
  visitors: Visitors;
  match: Match;
  items: Item[];
}

let counter = 0;

/*
 * newId gives a builder row a key that stays the same while it is edited.
 */
export function newId(): string {
  counter += 1;
  return `r${counter}`;
}

function isCondition(rule: Rule): rule is Condition {
  return "field" in rule;
}

/*
 * matchOf reads a group node as all, any or none, nothing for a
 * condition or a "not" the builder cannot show.
 */
function matchOf(rule: Rule): { match: Match; children: Rule[] } | null {
  if ("all" in rule) {
    return { match: "all", children: rule.all };
  }
  if ("any" in rule) {
    return { match: "any", children: rule.any };
  }
  if ("not" in rule && "any" in rule.not) {
    return { match: "none", children: rule.not.any };
  }
  if ("not" in rule && isCondition(rule.not)) {
    return { match: "none", children: [rule.not] };
  }

  return null;
}

function conditionItem(rule: Rule): ConditionItem | null {
  return isCondition(rule) ? { kind: "condition", id: newId(), condition: rule } : null;
}

function itemOf(rule: Rule): Item | null {
  const leaf = conditionItem(rule);
  if (leaf) {
    return leaf;
  }
  const group = matchOf(rule);
  if (!group) {
    return null;
  }
  const conditions = group.children.map(conditionItem);
  if (conditions.some((c) => c === null)) {
    return null;
  }

  return { kind: "group", id: newId(), match: group.match, conditions: conditions as ConditionItem[] };
}

/*
 * toForm reads stored rules into the builder's shape, nothing when the
 * tree is deeper than the builder shows.
 */
export function toForm(rules: AudienceRules | undefined): Form | null {
  const visitors = rules?.visitors ?? "everyone";
  if (!rules?.rules) {
    return { visitors, match: "all", items: [] };
  }
  const leaf = conditionItem(rules.rules);
  if (leaf) {
    return { visitors, match: "all", items: [leaf] };
  }
  const top = matchOf(rules.rules);
  if (!top) {
    return null;
  }
  const items = top.children.map(itemOf);
  if (items.some((item) => item === null)) {
    return null;
  }

  return { visitors, match: top.match, items: items as Item[] };
}

function grouped(match: Match, children: Rule[]): Rule {
  if (match === "none") {
    return { not: { any: children } };
  }

  return match === "all" ? { all: children } : { any: children };
}

/*
 * toRules writes the builder's shape as stored rules. Empty groups are
 * left out, so a half-built rule still reads.
 */
export function toRules(form: Form): AudienceRules {
  const children: Rule[] = [];
  for (const item of form.items) {
    if (item.kind === "condition") {
      children.push(item.condition);
    } else if (item.conditions.length > 0) {
      children.push(
        grouped(
          item.match,
          item.conditions.map((c) => c.condition),
        ),
      );
    }
  }
  const out: AudienceRules = { visitors: form.visitors };
  if (children.length > 0) {
    out.rules = grouped(form.match, children);
  }

  return out;
}

/*
 * The visitor groups, in the builder's words.
 */
export const visitorChoices: [Visitors, string][] = [
  ["everyone", "Everyone"],
  ["identified", "Signed-in users"],
  ["leads", "Leads"],
  ["anonymous", "Visitors"],
  ["anonymous_and_leads", "Visitors and leads"],
];

export const matchWords: Record<Match, string> = { all: "all", any: "any", none: "none" };

const fieldNames: Record<string, string> = {
  "contact.kind": "Person type",
  "contact.email": "Email",
  "contact.email_domain": "Email domain",
  "contact.email_verified": "Email verified",
  "contact.phone": "Phone",
  "contact.name": "Name",
  "contact.external_id": "User id",
  "contact.locale": "Language",
  "contact.timezone": "Time zone",
  "contact.country": "Country",
  "contact.region": "Region",
  "contact.city": "City",
  "contact.created_at": "First added",
  "contact.first_seen_at": "First seen",
  "contact.last_seen_at": "Last seen",
  "contact.has_company": "Has a company",
  "contact.tags": "Person tag",
  "company.name": "Company name",
  "company.domain": "Company domain",
  "company.created_at": "Company added",
  "company.tags": "Company tag",
  audience: "Saved audience",
  "session.verified": "Signed in this visit",
  "session.country": "Country this visit",
  "session.browser": "Browser",
  "session.os": "Operating system",
  "visitor.locale": "Browser language",
  "visitor.device": "Device",
  "page.url": "Page address",
  "page.path": "Page path",
  "page.query": "Page query",
  "page.host": "Page host",
  "page.referrer": "Came from",
};

/*
 * humanize turns a custom attribute key into words: trial_ends_at reads
 * "Trial ends at".
 */
function humanize(key: string): string {
  const words = key.replace(/_/g, " ").trim();
  return words.charAt(0).toUpperCase() + words.slice(1);
}

export function fieldName(key: string): string {
  const known = fieldNames[key];
  if (known) {
    return known;
  }
  const custom = key.match(/^(contact|company)\.custom\.(.+)$/);
  if (custom?.[2]) {
    return custom[1] === "company" ? `Company ${humanize(custom[2]).toLowerCase()}` : humanize(custom[2]);
  }

  return key;
}

export type FieldGroup = "Person" | "Company" | "Tags and audiences" | "Visit" | "Page";

export function fieldGroup(key: string): FieldGroup {
  if (key === "contact.tags" || key === "company.tags" || key === "audience") {
    return "Tags and audiences";
  }
  if (key.startsWith("company.")) {
    return "Company";
  }
  if (key.startsWith("page.") || key.startsWith("visitor.")) {
    return "Page";
  }
  if (key.startsWith("session.")) {
    return "Visit";
  }

  return "Person";
}

export const fieldGroups: FieldGroup[] = ["Person", "Company", "Tags and audiences", "Visit", "Page"];

export const opNames: Record<string, string> = {
  eq: "is",
  neq: "is not",
  in: "is any of",
  not_in: "is none of",
  contains: "contains",
  not_contains: "does not contain",
  starts_with: "starts with",
  ends_with: "ends with",
  contains_any: "contains any of",
  not_contains_any: "contains none of",
  exists: "is known",
  not_exists: "is unknown",
  gt: "is more than",
  gte: "is at least",
  lt: "is less than",
  lte: "is at most",
  between: "is between",
  is_true: "is yes",
  is_false: "is no",
  before: "is before",
  after: "is after",
  on: "is on",
  within_last_days: "is within the last",
  more_than_days_ago: "is more than",
  days_ago: "is exactly",
  within_next_days: "is within the next",
  more_than_days_from_now: "is more than",
  has: "has",
  has_any: "has any of",
  has_none: "has none of",
  matches: "matches",
};

/*
 * Shape is what kind of value an operator takes.
 */
export type Shape = "none" | "list" | "number" | "range" | "days" | "date" | "text";

export function shapeOf(op: string, type?: AudienceField["type"]): Shape {
  if (["exists", "not_exists", "is_true", "is_false"].includes(op)) {
    return "none";
  }
  if (["in", "not_in", "contains_any", "not_contains_any", "has_any", "has_none"].includes(op)) {
    return "list";
  }
  if (["gt", "gte", "lt", "lte"].includes(op) || (type === "number" && (op === "eq" || op === "neq"))) {
    return "number";
  }
  if (op === "between") {
    return "range";
  }
  if (
    ["within_last_days", "more_than_days_ago", "days_ago", "within_next_days", "more_than_days_from_now"].includes(op)
  ) {
    return "days";
  }
  if (["before", "after", "on"].includes(op)) {
    return "date";
  }

  return "text";
}

/*
 * daysSuffix is what follows the number of days, so the line reads as a
 * sentence: "Last seen is more than 30 days ago".
 */
export function daysSuffix(op: string): string {
  if (op === "more_than_days_ago" || op === "days_ago") {
    return "days ago";
  }
  if (op === "more_than_days_from_now") {
    return "days from now";
  }

  return "days";
}

/*
 * blankValue is the value a condition starts with for an operator.
 */
export function blankValue(shape: Shape): unknown {
  switch (shape) {
    case "none":
      return undefined;
    case "list":
      return [];
    case "number":
      return 0;
    case "range":
      return [0, 10];
    case "days":
      return 30;
    default:
      return "";
  }
}

/*
 * withField moves a condition to another field, starting from its first
 * operator.
 */
export function withField(field: AudienceField): Condition {
  const op = field.ops[0] ?? "eq";
  const value = blankValue(shapeOf(op, field.type));

  return value === undefined ? { field: field.key, op } : { field: field.key, op, value };
}

/*
 * withOp changes a condition's operator, keeping the value when it still
 * fits.
 */
export function withOp(condition: Condition, op: string, type?: AudienceField["type"]): Condition {
  const before = shapeOf(condition.op, type);
  const after = shapeOf(op, type);
  const value = before === after ? condition.value : blankValue(after);

  return value === undefined ? { field: condition.field, op } : { field: condition.field, op, value };
}

/*
 * listText is a list value as typed: words separated by commas.
 */
export function listText(value: unknown): string {
  return Array.isArray(value) ? value.join(", ") : "";
}

export function listOf(text: string): string[] {
  return text
    .split(",")
    .map((word) => word.trim())
    .filter(Boolean);
}

/*
 * valueWords says a condition's value in words, names in place of ids.
 */
export function valueWords(condition: Condition, names: Record<string, string> = {}): string {
  const shape = shapeOf(condition.op);
  const v = condition.value;
  const name = (x: unknown) => names[String(x)] ?? String(x);
  if (shape === "none") {
    return "";
  }
  if (Array.isArray(v)) {
    return shape === "range" ? `${v[0]} and ${v[1]}` : v.map(name).join(", ");
  }
  if (shape === "days") {
    return `${v} ${daysSuffix(condition.op)}`;
  }

  return name(v);
}

/*
 * sentence says a condition in words: "Plan is any of pro, team".
 */
export function sentence(condition: Condition, names: Record<string, string> = {}): string {
  return [fieldName(condition.field), opNames[condition.op] ?? condition.op, valueWords(condition, names)]
    .filter(Boolean)
    .join(" ");
}

/*
 * summary reads a whole audience as one line, for the list and the
 * header.
 */
export function summary(rules: AudienceRules, names: Record<string, string> = {}): string {
  const who = visitorChoices.find(([v]) => v === (rules.visitors ?? "everyone"))?.[1] ?? "Everyone";
  const form = toForm(rules);
  if (!form) {
    return `${who}, with a rule written through the API`;
  }
  if (form.items.length === 0) {
    return who;
  }
  const lines = form.items.map((item) =>
    item.kind === "condition" ? sentence(item.condition, names) : `${item.conditions.length} grouped conditions`,
  );
  const joined: Record<Match, string> = {
    all: lines.join(" and "),
    any: lines.join(" or "),
    none: `none of: ${lines.join("; ")}`,
  };

  return `${who} where ${joined[form.match]}`;
}
