/*
 * The editable shapes of an action. Inputs are edited as rows (a name, a
 * type, whether it is required, a description and its limits) and saved
 * as the JSON Schema the agent receives. Templates hold placeholders
 * such as {input.order_id} or {contact.external_id}, and every one must
 * name something that exists. Response fields are picked from paths of
 * a real answer. Nothing here talks to the server.
 */
import type { FieldType, InputSchema, SchemaProperty } from "./api";

/*
 * InputField is one input row: what the agent fills in. choices holds
 * the allowed values of a text field, separated by commas; empty means
 * any text.
 */
export interface InputField {
  key: string;
  name: string;
  type: FieldType;
  required: boolean;
  description: string;
  choices: string;
  minimum: string;
  maximum: string;
}

export const fieldTypeLabels: Record<FieldType, string> = {
  string: "Text",
  number: "Number",
  integer: "Whole number",
  boolean: "Yes or no",
};

/*
 * fieldsFromSchema turns a schema into rows, in the order its
 * properties are listed.
 */
export function fieldsFromSchema(schema: InputSchema): InputField[] {
  const required = new Set(schema.required ?? []);

  return Object.entries(schema.properties ?? {}).map(([name, property]) => ({
    key: name,
    name,
    type: property.type,
    required: required.has(name),
    description: property.description ?? "",
    choices: (property.enum ?? []).join(", "),
    minimum: property.minimum === undefined ? "" : String(property.minimum),
    maximum: property.maximum === undefined ? "" : String(property.maximum),
  }));
}

/*
 * schemaFromFields turns rows back into the schema the server stores.
 * Rows without a name are left out; limits only apply to numbers.
 */
export function schemaFromFields(fields: InputField[]): InputSchema {
  const properties: Record<string, SchemaProperty> = {};
  const required: string[] = [];
  for (const field of fields) {
    const name = field.name.trim();
    if (!name) {
      continue;
    }
    properties[name] = propertyOf(field);
    if (field.required) {
      required.push(name);
    }
  }

  return { type: "object", properties, required, additionalProperties: false };
}

function propertyOf(field: InputField): SchemaProperty {
  const property: SchemaProperty = { type: field.type };
  if (field.description.trim()) {
    property.description = field.description.trim();
  }
  const choices = field.choices
    .split(",")
    .map((choice) => choice.trim())
    .filter(Boolean);
  if (choices.length > 0 && field.type === "string") {
    property.enum = choices;
  }
  if (field.type === "number" || field.type === "integer") {
    const minimum = Number.parseFloat(field.minimum);
    const maximum = Number.parseFloat(field.maximum);
    if (!Number.isNaN(minimum)) {
      property.minimum = minimum;
    }
    if (!Number.isNaN(maximum)) {
      property.maximum = maximum;
    }
  }

  return property;
}

/*
 * parseSchema reads a schema typed as JSON, answering a problem in words
 * when it is not one this editor can use.
 */
export function parseSchema(text: string): { schema: InputSchema } | { problem: string } {
  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch {
    return { problem: "This is not valid JSON." };
  }
  if (!parsed || typeof parsed !== "object" || (parsed as { type?: unknown }).type !== "object") {
    return { problem: "The schema must be an object with type “object”." };
  }
  const properties = (parsed as { properties?: unknown }).properties ?? {};
  if (typeof properties !== "object" || properties === null) {
    return { problem: "properties must be an object." };
  }

  return { schema: { ...(parsed as InputSchema), properties: properties as InputSchema["properties"] } };
}

const namePattern = /^[a-z][a-z0-9_]{2,47}$/;
const reserved = ["search_knowledge", "handoff"];

/*
 * nameProblem says what is wrong with an action's tool name, or null
 * when the agent can use it.
 */
export function nameProblem(name: string): string | null {
  if (!name) {
    return "Give the action a name.";
  }
  if (!namePattern.test(name)) {
    return "Use 3 to 48 lowercase letters, digits and underscores, starting with a letter.";
  }
  if (reserved.includes(name) || name.startsWith("dimmi_")) {
    return "This name is reserved for dimmi.";
  }

  return null;
}

