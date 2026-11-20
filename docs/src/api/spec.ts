/*
 * The REST API reference, built from the committed contract
 * (tests/contracts/api_v1.openapi.json) at build time: one page per
 * public operation (those not marked x-internal), the API tab's sidebar,
 * field lists, example payloads, code samples and each page's Markdown.
 * Nothing here reaches the browser.
 */
import raw from "../../../tests/contracts/api_v1.openapi.json?raw";

export type Schema = {
  $ref?: string;
  type?: string | string[];
  format?: string;
  description?: string;
  properties?: Record<string, Schema>;
  required?: string[];
  items?: Schema;
  additionalProperties?: boolean | Schema;
  enum?: unknown[];
  default?: unknown;
  anyOf?: Schema[];
  oneOf?: Schema[];
  allOf?: Schema[];
  minLength?: number;
  maxLength?: number;
  minItems?: number;
  maxItems?: number;
  minimum?: number;
  maximum?: number;
};

type Parameter = { in: string; name: string; required?: boolean; description?: string; schema?: Schema };
type Media = Record<string, { schema?: Schema }>;
type RawOperation = {
  operationId: string;
  summary?: string;
  description?: string;
  deprecated?: boolean;
  parameters?: Parameter[];
  requestBody?: { required?: boolean; content?: Media };
  responses: Record<string, { description?: string; content?: Media }>;
  "x-abilities"?: string[];
  "x-internal"?: boolean;
};

type Spec = {
  info: { title: string; version: string };
  paths: Record<string, Record<string, RawOperation>>;
  components: { schemas: Record<string, Schema> };
};

const spec = JSON.parse(raw) as Spec;

export const API_VERSION = spec.info.version;
export const BASE_URL = "https://trydimmi.com/api/v1";
export const API_ROOT = "/api-reference/";

/* ---------------------------------------------------------------- naming */

const ACRONYMS: Record<string, string> = {
  ai: "AI",
  api: "API",
  sla: "SLA",
  sso: "SSO",
  dkim: "DKIM",
  dns: "DNS",
  gif: "GIF",
  oauth: "OAuth",
  chatgpt: "ChatGPT",
  url: "URL",
};

const words = (segment: string) =>
  segment
    .split(/[-_]/)
    .map((word) => ACRONYMS[word] ?? word)
    .join(" ");

const capitalize = (text: string) => text.charAt(0).toUpperCase() + text.slice(1);

const UNCOUNTABLE = new Set(["series", "settings", "preferences", "analytics", "news", "status", "focus"]);

function singular(segment: string): string {
  if (UNCOUNTABLE.has(segment) || segment.endsWith("ss") || segment.endsWith("us")) {
    return segment;
  }
  if (segment.endsWith("ies")) {
    return `${segment.slice(0, -3)}y`;
  }
  if (/(sses|uses|xes|ches|shes)$/.test(segment)) {
    return segment.slice(0, -2);
  }
  return segment.endsWith("s") ? segment.slice(0, -1) : segment;
}

// Sub-resources that name an action rather than a thing: POST to one
// performs the action on its parent.
const ACTIONS: Record<string, string> = {
  cancel: "Cancel",
  pause: "Pause",
  resume: "Resume",
  send: "Send",
  stop: "Stop",
  duplicate: "Duplicate",
  restore: "Restore",
  retry: "Retry",
  test: "Test",
  tests: "Test",
  checks: "Check",
  publications: "Publish",
  restorations: "Restore",
  merges: "Merge",
  freezes: "Freeze",
  previews: "Preview",
  connect: "Connect",
  archive: "Archive",
  unarchive: "Unarchive",
  redelivery: "Redeliver",
  callback: "Complete",
  "tool-syncs": "Sync tools for",
  "dkim-rotations": "Rotate DKIM for",
  "dns-setups": "Set up DNS for",
  renderings: "Render",
  "template-renderings": "Render template for",
};

const VERBS: Record<string, string> = {
  index: "List",
  show: "Get",
  store: "Create",
  update: "Update",
  destroy: "Delete",
  "update-many": "Update",
  "destroy-many": "Delete",
  reorder: "Reorder",
  raw: "Get raw",
};

function titleFor(id: string, path: string): string {
  const segments = id.split(".");
  const action = segments.pop() ?? "";
  const last = segments.pop() ?? "";
  const item = /\}$/.test(path);
  const parents = segments.map((segment) => words(segment === "ai" ? segment : singular(segment)));
  const phrase = (noun: string) => [...parents, words(noun)].join(" ");

  if (action === "store" && ACTIONS[last]) {
    return `${ACTIONS[last]} ${parents.join(" ") || words(singular(last))}`.trim();
  }
  if (action === "index" || action === "reorder" || action.endsWith("-many")) {
    return `${VERBS[action]} ${phrase(last)}`;
  }
  if (VERBS[action]) {
    return `${VERBS[action]} ${phrase(item || action === "store" ? singular(last) : last)}`;
  }
  return `${capitalize(words(action))} ${phrase(singular(last))}`;
}

