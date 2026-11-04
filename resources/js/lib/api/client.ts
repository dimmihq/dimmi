/*
 * The one way the browser app talks to the API. Requests go to the same
 * origin with the session cookie, so the server's cross-origin
 * protection lets them through, and every failure comes back as an
 * ApiError carrying the server's stable code and its message, already in
 * the reader's language.
 */

const base = "/api/v1";

/*
 * ApiError is a request the server refused. Code is stable for programs;
 * message is meant for people. Fields maps each rejected field to its
 * first message.
 */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly fields: Record<string, string>;

  constructor(status: number, code: string, message: string, fields: Record<string, string> = {}) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.fields = fields;
  }
}

interface ErrorBody {
  code?: string;
  message?: string;
  errors?: Record<string, string[]>;
}

/*
 * toApiError reads the error envelope of a failed response. A body that
 * is not the envelope, such as a proxy's error page, still becomes an
 * ApiError so callers handle one kind of failure.
 */
function toApiError(status: number, body: ErrorBody | null): ApiError {
  const fields: Record<string, string> = {};

  for (const [field, messages] of Object.entries(body?.errors ?? {})) {
    if (messages[0]) {
      fields[field] = messages[0];
    }
  }

  return new ApiError(
    status,
    body?.code ?? "unexpected_response",
    body?.message ?? "Something went wrong. Try again.",
    fields,
  );
}

/*
 * RequestOptions changes one call: workspace names the workspace a
 * workspace route works in, sent in the Dimmi-Workspace header, and
 * headers adds any other header the call needs, such as a token that
 * must never sit in an address.
 */
export interface RequestOptions {
  workspace?: string;
  headers?: Record<string, string>;
}

/*
 * request sends one API call and returns the data of the response, or
 * undefined for a response without a body.
 */
export async function request<T>(
  method: string,
  path: string,
  body?: unknown,
  options: RequestOptions = {},
): Promise<T> {
  const headers: Record<string, string> = { ...options.headers, Accept: "application/json" };
  if (body !== undefined) {
    headers["Content-Type"] = "application/json";
  }
  if (options.workspace) {
    headers["Dimmi-Workspace"] = options.workspace;
  }

  const response = await fetch(base + path, {
    method,
    headers,
    credentials: "same-origin",
    body: body === undefined ? undefined : JSON.stringify(body),
  });

  if (response.status === 204) {
    return undefined as T;
  }

  const json: unknown = await response.json().catch(() => null);
  if (!response.ok) {
    throw toApiError(response.status, json as ErrorBody | null);
  }

  return (json as { data: T }).data;
}

/*
 * Page is one page of a list and the cursor of the next, null on the
 * last page.
 */
export interface Page<T> {
  items: T[];
  next: string | null;
}

/*
 * requestPage reads one page of a list endpoint, keeping the cursor of
 * the next page the server puts beside the data.
 */
export async function requestPage<T>(path: string, options: RequestOptions = {}): Promise<Page<T>> {
  const headers: Record<string, string> = { Accept: "application/json" };
  if (options.workspace) {
    headers["Dimmi-Workspace"] = options.workspace;
  }

  const response = await fetch(base + path, { headers, credentials: "same-origin" });
  const json: unknown = await response.json().catch(() => null);
  if (!response.ok) {
    throw toApiError(response.status, json as ErrorBody | null);
  }

  const body = json as { data?: T[]; meta?: { next_cursor?: string | null } } | null;

  return { items: body?.data ?? [], next: body?.meta?.next_cursor ?? null };
}
