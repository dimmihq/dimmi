/*
 * The choices of an import's mapping: the fields a column can fill for
 * people or companies, the custom attributes of that kind, and a new
 * attribute the import defines from the column's header. A field is
 * picked at most once, and a mapping can run only when it names
 * something to find each row by.
 */
import type { AttributeDefinition } from "@/lib/api/attributes";
import type { ImportColumn, ImportKind, ImportPreview } from "@/lib/api/imports";

export const skipField = "";
export const newAttributeField = "attribute.__new__";

export interface FieldChoice {
  id: string;
  label: string;
}

const standard: Record<ImportKind, FieldChoice[]> = {
  contacts: [
    { id: "email", label: "Email" },
    { id: "name", label: "Name" },
    { id: "external_id", label: "External ID" },
    { id: "phone", label: "Phone" },
    { id: "company_name", label: "Company name" },
    { id: "company_domain", label: "Company domain" },
    { id: "tags", label: "Tags" },
  ],
  companies: [
    { id: "name", label: "Name" },
    { id: "domain", label: "Domain" },
    { id: "external_id", label: "External ID" },
    { id: "tags", label: "Tags" },
  ],
};

/*
 * fieldChoices lists what a column of the kind can fill: the standard
 * fields, then the kind's active custom attributes.
 */
export function fieldChoices(kind: ImportKind, attributes: AttributeDefinition[]): FieldChoice[] {
  const of = kind === "contacts" ? "contact" : "company";
  const custom = attributes
    .filter((a) => a.of === of && a.status === "active")
    .map((a) => ({ id: `attribute.${a.key}`, label: a.label }));

  return [...standard[kind], ...custom];
}

/*
 * attributeKey turns a header into a key a new attribute can take:
 * lower case, words joined by underscores, starting with a letter.
 */
export function attributeKey(header: string): string {
  const key = header
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_+|_+$/g, "")
    .slice(0, 64);

  return /^[a-z]/.test(key) ? key : `field_${key}`.replace(/_+$/, "");
}

/*
 * keyFields are the fields that find a row of the kind.
 */
export function keyFields(kind: ImportKind): string[] {
  return kind === "contacts" ? ["email", "external_id"] : ["domain", "external_id"];
}

/*
 * planFrom turns the field picked for each column into the mapping the
 * server takes, leaving out skipped columns.
 */
export function planFrom(fields: string[]): ImportColumn[] {
  return fields.flatMap((field, column) => (field === skipField ? [] : [{ column, field }]));
}

/*
 * mappingProblem says why a mapping cannot run yet, or null when it can.
 */
export function mappingProblem(kind: ImportKind, fields: string[]): string | null {
  const used = fields.filter((f) => f !== skipField);
  if (new Set(used).size !== used.length) {
    return "Each field can be filled by one column only.";
  }
  if (used.some((f) => f === "attribute." || f === newAttributeField)) {
    return "Name each new attribute.";
  }
  if (!keyFields(kind).some((f) => used.includes(f))) {
    return kind === "contacts"
      ? "Map an email or external ID column, so each person can be found."
      : "Map a domain or external ID column, so each company can be found.";
  }

  return null;
}

/*
 * initialFields returns the field the server suggests for each column,
 * skipping the rest.
 */
export function initialFields(columns: string[], suggested: ImportColumn[]): string[] {
  return columns.map((_, i) => suggested.find((s) => s.column === i)?.field ?? skipField);
}

/*
 * previewSummary words a dry run's counts.
 */
export function previewSummary(p: ImportPreview, noun: string): string {
  const parts = [
    p.created > 0 ? `${p.created} new` : null,
    p.updated > 0 ? `${p.updated} updated` : null,
    p.skipped > 0 ? `${p.skipped} skipped` : null,
    p.failed > 0 ? `${p.failed} with problems` : null,
  ].filter(Boolean);

  return `${p.rows} ${noun}: ${parts.join(", ") || "nothing to change"}.`;
}
