/*
 * The fields people can be filtered by, as the Add filter menu lists them:
 * the person's own data, then each active custom attribute of contacts.
 * Each field has its comparisons, picked before the value, as Intercom
 * does: text is, is not, contains and the like; numbers compare; tags are
 * any, all or none of several picked at once; times count days back. A
 * filter is one chip, turned here into a condition the server checks; a
 * chip still missing its value narrows nothing.
 */
import {
  type RemixiconComponentType,
  RiBuilding2Line,
  RiCalendarLine,
  RiFingerprintLine,
  RiGroupLine,
  RiHashtag,
  RiListCheck,
  RiMailLine,
  RiPhoneLine,
  RiPriceTag3Line,
  RiText,
  RiTimeLine,
  RiToggleLine,
  RiUser3Line,
  RiUserLine,
} from "@remixicon/react";

import type { AttributeDefinition } from "@/lib/api/attributes";
import type { Condition } from "@/lib/api/customers";

export type FieldKind = "text" | "number" | "boolean" | "list" | "days" | "kind" | "company" | "tags" | "audience";

export interface FilterField {
  key: string;
  label: string;
  description: string;
  icon: RemixiconComponentType;
  group: "People data" | "Your attributes";
  kind: FieldKind;
  options?: string[];
  attribute?: AttributeDefinition;
}

/*
 * Needs is what a comparison asks for after it: nothing, a typed text,
 * a number, a count of days, one choice or several.
 */
export type Needs = "none" | "text" | "number" | "days" | "one" | "many";

export interface Comparison {
  id: string;
  label: string;
  needs: Needs;
}

/*
 * Filter is one chip. Values holds what was picked, and shown how each
 * reads, such as a tag's name for its id.
 */
export interface Filter {
  id: string;
  field: string;
  op: string;
  value: string;
  values: string[];
  shown: string[];
}

const presence: Comparison[] = [
  { id: "unknown", label: "is unknown", needs: "none" },
  { id: "known", label: "has any value", needs: "none" },
];

const comparisons: Record<FieldKind, Comparison[]> = {
  text: [
    { id: "is", label: "is", needs: "text" },
    { id: "is_not", label: "is not", needs: "text" },
    { id: "contains", label: "contains", needs: "text" },
    { id: "not_contains", label: "does not contain", needs: "text" },
    { id: "starts_with", label: "starts with", needs: "text" },
    { id: "ends_with", label: "ends with", needs: "text" },
    ...presence,
  ],
  number: [
    { id: "eq", label: "is", needs: "number" },
    { id: "neq", label: "is not", needs: "number" },
    { id: "gt", label: "is greater than", needs: "number" },
    { id: "lt", label: "is less than", needs: "number" },
    ...presence,
  ],
  boolean: [
    { id: "true", label: "is true", needs: "none" },
    { id: "false", label: "is false", needs: "none" },
    ...presence,
  ],
  list: [{ id: "is", label: "is", needs: "one" }, { id: "is_not", label: "is not", needs: "one" }, ...presence],
  days: [
    { id: "less_than", label: "less than", needs: "days" },
    { id: "more_than", label: "more than", needs: "days" },
    { id: "unknown", label: "never", needs: "none" },
  ],
  kind: [
    { id: "is", label: "is", needs: "one" },
    { id: "is_not", label: "is not", needs: "one" },
  ],
  company: [
    { id: "is", label: "is any of", needs: "many" },
    { id: "is_not", label: "is none of", needs: "many" },
    ...presence,
  ],
  audience: [
    { id: "in_any", label: "is in any of", needs: "many" },
    { id: "in_none", label: "is in none of", needs: "many" },
  ],
  tags: [
    { id: "has_any", label: "has any of", needs: "many" },
    { id: "has_all", label: "has all of", needs: "many" },
    { id: "has_none", label: "has none of", needs: "many" },
  ],
};

/*
 * comparisonsOf lists how a field compares, the first being the default.
 */
export function comparisonsOf(field: FilterField): Comparison[] {
  return comparisons[field.kind];
}

export function comparisonOf(field: FilterField, op: string): Comparison {
  const all = comparisonsOf(field);

  return all.find((c) => c.id === op) ?? (all[0] as Comparison);
}

/*
 * maxFilters is how many chips the list takes at once.
 */
export const maxFilters = 10;

