/*
 * The action editor's working copy. The page edits rows and text (input
 * rows, query and header pairs, the body as typed) and turns them into
 * the definition the server stores only when saving, so a half-typed
 * body never throws away what was written. A new action starts with the
 * safe defaults: a lookup the customer never has to confirm, or, once
 * marked as changing data, one the customer confirms first. Problems
 * name the field they belong to, so the page shows each under its field.
 */
import {
  ACTION_AGENTS,
  ACTION_CHANNELS,
  type ActionAgent,
  type ActionChannel,
  type ActionConnection,
  type ActionDefinition,
  type AiAction,
  type Approval,
  type Confirmation,
  type HttpMethod,
  type InputSchema,
  type SideEffect,
} from "./api";
import {
  fieldsFromSchema,
  type InputField,
  nameProblem,
  schemaFromFields,
  unknownPlaceholder,
  usesIdentity,
} from "./schema";

export interface Pair {
  key: string;
  name: string;
  value: string;
}

export interface Draft {
  connection_id: string;
  name: string;
  title: string;
  description: string;
  side_effect: SideEffect;
  method: HttpMethod;
  path: string;
  query: Pair[];
  headers: Pair[];
  body: string;
  body_encoding: "json" | "form";
  tool: string;
  bound: Pair[];
  fields: InputField[];
  response: Pair[];
  channels: ActionChannel[];
  agents: ActionAgent[];
  confirmation: Confirmation;
  approval: Approval;
  approval_timeout_minutes: number;
}

let made = 0;

/*
 * rowKey names a new row so React keeps it in place while rows above it
 * come and go.
 */
export function rowKey(): string {
  made += 1;
  return `row-${made}`;
}

function pairs(record: Record<string, string>): Pair[] {
  return Object.entries(record).map(([name, value]) => ({ key: rowKey(), name, value }));
}

function record(rows: Pair[]): Record<string, string> {
  return Object.fromEntries(rows.filter((row) => row.name.trim()).map((row) => [row.name.trim(), row.value]));
}

/*
 * newDraft starts an action on a connection.
 */
export function newDraft(connection: Pick<ActionConnection, "id" | "kind">): Draft {
  return {
    connection_id: connection.id,
    name: "",
    title: "",
    description: "",
    side_effect: "read",
    method: "GET",
    path: "/",
    query: [],
    headers: [],
    body: "",
    body_encoding: "json",
    tool: "",
    bound: [],
    fields: [],
    response: [],
    channels: ["widget"],
    agents: ["dimmi"],
    confirmation: "none",
    approval: "none",
    approval_timeout_minutes: 60,
  };
}

/*
 * Stored is an action as the server answers it, or as the editor saved
 * it: the fields the working copy is made from.
 */
type Stored = Pick<
  AiAction,
  | "connection_id"
  | "name"
  | "title"
  | "description"
  | "side_effect"
  | "confirmation"
  | "approval"
  | "approval_timeout_minutes"
  | "body_encoding"
  | "response_fields"
> & {
  body_template?: unknown;
  method?: HttpMethod | null;
  path_template?: string | null;
  query?: Record<string, string>;
  headers?: Record<string, string>;
  mcp_tool_name?: string | null;
  bound_arguments?: Record<string, unknown>;
  input_schema?: Record<string, unknown>;
  channels?: string[];
  agents?: string[];
};

/*
 * draftOf turns a stored action into the editor's working copy.
 */
export function draftOf(action: Stored): Draft {
  return {
    connection_id: action.connection_id,
    name: action.name,
    title: action.title,
    description: action.description,
    side_effect: action.side_effect,
    ...requestOf(action),
    fields: fieldsFromSchema(schemaOf(action.input_schema)),
    response: (action.response_fields ?? []).map((field) => ({ key: rowKey(), name: field.name, value: field.path })),
    channels: channelsOf(action.channels),
    agents: agentsOf(action.agents),
    ...permissionsOf(action),
  };
}

function schemaOf(schema: Record<string, unknown> | undefined): InputSchema {
  const properties = schema?.properties;
  if (!properties || typeof properties !== "object") {
    return { type: "object", properties: {} };
  }

  return schema as InputSchema;
}

function channelsOf(channels: string[] | undefined): ActionChannel[] {
  const known = (channels ?? ["widget"]).filter((c): c is ActionChannel =>
    (ACTION_CHANNELS as readonly string[]).includes(c),
  );

  return known.length > 0 ? known : ["widget"];
}

function agentsOf(agents: string[] | undefined): ActionAgent[] {
  return (agents ?? ["dimmi"]).filter((a): a is ActionAgent => (ACTION_AGENTS as readonly string[]).includes(a));
}

function strings(record: Record<string, unknown> | undefined): Record<string, string> {
  return Object.fromEntries(Object.entries(record ?? {}).map(([key, value]) => [key, String(value)]));
}

function requestOf(
  action: Stored,
): Pick<Draft, "method" | "path" | "query" | "headers" | "body" | "body_encoding" | "tool" | "bound"> {
  const body = action.body_template;

  return {
    method: action.method ?? "GET",
    path: action.path_template ?? "/",
    query: pairs(action.query ?? {}),
    headers: pairs(action.headers ?? {}),
    body: body === null || body === undefined ? "" : JSON.stringify(body, null, 2),
    body_encoding: action.body_encoding ?? "json",
    tool: action.mcp_tool_name ?? "",
    bound: pairs(strings(action.bound_arguments)),
  };
}