const kebab = (text: string) =>
  text
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");

/* -------------------------------------------------------------- grouping */

// Each category of the sidebar and the resources (first path segments) in
// it; a trailing * matches a prefix.
const CATEGORIES: [string, string[]][] = [
  [
    "Conversations",
    [
      "conversations",
      "conversation-*",
      "drafts",
      "tags",
      "saved-replies",
      "saved-reply-imports",
      "snooze-*",
      "gif-search",
      "search",
      "inboxes",
      "inbox-preferences",
      "sla-policies",
      "schedules",
      "redaction-*",
    ],
  ],
  ["Contacts and companies", ["contacts", "contact-*", "companies", "attribute-*", "audience*", "event-*", "imports"]],
  ["Tickets", ["ticket*"]],
  ["AI agent", ["ai", "knowledge", "operator", "procedures"]],
  ["Help center", ["help-*", "articles", "collections"]],
  ["Messenger", ["widget-*", "identity-*"]],
  ["Broadcasts and series", ["broadcast*", "series"]],
  ["Email", ["email-addresses", "email-domains", "email-signatures", "email-suppressions"]],
  ["Feedback and changelog", ["feedback-*", "changelog-*"]],
  ["Workflows and integrations", ["workflows", "integration*", "channels"]],
  ["Webhooks", ["webhook-*"]],
  ["Reports and data", ["report*", "data-exports", "audit-logs", "retention-previews"]],
  [
    "Workspace and team",
    [
      "workspace",
      "workspaces",
      "members",
      "member-imports",
      "invitations",
      "teams",
      "roles",
      "permissions",
      "organization-domains",
      "sso-*",
      "tokens",
    ],
  ],
  ["Notifications", ["notification*", "push-*"]],
  [
    "Account",
    [
      "user",
      "users",
      "me",
      "sessions",
      "password-resets",
      "magic-links",
      "two-factor-resets",
      "email-changes",
      "email-verifications",
      "oauth",
      "auth",
    ],
  ],
];

function categoryOf(resource: string): string {
  for (const [label, patterns] of CATEGORIES) {
    if (patterns.some((p) => (p.endsWith("*") ? resource.startsWith(p.slice(0, -1)) : resource === p))) {
      return label;
    }
  }
  return "More";
}

/* ------------------------------------------------------------ operations */

export type Operation = {
  id: string;
  method: string;
  path: string;
  title: string;
  href: string;
  category: string;
  resource: string;
  abilities: string[];
  raw: RawOperation;
};

const ORDER = ["index", "store", "show", "update", "destroy"];
const rank = (op: Operation) => {
  const action = op.id.split(".").at(-1) ?? "";
  const index = ORDER.indexOf(action);
  return index === -1 ? ORDER.length : index;
};

// One operation's entry, its page slug unique within its category.
function toOperation(path: string, method: string, raw: RawOperation, taken: Set<string>): Operation {
  const segments = path.split("/").filter(Boolean);
  const resource = segments[0] === "ai" && segments[1] ? `ai/${segments[1]}` : segments[0];
  const category = categoryOf(segments[0]);
  const title = raw.summary ?? titleFor(raw.operationId, path);
  let slug = kebab(title);
  if (taken.has(`${kebab(category)}/${slug}`)) {
    slug = kebab(raw.operationId);
  }
  taken.add(`${kebab(category)}/${slug}`);
  return {
    id: raw.operationId,
    method: method.toUpperCase(),
    path,
    title,
    href: `${API_ROOT}${kebab(category)}/${slug}/`,
    category,
    resource,
    abilities: raw["x-abilities"] ?? [],
    raw,
  };
}

function buildOperations(): Operation[] {
  const list: Operation[] = [];
  const taken = new Set<string>();
  for (const [path, item] of Object.entries(spec.paths)) {
    for (const [method, raw] of Object.entries(item)) {
      // x-internal marks routes built for dimmi's own app (routing.Internal):
      // they stay in the API and SDKs but not in the public reference.
      if (raw?.operationId && !raw["x-internal"]) {
        list.push(toOperation(path, method, raw, taken));
      }
    }
  }
  const categoryRank = (label: string) => {
    const index = CATEGORIES.findIndex(([name]) => name === label);
    return index === -1 ? CATEGORIES.length : index;
  };
  return list.sort(
    (a, b) =>
      categoryRank(a.category) - categoryRank(b.category) ||
      a.resource.localeCompare(b.resource) ||
      rank(a) - rank(b) ||
      a.path.length - b.path.length ||
      a.path.localeCompare(b.path),
  );
}