const people: FilterField[] = [
  { key: "name", label: "Name", description: "The person's full name.", icon: RiUserLine, kind: "text" as const },
  {
    key: "email",
    label: "Email",
    description: "The person's email address.",
    icon: RiMailLine,
    kind: "text" as const,
  },
  {
    key: "phone",
    label: "Phone",
    description: "The person's phone number.",
    icon: RiPhoneLine,
    kind: "text" as const,
  },
  {
    key: "user_id",
    label: "User ID",
    description: "The id your product knows the person by.",
    icon: RiFingerprintLine,
    kind: "text" as const,
  },
  {
    key: "kind",
    label: "Type",
    description: "Users the product signed in, or leads nobody knows yet.",
    icon: RiUser3Line,
    kind: "kind" as const,
    options: ["user", "lead"],
  },
  {
    key: "company",
    label: "Company",
    description: "The company the person works at.",
    icon: RiBuilding2Line,
    kind: "company" as const,
  },
  { key: "tags", label: "Tags", description: "Tags the person carries.", icon: RiPriceTag3Line, kind: "tags" as const },
  {
    key: "audience",
    label: "Audience",
    description: "Saved audiences the person is in right now.",
    icon: RiGroupLine,
    kind: "audience" as const,
  },
  {
    key: "last_seen",
    label: "Last seen",
    description: "When the person was last active.",
    icon: RiTimeLine,
    kind: "days" as const,
  },
  {
    key: "created",
    label: "Signed up",
    description: "When the person was added to dimmi.",
    icon: RiCalendarLine,
    kind: "days" as const,
  },
].map((f) => ({ ...f, group: "People data" as const }));

const attributeKinds: Record<AttributeDefinition["data_type"], FieldKind> = {
  string: "text",
  number: "number",
  boolean: "boolean",
  date: "text",
  list: "list",
};

const attributeIcons: Record<AttributeDefinition["data_type"], RemixiconComponentType> = {
  string: RiText,
  number: RiHashtag,
  boolean: RiToggleLine,
  date: RiCalendarLine,
  list: RiListCheck,
};

/*
 * filterFields lists every field, the workspace's contact attributes
 * after the person's own data. Dated attributes are left out until the
 * list compares them as times.
 */
export function filterFields(attributes: AttributeDefinition[]): FilterField[] {
  return [
    ...people,
    ...attributes
      .filter((a) => a.of === "contact" && a.status === "active" && a.data_type !== "date")
      .map(
        (a): FilterField => ({
          key: `custom.${a.key}`,
          label: a.label || a.key,
          description: a.description || `The ${a.label || a.key} attribute.`,
          icon: attributeIcons[a.data_type],
          group: "Your attributes",
          kind: attributeKinds[a.data_type],
          options: a.options,
          attribute: a,
        }),
      ),
  ];
}

/*
 * complete says the chip has what its comparison asks for.
 */
export function complete(filter: Filter, field: FilterField): boolean {
  const needs = comparisonOf(field, filter.op).needs;
  const typed = filter.value.trim();
  const checks: Record<Needs, () => boolean> = {
    none: () => true,
    one: () => filter.values.length > 0,
    many: () => filter.values.length > 0,
    days: () => Number.isInteger(Number(typed)) && Number(typed) > 0,
    number: () => typed !== "" && !Number.isNaN(Number(typed)),
    text: () => typed !== "",
  };

  return checks[needs]();
}

function picksText(filter: Filter): string {
  const names = filter.shown.length > 0 ? filter.shown : filter.values;
  if (names.length === 0) {
    return "…";
  }

  return names.length > 2 ? `${names.slice(0, 2).join(", ")} +${names.length - 2}` : names.join(", ");
}

function daysText(value: string): string {
  return value ? `${value} ${value === "1" ? "day" : "days"} ago` : "…";
}

function valueText(filter: Filter, needs: Needs): string {
  const texts: Record<Needs, () => string> = {
    none: () => "",
    days: () => daysText(filter.value),
    one: () => picksText(filter),
    many: () => picksText(filter),
    number: () => filter.value || "…",
    text: () => filter.value || "…",
  };

  return texts[needs]();
}

/*
 * chipText is how a chip reads: the field, how it compares and the
 * value.
 */
export function chipText(filter: Filter, field: FilterField | undefined): string {
  if (!field) {
    return filter.field;
  }
  const c = comparisonOf(field, filter.op);

  return [field.label, c.label, valueText(filter, c.needs)].filter(Boolean).join(" ");
}

/*
 * conditionOf turns a complete chip into the condition the server reads:
 * picks of companies and tags as a list, other picks as one value.
 */
function conditionOf(filter: Filter, field: FilterField): Condition {
  const needs = comparisonOf(field, filter.op).needs;
  const out: Condition = { field: field.key, op: filter.op };
  if (field.attribute) {
    out.type = field.attribute.data_type === "list" ? "string" : field.attribute.data_type;
  }
  if (needs === "days") {
    out.days = Number(filter.value);
  } else if (needs === "many") {
    out.values = filter.values;
  } else if (needs === "one") {
    out.value = filter.values[0];
  } else if (needs !== "none") {
    out.value = filter.value.trim();
  }

  return out;
}

/*
 * conditions turns the chips into the list's conditions, leaving out
 * those still missing a value.
 */
export function conditions(filters: Filter[], fields: FilterField[]): Condition[] {
  return filters.flatMap((filter) => {
    const field = fields.find((f) => f.key === filter.field);

    return field && complete(filter, field) ? [conditionOf(filter, field)] : [];
  });
}

/*
 * newFilter starts a chip of the field with its first comparison.
 */
export function newFilter(field: FilterField): Filter {
  return {
    id: crypto.randomUUID(),
    field: field.key,
    op: comparisonsOf(field)[0]?.id ?? "is",
    value: "",
    values: [],
    shown: [],
  };
}
