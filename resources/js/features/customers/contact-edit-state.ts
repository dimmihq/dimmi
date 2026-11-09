/*
 * What the customer edit modal holds while the member types, and the one
 * change it sends on Save. Only what differs from the customer as loaded
 * is sent; a cleared field or attribute is removed rather than saved
 * empty, and typed attributes keep their type.
 */
import type { AttributeDefinition } from "@/lib/api/attributes";
import type { Contact, ContactChanges } from "@/lib/api/customers";
import type { Tag } from "@/lib/api/tags";

/*
 * ExtraAttribute is an attribute the workspace keeps no definition for,
 * one row with its own key.
 */
export interface ExtraAttribute {
  id: string;
  key: string;
  value: string;
}

export interface EditState {
  name: string;
  email: string;
  phone: string;
  notes: string;
  tags: Tag[];
  typed: Record<string, string>;
  extra: ExtraAttribute[];
}

/*
 * textOf shows a stored attribute as the text its input edits.
 */
export function textOf(value: unknown, type?: AttributeDefinition["data_type"]): string {
  if (value === null || value === undefined) {
    return "";
  }
  if (typeof value === "string") {
    return type === "date" ? value.slice(0, 10) : value;
  }

  return typeof value === "object" ? JSON.stringify(value) : String(value);
}

/*
 * definedOn lists the workspace's active attributes of people, the ones
 * every customer gets an input for.
 */
export function definedOn(definitions: AttributeDefinition[]): AttributeDefinition[] {
  return definitions.filter((d) => d.of === "contact" && d.status === "active");
}

export function editStateOf(contact: Contact, defined: AttributeDefinition[]): EditState {
  const keys = new Set(defined.map((d) => d.key));

  return {
    name: contact.name ?? "",
    email: contact.email ?? "",
    phone: contact.phone ?? "",
    notes: contact.notes ?? "",
    tags: contact.tags,
    typed: Object.fromEntries(defined.map((d) => [d.key, textOf(contact.custom_attributes[d.key], d.data_type)])),
    extra: Object.entries(contact.custom_attributes)
      .filter(([key]) => !keys.has(key))
      .map(([key, value]) => ({ id: key, key, value: textOf(value) })),
  };
}

/*
 * guessed reads an untyped value the way the product sends it: true,
 * false and numbers keep their type, anything else is text.
 */
function guessed(text: string): unknown {
  if (text === "true" || text === "false") {
    return text === "true";
  }

  return Number.isFinite(Number(text)) ? Number(text) : text;
}

function typedValue(text: string, type: AttributeDefinition["data_type"] | undefined): unknown {
  const trimmed = text.trim();
  if (trimmed === "") {
    return null;
  }
  if (type === "number") {
    return Number(trimmed);
  }
  if (type === "boolean") {
    return trimmed === "true";
  }

  return type === undefined ? guessed(trimmed) : trimmed;
}

function finalTexts(state: EditState): Map<string, string> {
  const out = new Map(Object.entries(state.typed));
  for (const row of state.extra) {
    if (row.key.trim() !== "") {
      out.set(row.key.trim(), row.value);
    }
  }

  return out;
}

function attributeChanges(contact: Contact, defined: AttributeDefinition[], state: EditState) {
  const types = new Map(defined.map((d) => [d.key, d.data_type]));
  const texts = finalTexts(state);
  const out: Record<string, unknown> = {};
  for (const key of Object.keys(contact.custom_attributes)) {
    if (!texts.has(key)) {
      out[key] = null;
    }
  }
  for (const [key, text] of texts) {
    const before = textOf(contact.custom_attributes[key], types.get(key));
    if (text.trim() !== before.trim()) {
      out[key] = typedValue(text, types.get(key));
    }
  }

  return out;
}

function textChange(before: string | null, after: string): string | null | undefined {
  const next = after.trim() === "" ? null : after.trim();

  return next === (before ?? null) ? undefined : next;
}

/*
 * changesOf is everything the member changed, as the one update the
 * server applies; it is empty when nothing changed.
 */
export function changesOf(contact: Contact, defined: AttributeDefinition[], state: EditState): ContactChanges {
  const had = new Set(contact.tags.map((t) => t.id));
  const has = new Set(state.tags.map((t) => t.id));
  const fields: ContactChanges = {
    name: textChange(contact.name, state.name),
    email: textChange(contact.email, state.email),
    phone: textChange(contact.phone, state.phone),
    notes: textChange(contact.notes, state.notes),
    add_tags: state.tags.filter((t) => !had.has(t.id)).map((t) => t.name),
    remove_tags: contact.tags.filter((t) => !has.has(t.id)).map((t) => t.id),
    custom_attributes: attributeChanges(contact, defined, state),
  };

  return Object.fromEntries(
    Object.entries(fields).filter(([, value]) => {
      if (value === undefined) {
        return false;
      }

      return typeof value !== "object" || value === null || Object.keys(value).length > 0;
    }),
  ) as ContactChanges;
}