export const operations = buildOperations();

export function resourceLabel(resource: string): string {
  return capitalize(words(resource.replace(/^ai\//, "")));
}

/* --------------------------------------------------------------- sidebar */

export type ApiNavLink = { label: string; href: string; method?: string };
export type ApiNavGroup = { label: string; items: (ApiNavLink | ApiNavGroup)[] };

// Categories, then a group per resource when a category holds several.
export function apiNavigation(): ApiNavGroup[] {
  const categories = new Map<string, Map<string, Operation[]>>();
  for (const op of operations) {
    const resources = categories.get(op.category) ?? new Map<string, Operation[]>();
    resources.set(op.resource, [...(resources.get(op.resource) ?? []), op]);
    categories.set(op.category, resources);
  }
  const link = (op: Operation): ApiNavLink => ({ label: op.title, href: op.href, method: op.method });
  return [
    { label: "Overview", items: [{ label: "Introduction", href: API_ROOT }] },
    ...[...categories].map(([label, resources]) => ({
      label,
      items:
        resources.size === 1
          ? [...resources.values()][0].map(link)
          : [...resources].map(([resource, ops]) => ({ label: resourceLabel(resource), items: ops.map(link) })),
    })),
  ];
}

/* --------------------------------------------------------------- schemas */

const MAX_DEPTH = 4;

function deref(schema: Schema | undefined): { schema: Schema; name?: string } {
  if (!schema?.$ref) {
    return { schema: schema ?? {} };
  }
  const name = schema.$ref.split("/").at(-1) ?? "";
  return { schema: spec.components.schemas[name] ?? {}, name };
}

function types(schema: Schema): string[] {
  if (Array.isArray(schema.type)) {
    return schema.type;
  }
  if (schema.type) {
    return [schema.type];
  }
  if (schema.properties) {
    return ["object"];
  }
  const union = schema.anyOf ?? schema.oneOf;
  if (union) {
    return union.flatMap((part) => types(deref(part).schema));
  }
  return [];
}

export function typeLabel(input: Schema): string {
  const { schema } = deref(input);
  const list = types(schema).map((type) =>
    type === "array" ? `${typeLabel(schema.items ?? {}).replace(/ \| null$/, "")}[]` : type,
  );
  const unique = [...new Set(list)];
  return unique.length ? unique.join(" | ") : "any";
}

export type Field = {
  name: string;
  type: string;
  required: boolean;
  description?: string;
  options?: string[];
  defaultValue?: string;
  notes: string[];
  children: Field[];
};

function notesFor(schema: Schema): string[] {
  const notes: string[] = [];
  if (schema.format) {
    notes.push(`Format: ${schema.format}`);
  }
  if (schema.minLength !== undefined) {
    notes.push(`Minimum length: ${schema.minLength}`);
  }
  if (schema.maxLength !== undefined) {
    notes.push(`Maximum length: ${schema.maxLength}`);
  }
  if (schema.minItems !== undefined) {
    notes.push(`Minimum items: ${schema.minItems}`);
  }
  if (schema.maxItems !== undefined) {
    notes.push(`Maximum items: ${schema.maxItems}`);
  }
  if (schema.minimum !== undefined) {
    notes.push(`Minimum: ${schema.minimum}`);
  }
  if (schema.maximum !== undefined) {
    notes.push(`Maximum: ${schema.maximum}`);
  }
  return notes;
}

// The object whose properties a field shows as children, if any.
function childObject(schema: Schema): Schema | undefined {
  if (schema.properties) {
    return schema;
  }
  if (types(schema).includes("array")) {
    const items = deref(schema.items).schema;
    return items.properties ? items : undefined;
  }
  return undefined;
}

export function fieldsOf(input: Schema | undefined, depth = 0, seen: string[] = []): Field[] {
  const { schema, name } = deref(input);
  if (!schema.properties || depth > MAX_DEPTH || (name && seen.includes(name))) {
    return [];
  }
  const trail = name ? [...seen, name] : seen;
  const required = new Set(schema.required ?? []);
  return Object.entries(schema.properties).map(([key, value]) => {
    const resolved = deref(value);
    const inner = resolved.schema;
    const nested = childObject(inner);
    const itemName = types(inner).includes("array") ? deref(inner.items).name : resolved.name;
    const cycle = itemName !== undefined && trail.includes(itemName);
    return {
      name: key,
      type: typeLabel(value),
      required: required.has(key),
      description: value.description ?? inner.description,
      options: inner.enum?.map(String) ?? deref(inner.items).schema.enum?.map(String),
      defaultValue: inner.default === undefined ? undefined : JSON.stringify(inner.default),
      notes: notesFor(inner),
      children: nested && !cycle ? fieldsOf(nested, depth + 1, itemName ? [...trail, itemName] : trail) : [],
    };
  });
}

export function parameterFields(op: Operation, location: string): Field[] {
  return (op.raw.parameters ?? [])
    .filter((param) => param.in === location)
    .map((param) => ({
      name: param.name,
      type: typeLabel(param.schema ?? {}),
      required: Boolean(param.required),
      description: param.description ?? param.schema?.description,
      options: deref(param.schema).schema.enum?.map(String),
      defaultValue: param.schema?.default === undefined ? undefined : JSON.stringify(param.schema.default),
      notes: notesFor(deref(param.schema).schema),
      children: [],
    }));
}

const json = (media?: Media) => media?.["application/json"]?.schema ?? Object.values(media ?? {})[0]?.schema;

export function requestSchema(op: Operation): Schema | undefined {
  return json(op.raw.requestBody?.content);
}

const STATUS: Record<string, string> = {
  "200": "OK",
  "201": "Created",
  "202": "Accepted",
  "204": "No Content",
  default: "Error",
};

export type ApiResponse = { status: string; label: string; schema?: Schema; success: boolean };

export function responsesOf(op: Operation): ApiResponse[] {
  return Object.entries(op.raw.responses).map(([status, response]) => ({
    status,
    label: response.description ?? STATUS[status] ?? status,
    schema: json(response.content),
    success: status.startsWith("2"),
  }));
}

/* -------------------------------------------------------------- examples */

function exampleString(schema: Schema): string {
  switch (schema.format) {
    case "date-time":
      return "2026-11-01T09:30:00Z";
    case "date":
      return "2026-11-01";
    case "email":
      return "ada@example.com";
    case "uri":
    case "url":
      return "https://example.com";
    default:
      return "<string>";
  }
}

function exampleObject(schema: Schema, depth: number, trail: string[]): unknown {
  if (schema.properties) {
    return Object.fromEntries(
      Object.entries(schema.properties).map(([key, value]) => [key, example(value, depth + 1, trail)]),
    );
  }
  return typeof schema.additionalProperties === "object"
    ? { "<key>": example(schema.additionalProperties, depth + 1, trail) }
    : {};
}

export function example(input: Schema | undefined, depth = 0, seen: string[] = []): unknown {
  const { schema, name } = deref(input);
  if (name && seen.includes(name)) {
    return {};
  }
  const trail = name ? [...seen, name] : seen;
  if (schema.enum?.length) {
    return schema.enum[0];
  }
  if (schema.default !== undefined && schema.default !== null) {
    return schema.default;
  }
  const type = types(schema).find((t) => t !== "null") ?? (schema.properties ? "object" : undefined);
  switch (type) {
    case "object":
      return depth > MAX_DEPTH ? {} : exampleObject(schema, depth, trail);
    case "array":
      return depth > MAX_DEPTH ? [] : [example(schema.items, depth + 1, trail)];
    case "string":
      return exampleString(schema);
    case "integer":
      return 123;
    case "number":
      return 123.45;
    case "boolean":
      return true;
    default:
      return null;
  }
}

/* ---------------------------------------------------------- code samples */

export function exampleBody(op: Operation): string | undefined {
  const schema = requestSchema(op);
  return schema ? JSON.stringify(example(schema), null, 2) : undefined;
}

function url(op: Operation): string {
  const required = (op.raw.parameters ?? []).filter((param) => param.in === "query" && param.required);
  const query = required.map((param) => `${param.name}=<${param.name}>`).join("&");
  return `${BASE_URL}${op.path}${query ? `?${query}` : ""}`;
}

function python(value: unknown, indent = 0): string {
  const pad = " ".repeat(indent + 4);
  if (value === null) {
    return "None";
  }
  if (typeof value === "boolean") {
    return value ? "True" : "False";
  }
  if (Array.isArray(value)) {
    return value.length
      ? `[\n${value.map((v) => pad + python(v, indent + 4)).join(",\n")}\n${" ".repeat(indent)}]`
      : "[]";
  }
  if (typeof value === "object") {
    const entries = Object.entries(value as Record<string, unknown>);
    return entries.length
      ? `{\n${entries.map(([k, v]) => `${pad}${JSON.stringify(k)}: ${python(v, indent + 4)}`).join(",\n")}\n${" ".repeat(indent)}}`
      : "{}";
  }
  return JSON.stringify(value);
}

export type Sample = { label: string; lang: string; code: string };

export function samples(op: Operation): Sample[] {
  const body = exampleBody(op);
  const target = url(op);
  const value = body ? JSON.parse(body) : undefined;

  const curl = [
    `curl --request ${op.method} \\`,
    `  --url '${target}' \\`,
    `  --header 'Authorization: Bearer <token>'${body ? " \\" : ""}`,
    ...(body ? ["  --header 'Content-Type: application/json' \\", `  --data '${body.replaceAll("'", "'\\''")}'`] : []),
  ].join("\n");

  const js = [
    `const response = await fetch("${target}", {`,
    `  method: "${op.method}",`,
    "  headers: {",
    '    Authorization: "Bearer <token>",',
    ...(body ? ['    "Content-Type": "application/json",'] : []),
    "  },",
    ...(body ? [`  body: JSON.stringify(${body.replaceAll("\n", "\n  ")}),`] : []),
    "});",
    "",
    "const data = await response.json();",
  ].join("\n");

  const py = [
    "import requests",
    "",
    `response = requests.${op.method.toLowerCase()}(`,
    `    "${target}",`,
    '    headers={"Authorization": "Bearer <token>"},',
    ...(value === undefined ? [] : [`    json=${python(value, 4)},`]),
    ")",
    "",
    "print(response.json())",
  ].join("\n");

  const go = [
    "package main",
    "",
    "import (",
    '\t"fmt"',
    '\t"io"',
    '\t"net/http"',
    ...(body ? ['\t"strings"'] : []),
    ")",
    "",
    "func main() {",
    ...(body ? [`\tpayload := strings.NewReader(\`${body.replaceAll("\n", "\n\t")}\`)`, ""] : []),
    `\treq, _ := http.NewRequest("${op.method}", "${target}", ${body ? "payload" : "nil"})`,
    '\treq.Header.Add("Authorization", "Bearer <token>")',
    ...(body ? ['\treq.Header.Add("Content-Type", "application/json")'] : []),
    "",
    "\tres, _ := http.DefaultClient.Do(req)",
    "\tdefer res.Body.Close()",
    "\tbody, _ := io.ReadAll(res.Body)",
    "",
    "\tfmt.Println(string(body))",
    "}",
  ].join("\n");

  return [
    { label: "cURL", lang: "bash", code: curl },
    { label: "JavaScript", lang: "js", code: js },
    { label: "Python", lang: "python", code: py },
    { label: "Go", lang: "go", code: go },
  ];
}

/* -------------------------------------------------------------- markdown */

function fieldsMarkdown(fields: Field[], indent = ""): string[] {
  return fields.flatMap((field) => [
    `${indent}- \`${field.name}\` (${field.type}${field.required ? ", required" : ""})${field.description ? `: ${field.description}` : ""}${field.options ? ` One of: ${field.options.map((o) => `\`${o}\``).join(", ")}.` : ""}`,
    ...fieldsMarkdown(field.children, `${indent}  `),
  ]);
}

export function operationMarkdown(op: Operation): string {
  const lines = [`# ${op.title}`, "", "```http", `${op.method} ${BASE_URL}${op.path}`, "```", ""];
  lines.push("## Authorization", "", "Send an API token as `Authorization: Bearer <token>`.");
  if (op.abilities.length) {
    lines.push(`The token needs: ${op.abilities.map((a) => `\`${a}\``).join(", ")}.`);
  }
  lines.push("");
  for (const [location, heading] of [
    ["path", "Path parameters"],
    ["query", "Query parameters"],
    ["header", "Headers"],
  ]) {
    const fields = parameterFields(op, location);
    if (fields.length) {
      lines.push(`## ${heading}`, "", ...fieldsMarkdown(fields), "");
    }
  }
  const body = requestSchema(op);
  if (body) {
    lines.push("## Body", "", ...fieldsMarkdown(fieldsOf(body)), "");
  }
  for (const response of responsesOf(op)) {
    lines.push(`## Response ${response.status} (${response.label})`, "");
    if (response.schema) {
      lines.push(...fieldsMarkdown(fieldsOf(response.schema)), "");
    }
  }
  lines.push("## Example request", "", "```bash", samples(op)[0].code, "```", "");
  const success = responsesOf(op).find((r) => r.success && r.schema);
  if (success) {
    lines.push("## Example response", "", "```json", JSON.stringify(example(success.schema), null, 2), "```", "");
  }
  return lines.join("\n");
}
