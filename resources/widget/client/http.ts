/*
 * How the messenger talks to its server. Every call goes to the install's
 * widget API with the visitor's bearer token once there is one, and every
 * refusal comes back as an ApiError carrying the server's stable code, so
 * the rest of the client decides by code and never by message text. The
 * server's clock is remembered from each answer, so times shown to the
 * visitor line up with the team's even when the visitor's clock is off.
 *
 * Nothing here touches the DOM: fetch and the clock are handed in, so the
 * same code runs in the browser, in tests and in headless clients.
 */

export interface Transport {
  fetch: typeof fetch;
  now: () => number;
}

/*
 * ApiError is a refusal from the server, or a request that never got an
 * answer, which has status 0 and code "network".
 */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly retryAfter: number | undefined;
  readonly body: Record<string, unknown>;

  constructor(status: number, code: string, message: string, body: Record<string, unknown> = {}, retryAfter?: number) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.body = body;
    this.retryAfter = retryAfter;
  }
}

export interface RequestOptions {
  body?: unknown;
  raw?: BodyInit;
  contentType?: string;
  headers?: Record<string, string>;
}

export class Http {
  token: string | undefined;
  private offset = 0;

  constructor(
    private readonly transport: Transport,
    readonly base: string,
  ) {}

  /*
   * serverNow is the server's time, as far as the last answer told.
   */
  serverNow(): number {
    return this.transport.now() + this.offset;
  }

  get<T>(path: string): Promise<T> {
    return this.request<T>("GET", path);
  }

  post<T>(path: string, body?: unknown): Promise<T> {
    return this.request<T>("POST", path, { body });
  }

  patch<T>(path: string, body?: unknown): Promise<T> {
    return this.request<T>("PATCH", path, { body });
  }

  delete<T>(path: string): Promise<T> {
    return this.request<T>("DELETE", path);
  }

  async request<T>(method: string, path: string, options: RequestOptions = {}): Promise<T> {
    const headers: Record<string, string> = { Accept: "application/json", ...options.headers };
    let body: BodyInit | undefined = options.raw;
    if (options.body !== undefined) {
      headers["Content-Type"] = "application/json";
      body = JSON.stringify(options.body);
    } else if (options.contentType) {
      headers["Content-Type"] = options.contentType;
    }
    if (this.token) {
      headers.Authorization = `Bearer ${this.token}`;
    }

    let response: Response;
    try {
      response = await this.transport.fetch(this.url(path), { method, headers, body, credentials: "omit" });
    } catch (error) {
      throw new ApiError(0, "network", error instanceof Error ? error.message : "The server could not be reached.");
    }

    this.clock(response.headers.get("Dimmi-Server-Time"));
    const parsed = await read(response);
    if (!response.ok) {
      throw refusal(response, parsed);
    }

    return parsed as T;
  }

  /*
   * url resolves a path against the API base; a full address, such as a
   * signed upload, is used as it is.
   */
  url(path: string): string {
    return /^https?:\/\//.test(path) ? path : `${this.base}${path}`;
  }

  private clock(header: string | null) {
    const server = header ? Date.parse(header) : Number.NaN;
    if (!Number.isNaN(server)) {
      this.offset = server - this.transport.now();
    }
  }
}

async function read(response: Response): Promise<Record<string, unknown>> {
  const text = await response.text();
  if (text === "") {
    return {};
  }
  try {
    return JSON.parse(text) as Record<string, unknown>;
  } catch {
    return {};
  }
}

function refusal(response: Response, body: Record<string, unknown>): ApiError {
  const code = typeof body.code === "string" ? body.code : `http_${response.status}`;
  const message = typeof body.message === "string" ? body.message : response.statusText;
  const retry = Number(response.headers.get("Retry-After"));

  return new ApiError(response.status, code, message, body, Number.isFinite(retry) && retry > 0 ? retry : undefined);
}