/*
 * toSnake turns a title into a tool name: "Refund an invoice" becomes
 * refund_an_invoice.
 */
export function toSnake(title: string): string {
  return title
    .normalize("NFKD")
    .replace(/[̀-ͯ]/g, "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_+|_+$/g, "")
    .replace(/^[0-9_]+/, "")
    .slice(0, 48);
}

const placeholderPattern = /\{([a-z]+)\.([a-z0-9_.]+)\}/g;

/*
 * placeholdersIn lists the placeholders a template uses, such as
 * input.order_id, in the order they appear.
 */
export function placeholdersIn(text: string): string[] {
  return [...text.matchAll(placeholderPattern)].map((match) => `${match[1]}.${match[2]}`);
}

/*
 * contactPlaceholders are the verified details of a signed-in customer
 * a template can read.
 */
export const contactPlaceholders = [
  { token: "contact.external_id", label: "Customer's ID in your system" },
  { token: "contact.email", label: "Customer's verified email" },
  { token: "contact.phone", label: "Customer's verified phone" },
  { token: "contact.company.external_id", label: "Company's ID in your system" },
];

const contactPaths = ["id", "external_id", "email", "phone", "company.external_id"];
const fixedPaths: Record<string, string[]> = {
  conversation: ["id", "number", "channel_type"],
  workspace: ["id"],
  call: ["id"],
};

/*
 * unknownPlaceholder returns the first placeholder of a template that
 * names nothing: an input the action does not have, a connection
 * variable never set, or a namespace that does not exist.
 */
export function unknownPlaceholder(text: string, known: { inputs: string[]; variables: string[] }): string | null {
  for (const placeholder of placeholdersIn(text)) {
    if (!placeholderKnown(placeholder, known)) {
      return placeholder;
    }
  }

  return null;
}

function placeholderKnown(placeholder: string, known: { inputs: string[]; variables: string[] }): boolean {
  const [space = "", ...rest] = placeholder.split(".");
  const path = rest.join(".");
  if (space === "input") {
    return known.inputs.includes(rest[0] ?? "");
  }
  if (space === "connection") {
    return known.variables.includes(path);
  }
  if (space === "contact") {
    return contactPaths.includes(path) || /^(attr|custom_attributes)\.[a-z0-9_]+$/.test(path);
  }

  return fixedPaths[space]?.includes(path) ?? false;
}

/*
 * usesIdentity says a template reads who the customer is, which only
 * resolves when they signed in.
 */
export function usesIdentity(texts: string[]): boolean {
  return texts.some((text) =>
    placeholdersIn(text).some((placeholder) => placeholder.startsWith("contact.") && placeholder !== "contact.id"),
  );
}

/*
 * identityLike says an input's name looks like it names the customer,
 * such as customer_id or email: something the agent must never choose.
 */
export function identityLike(name: string): boolean {
  return /^(user|customer|account|contact|member)?_?(id|email|external_id|uuid)$/.test(name) && name !== "id";
}

/*
 * responsePaths lists the paths of an answer a response field can keep,
 * down to three levels, with the first item standing in for each list
 * (data.0.status). Lists of scalars stop at the list itself.
 */
export function responsePaths(body: unknown, prefix = "", depth = 0): string[] {
  if (body === null || typeof body !== "object" || depth > 2) {
    return prefix ? [prefix] : [];
  }
  if (Array.isArray(body)) {
    const first = body[0];
    if (first === undefined || first === null || typeof first !== "object") {
      return prefix ? [prefix] : [];
    }
    return responsePaths(first, join(prefix, "0"), depth);
  }

  return Object.entries(body).flatMap(([key, value]) => responsePaths(value, join(prefix, key), depth + 1));
}

function join(prefix: string, key: string): string {
  return prefix ? `${prefix}.${key}` : key;
}

/*
 * fieldNameOf turns a response path into a short field name the agent
 * reads: data.0.status becomes status, shipment.carrier becomes
 * shipment_carrier.
 */
export function fieldNameOf(path: string): string {
  const parts = path.split(".").filter((part) => !/^[0-9#]+$/.test(part) && part !== "data");

  return toSnake(parts.slice(-2).join("_")) || "value";
}
