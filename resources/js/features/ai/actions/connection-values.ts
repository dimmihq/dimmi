/*
 * A connection's details as its form edits them, how they are checked
 * before anything is sent, and the body they become. A new connection
 * needs its secret; a stored one keeps its own when the field stays
 * empty, since secrets are write-only and never come back.
 */
import { RiGlobalLine, RiPlugLine } from "@remixicon/react";

import { ApiError } from "@/lib/api/client";

import type { ActionConnection, AuthType, ConnectionKind, NewConnection } from "./api";
import { type Pair, rowKey } from "./draft";

export const kindInfo: Record<
  ConnectionKind,
  { name: string; icon: typeof RiGlobalLine; tint: "blue" | "purple"; blurb: string }
> = {
  http: {
    name: "REST API",
    icon: RiGlobalLine,
    tint: "blue",
    blurb: "Any HTTPS API: you describe each request, dimmi fills it in.",
  },
  mcp: {
    name: "MCP server",
    icon: RiPlugLine,
    tint: "purple",
    blurb: "A remote server over Streamable HTTP. Its tools become actions.",
  },
};

export const authChoices: Record<ConnectionKind, AuthType[]> = {
  http: ["none", "bearer", "api_key_header", "api_key_query", "basic"],
  mcp: ["none", "bearer", "api_key_header"],
};

export interface ConnectionValues {
  name: string;
  url: string;
  auth: AuthType;
  header: string;
  param: string;
  token: string;
  username: string;
  password: string;
  headers: Pair[];
  variables: Pair[];
  timeout: number;
}

export type ConnectionProblems = Partial<Record<keyof ConnectionValues | "form", string>>;

function pairs(record: Record<string, string>): Pair[] {
  return Object.entries(record).map(([name, value]) => ({ key: rowKey(), name, value }));
}

function record(rows: Pair[]): Record<string, string> {
  return Object.fromEntries(rows.filter((row) => row.name.trim()).map((row) => [row.name.trim(), row.value]));
}

/*
 * initialValues fills the form from a stored connection, or with the
 * usual defaults for a new one.
 */
export function initialValues(kind: ConnectionKind, connection?: ActionConnection): ConnectionValues {
  return {
    name: connection?.name ?? "",
    url: connection?.base_url ?? "",
    auth: connection?.auth_type ?? "bearer",
    header: connection?.auth_config.header_name ?? (kind === "mcp" ? "X-API-Key" : ""),
    param: connection?.auth_config.query_param ?? "api_key",
    token: "",
    username: "",
    password: "",
    headers: pairs(connection?.default_headers ?? {}),
    variables: pairs(connection?.variables ?? {}),
    timeout: connection?.timeout_ms ?? 8000,
  };
}

/*
 * checkConnection finds what is missing before anything is sent.
 */
export function checkConnection(values: ConnectionValues, editing: boolean): ConnectionProblems {
  const problems: ConnectionProblems = {};
  if (!values.name.trim()) {
    problems.name = "Give the connection a name.";
  }
  if (!/^https?:\/\/[^\s/]+/.test(values.url.trim())) {
    problems.url = "Enter the full address, starting with https://.";
  }

  return { ...problems, ...secretProblems(values, editing) };
}

function secretProblems(values: ConnectionValues, editing: boolean): ConnectionProblems {
  const problems: ConnectionProblems = {};
  const secret = values.auth === "basic" ? values.password : values.token;
  if (values.auth !== "none" && !editing && !secret.trim()) {
    problems[values.auth === "basic" ? "password" : "token"] = "Paste the secret dimmi signs in with.";
  }
  if (values.auth === "basic" && !editing && !values.username.trim()) {
    problems.username = "Enter the username.";
  }
  if (values.auth === "api_key_header" && !/^[A-Za-z0-9-]{1,64}$/.test(values.header)) {
    problems.header = "Use letters, digits and dashes, such as X-API-Key.";
  }

  return problems;
}

function authConfig(values: ConnectionValues): ActionConnection["auth_config"] {
  if (values.auth === "api_key_header") {
    return { header_name: values.header.trim() };
  }
  if (values.auth === "api_key_query") {
    return { query_param: values.param.trim() };
  }

  return {};
}

/*
 * connectionBody is what saving sends. Secrets are left out when empty
 * so a stored one stays.
 */
export function connectionBody(kind: ConnectionKind, values: ConnectionValues): NewConnection {
  return {
    name: values.name.trim(),
    kind,
    base_url: values.url.trim().replace(/\/+$/, ""),
    auth_type: values.auth,
    auth_config: authConfig(values),
    ...(values.token.trim() ? { token: values.token.trim() } : {}),
    ...(values.username.trim() ? { username: values.username.trim() } : {}),
    ...(values.password ? { password: values.password } : {}),
    ...(kind === "http"
      ? { default_headers: record(values.headers), variables: record(values.variables), timeout_ms: values.timeout }
      : {}),
  };
}

/*
 * connectionRefusal places the server's refusal beside the field it is
 * about, or above the buttons when it is about none.
 */
export function connectionRefusal(error: unknown): ConnectionProblems {
  if (!(error instanceof ApiError)) {
    return { form: "dimmi could not reach the server. Check your connection and try again." };
  }
  const map: Record<string, keyof ConnectionValues> = { name: "name", base_url: "url", token: "token" };
  const problems: ConnectionProblems = {};
  for (const [field, message] of Object.entries(error.fields)) {
    const key = map[field];
    if (key) {
      problems[key] = message;
    }
  }

  return Object.keys(problems).length > 0 ? problems : { form: error.message };
}
