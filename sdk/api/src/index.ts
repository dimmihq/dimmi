/*
 * @dimmi/api is a typed client for the dimmi REST API: openapi-fetch over
 * types openapi-typescript generated from the API's OpenAPI document. It
 * runs anywhere fetch does. createDimmiApi adds the token, the
 * Dimmi-Workspace header, an Idempotency-Key on POSTs when asked, and
 * sends a call answered 429 again after the server's Retry-After.
 * paginate() walks every item of a list, by cursor or by offset.
 *
 *   const api = createDimmiApi({ host, token, workspace });
 *   const { data, error } = await api.GET("/contacts/{contact}", { params: { path: { contact } } });
 */
import createClient, { type Client, type Middleware } from "openapi-fetch";

import { paging } from "./paging.js";
import type { components, paths } from "./schema.js";

export { API_VERSION } from "./paging.js";
export type { components, operations, paths } from "./schema.js";

export type Schemas = components["schemas"];
export type DimmiApi = Client<paths>;

export interface DimmiApiOptions {
  /* A personal access token or an app's access token. */
  token: string;
  /* Your dimmi host, such as https://dimmi.acme.com; the API is under /api/v1. */
  host?: string;
  /* The API's full base URL, instead of host. */
  baseUrl?: string;
  /* The workspace calls work in, sent as Dimmi-Workspace. */
  workspace?: string;
  /* How many times a call answered 429 is sent again; 3 unless set. */
  maxRetries?: number;
  /* The longest wait before a retry, in milliseconds; 30 s unless set. */
  maxRetryDelay?: number;
  /* Adds an Idempotency-Key to every POST without one; retries reuse it. */
  autoIdempotency?: boolean;
  fetch?: (request: Request) => Promise<Response>;
  headers?: Record<string, string>;
}

/*
 * DimmiApiError is what paginate and unwrap throw for a failed call,
 * with the server's error code and per-field messages.
 */
export class DimmiApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly errors: Record<string, string[] | null>;
  readonly body: unknown;

  constructor(status: number, body: unknown) {
    const parsed = (body && typeof body === "object" ? body : {}) as Partial<Schemas["ResponseError"]>;
    super(parsed.message ?? `dimmi API answered ${status}`);
    this.name = "DimmiApiError";
    this.status = status;
    this.code = parsed.code ?? "http_error";
    this.errors = parsed.errors ?? {};
    this.body = body;
  }
}

/*
 * retryAfter reads a Retry-After header, in seconds or as an HTTP date,
 * as milliseconds.
 */
export function retryAfter(value: string | null, now = Date.now()): number | undefined {
  if (!value) {
    return undefined;
  }
  const seconds = Number(value);
  if (Number.isFinite(seconds)) {
    return Math.max(0, seconds * 1000);
  }
  const date = Date.parse(value);
  return Number.isNaN(date) ? undefined : Math.max(0, date - now);
}

const sleep = (ms: number, signal?: AbortSignal | null) =>
  new Promise<void>((resolve, reject) => {
    const timer = setTimeout(resolve, ms);
    signal?.addEventListener(
      "abort",
      () => {
        clearTimeout(timer);
        reject(signal.reason);
      },
      { once: true },
    );
  });

/*
 * retrying wraps fetch so a 429 is sent again after its Retry-After.
 */
export function retrying(
  send: (request: Request) => Promise<Response>,
  maxRetries = 3,
  maxDelay = 30_000,
): (request: Request) => Promise<Response> {
  return async (request) => {
    for (let attempt = 0; ; attempt++) {
      const response = await send(attempt < maxRetries ? request.clone() : request);
      if (response.status !== 429 || attempt >= maxRetries) {
        return response;
      }
      const wait = retryAfter(response.headers.get("retry-after")) ?? 2 ** attempt * 500;
      await sleep(Math.min(wait, maxDelay), request.signal);
    }
  };
}

/*
 * createDimmiApi answers an openapi-fetch client for one token.
 */