function permissionsOf(action: Stored): Pick<Draft, "confirmation" | "approval" | "approval_timeout_minutes"> {
  return {
    confirmation: action.confirmation ?? (action.side_effect === "write" ? "customer" : "none"),
    approval: action.approval ?? "none",
    approval_timeout_minutes: action.approval_timeout_minutes ?? 60,
  };
}

/*
 * withSideEffect changes what the action does to data, moving its
 * permissions to the safe default: a write starts with the customer
 * confirming and runs only in the messenger, a read needs neither.
 */
export function withSideEffect(draft: Draft, side: SideEffect): Draft {
  if (side === draft.side_effect) {
    return draft;
  }
  if (side === "write") {
    return { ...draft, side_effect: side, confirmation: "customer", channels: ["widget"] };
  }

  return { ...draft, side_effect: side, confirmation: "none", approval: "none" };
}

function bodyOf(draft: Draft): unknown {
  if (!draft.body.trim() || draft.method === "GET") {
    return null;
  }
  try {
    return JSON.parse(draft.body);
  } catch {
    return draft.body;
  }
}

/*
 * definitionOf turns the working copy into what the server stores.
 */
export function definitionOf(draft: Draft, kind: "http" | "mcp"): ActionDefinition {
  const target =
    kind === "http"
      ? {
          method: draft.method,
          path_template: draft.path.trim(),
          query: record(draft.query),
          headers: record(draft.headers),
          body_template: bodyOf(draft),
        }
      : { mcp_tool_name: draft.tool.trim(), bound_arguments: record(draft.bound) };

  return {
    connection_id: draft.connection_id,
    name: draft.name.trim(),
    title: draft.title.trim(),
    description: draft.description.trim(),
    side_effect: draft.side_effect,
    ...target,
    body_encoding: draft.body_encoding,
    input_schema: schemaFromFields(draft.fields),
    response_fields: draft.response
      .filter((row) => row.value.trim())
      .map((row) => ({ name: row.name.trim() || row.value.trim(), path: row.value.trim() })),
    channels: draft.channels,
    agents: draft.agents,
    confirmation: draft.side_effect === "write" ? draft.confirmation : "none",
    approval: draft.side_effect === "write" ? draft.approval : "none",
    approval_timeout_minutes: draft.approval_timeout_minutes,
  };
}

export type Problems = Partial<Record<string, string>>;

/*
 * problemsOf checks the working copy the way the server will, so the
 * admin sees what to fix beside each field before saving.
 */
export function problemsOf(draft: Draft, connection: Pick<ActionConnection, "kind" | "variables">): Problems {
  const problems: Problems = {};
  const name = nameProblem(draft.name);
  if (name) {
    problems.name = name;
  }
  if (!draft.title.trim()) {
    problems.title = "Give the action a title teammates and customers see.";
  }
  if (draft.description.trim().length < 20) {
    problems.description = "Tell the agent when to use this and what it returns, in a sentence or two.";
  }
  Object.assign(problems, requestProblems(draft, connection), inputProblems(draft));
  if (draft.channels.length === 0) {
    problems.channels = "Pick at least one channel.";
  }

  return problems;
}

function requestProblems(draft: Draft, connection: Pick<ActionConnection, "kind" | "variables">): Problems {
  if (connection.kind === "mcp") {
    return toolProblems(draft);
  }
  const known = { inputs: draft.fields.map((field) => field.name), variables: Object.keys(connection.variables) };
  const problems: Problems = {};
  if (!draft.path.startsWith("/")) {
    problems.path = "The path starts with /.";
  }
  const templates = [draft.path, ...draft.query.map((row) => row.value), draft.body];
  const unknown = templates.map((text) => unknownPlaceholder(text, known)).find(Boolean);
  if (unknown) {
    problems.path = `{${unknown}} does not name an input, a customer field or a connection variable.`;
  }
  if (draft.side_effect === "write" && !usesIdentity([...templates, ...draft.headers.map((row) => row.value)])) {
    problems.identity = "Bind the call to the signed-in customer, for example with {contact.external_id}.";
  }

  return problems;
}

function toolProblems(draft: Draft): Problems {
  if (!draft.tool.trim()) {
    return { tool: "Name the tool on the MCP server." };
  }
  if (draft.side_effect === "write" && !usesIdentity(draft.bound.map((row) => row.value))) {
    return { identity: "Bind one of the tool's arguments to the signed-in customer, such as their external ID." };
  }

  return {};
}

function inputProblems(draft: Draft): Problems {
  const names = draft.fields.map((field) => field.name.trim());
  const bad = names.find((name) => !/^[a-z][a-z0-9_]{0,63}$/.test(name));
  if (bad !== undefined) {
    return { fields: bad ? `“${bad}” must be lowercase letters, digits and underscores.` : "Name every input." };
  }
  const twice = names.find((name, index) => names.indexOf(name) !== index);

  return twice ? { fields: `Two inputs are called “${twice}”.` } : {};
}

/*
 * changed says the working copy differs from what was last saved.
 */
export function changed(draft: Draft, saved: AiAction | null, kind: "http" | "mcp"): boolean {
  if (!saved) {
    return true;
  }

  return JSON.stringify(definitionOf(draft, kind)) !== JSON.stringify(definitionOf(draftOf(saved), kind));
}
