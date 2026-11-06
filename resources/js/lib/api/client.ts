/*
 * The one way the browser app talks to the API. Requests go to the same
 * origin with the session cookie, so the server's cross-origin
 * protection lets them through, and every failure comes back as an
 * ApiError carrying the server's stable code and its message, already in
 * the reader's language: every request names the language the app reads
 * in, so the server writes its messages in the same one.
 */
import { activeLocale, i18n } from "@/lib/i18n";

import { reportAccessRequirement } from "./access-required";

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
  requirement?: string;
  action_url?: string | null;
  ip?: string;
}

/*
 * reportRefusal hands a workspace's refusal over to the screen that
 * explains it, whatever page the request came from.
 */
function reportRefusal(status: number, body: ErrorBody | null) {
  if (status === 403 && body?.requirement) {
    reportAccessRequirement({
      requirement: body.requirement,
      message: body.message ?? "This workspace needs something from you.",
      actionUrl: body.action_url ?? null,
      ip: body.ip,
    });
  }
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
    body?.message ?? i18n.t("common:errors.unexpected"),
    fields,
  );
}

/*
 * RequestOptions changes one call: workspace names the workspace a
 * workspace route works in, sent in the Dimmi-Workspace header, and
 * headers adds any other header the call needs, such as a token that
 * must never sit in an address. keepalive lets the call finish after
 * the page goes away, and signal cancels it.
 */
export interface RequestOptions {
  workspace?: string;
  headers?: Record<string, string>;
  keepalive?: boolean;
  signal?: AbortSignal;
}

/*
 * longestWait is the longest a read waits for the rate limit to open
 * again before it is tried once more; a longer wait fails at once.
 */
const longestWait = 10;

/*
 * send makes the call. A read refused for going too fast is tried once
 * more after the wait the server named, when that wait is short; a write
 * never is, so it cannot happen twice, and its refusal reaches the page.
 */
async function send(url: string, init: RequestInit): Promise<Response> {
  const response = await fetch(url, init);
  const method = (init.method ?? "GET").toUpperCase();
  const wait = Number(response.headers.get("Retry-After"));

  if (response.status !== 429 || method !== "GET" || !(wait > 0 && wait <= longestWait)) {
    return response;
  }

  await new Promise((resolve) => setTimeout(resolve, wait * 1000));

  return fetch(url, init);
}

/*
 * request sends one API call and returns the data of the response, or
 * undefined for a response without a body. A form or a file is sent as it
 * is; anything else as JSON.
 */
export async function request<T>(
  method: string,
  path: string,
  body?: unknown,
  options: RequestOptions = {},
): Promise<T> {
  const headers: Record<string, string> = {
    ...options.headers,
    Accept: "application/json",
    "Accept-Language": activeLocale(),
  };
  const isRaw = body instanceof FormData || body instanceof Blob;
  if (body !== undefined && !isRaw) {
    headers["Content-Type"] = "application/json";
  }
  if (options.workspace) {
    headers["Dimmi-Workspace"] = options.workspace;
  }

  const response = await send(base + path, {
    method,
    headers,
    credentials: "same-origin",
    body: isRaw ? body : body === undefined ? undefined : JSON.stringify(body),
    keepalive: options.keepalive,
    signal: options.signal,
  });

  if (response.status === 204) {
    return undefined as T;
  }

  const json: unknown = await response.json().catch(() => null);
  if (!response.ok) {
    reportRefusal(response.status, json as ErrorBody | null);
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
  const headers: Record<string, string> = { Accept: "application/json", "Accept-Language": activeLocale() };
  if (options.workspace) {
    headers["Dimmi-Workspace"] = options.workspace;
  }

  const response = await send(base + path, { headers, credentials: "same-origin" });
  const json: unknown = await response.json().catch(() => null);
  if (!response.ok) {
    reportRefusal(response.status, json as ErrorBody | null);
    throw toApiError(response.status, json as ErrorBody | null);
  }

  const body = json as { data?: T[]; meta?: { next_cursor?: string | null } } | null;

  return { items: body?.data ?? [], next: body?.meta?.next_cursor ?? null };
}

/*
 * DownloadedFile is a file the server sent as an attachment, with the
 * name it suggested.
 */
export interface DownloadedFile {
  blob: Blob;
  filename: string;
}

/*
 * requestFile fetches a file, such as an export, asking for its media
 * type. A refusal still arrives as the JSON error envelope.
 */
export async function requestFile(path: string, accept: string, options: RequestOptions = {}): Promise<DownloadedFile> {
  const headers: Record<string, string> = { Accept: accept };
  if (options.workspace) {
    headers["Dimmi-Workspace"] = options.workspace;
  }

  const response = await fetch(base + path, { headers, credentials: "same-origin" });
  if (!response.ok) {
    const json: unknown = await response.json().catch(() => null);
    throw toApiError(response.status, json as ErrorBody | null);
  }

  const disposition = response.headers.get("Content-Disposition") ?? "";
  const filename = /filename="([^"]+)"/.exec(disposition)?.[1] ?? "download";

  return { blob: await response.blob(), filename };
}