export function createDimmiApi(options: DimmiApiOptions): DimmiApi {
  if (!options.token) {
    throw new Error("dimmi: createDimmiApi needs a token.");
  }
  const baseUrl = options.baseUrl ?? (options.host ? `${options.host.replace(/\/+$/, "")}/api/v1` : "");
  if (!baseUrl) {
    throw new Error("dimmi: createDimmiApi needs host or baseUrl.");
  }
  const send = options.fetch ?? ((request: Request) => globalThis.fetch(request));
  const client = createClient<paths>({
    baseUrl,
    fetch: retrying(send, options.maxRetries, options.maxRetryDelay),
    headers: options.headers,
  });
  const auth: Middleware = {
    onRequest({ request }) {
      request.headers.set("Authorization", `Bearer ${options.token}`);
      if (options.workspace && !request.headers.has("Dimmi-Workspace")) {
        request.headers.set("Dimmi-Workspace", options.workspace);
      }
      if (options.autoIdempotency && request.method === "POST" && !request.headers.has("Idempotency-Key")) {
        request.headers.set("Idempotency-Key", globalThis.crypto.randomUUID());
      }
      return request;
    },
  };
  client.use(auth);
  return client;
}

/*
 * unwrap answers a call's data, or throws its error as a DimmiApiError.
 */
export async function unwrap<T>(call: Promise<{ data?: T; error?: unknown; response: Response }>): Promise<T> {
  const { data, error, response } = await call;
  if (error !== undefined || !response.ok) {
    throw new DimmiApiError(response.status, error);
  }
  return data as T;
}

type GetPaths = {
  [P in keyof paths]: paths[P] extends { get: { responses: { 200: unknown } } } ? P : never;
}[keyof paths];
type ListBody<P extends GetPaths> = paths[P]["get"]["responses"][200] extends {
  content: { "application/json": infer B };
}
  ? B
  : never;
type ListPaths = { [P in GetPaths]: ListBody<P> extends { data: unknown; meta: unknown } ? P : never }[GetPaths];
type ItemOf<P extends ListPaths> = ListBody<P> extends { data: Array<infer I> | null } ? I : never;
type Query = Record<string, unknown>;
type ListInit = { params?: { path?: Record<string, unknown>; query?: Query; header?: Record<string, unknown> } };
type Page = { data?: unknown[] | null; meta?: { next_cursor?: string; total?: number; per_page?: number } };

/*
 * pages walks every page of a list, following its cursor, its after
 * cursor, or its offset.
 */
export async function* pages<P extends ListPaths>(
  api: DimmiApi,
  path: P,
  init: ListInit = {},
): AsyncGenerator<ListBody<P>, void, undefined> {
  const query: Query = { ...init.params?.query };
  const kind = paging[path as string] ?? "cursor";
  const get = api.GET as unknown as (
    path: string,
    init: ListInit,
  ) => Promise<{ data?: Page; error?: unknown; response: Response }>;
  for (;;) {
    const page = await unwrap(get(path as string, { ...init, params: { ...init.params, query } }));
    yield page as ListBody<P>;
    const data = page.data ?? [];
    const next = page.meta?.next_cursor;
    if (next && kind !== "offset") {
      query[kind] = next;
      continue;
    }
    if (kind !== "offset" || data.length === 0) {
      return;
    }
    const offset = Number(query.offset ?? 0) + data.length;
    const total = page.meta?.total;
    const limit = Number(query.limit ?? page.meta?.per_page ?? data.length);
    if ((total !== undefined && offset >= total) || data.length < limit) {
      return;
    }
    query.offset = offset;
  }
}

/*
 * paginate walks every item of a list, across its pages.
 *
 *   for await (const contact of paginate(api, "/contacts")) { ... }
 */
export async function* paginate<P extends ListPaths>(
  api: DimmiApi,
  path: P,
  init: ListInit = {},
): AsyncGenerator<ItemOf<P>, void, undefined> {
  for await (const page of pages(api, path, init)) {
    yield* ((page as Page).data ?? []) as ItemOf<P>[];
  }
}
