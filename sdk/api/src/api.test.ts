import { createDimmiApi, DimmiApiError, paginate, retryAfter, unwrap } from "./index";

type Seen = { method: string; url: string; headers: Headers; body: string };

function server(answers: Array<(seen: Seen) => Response>) {
  const seen: Seen[] = [];
  const fetch = async (request: Request) => {
    const entry = { method: request.method, url: request.url, headers: request.headers, body: await request.text() };
    seen.push(entry);
    const answer = answers.shift();
    if (!answer) {
      throw new Error("no answer left");
    }
    return answer(entry);
  };
  return { seen, fetch };
}

const json = (body: unknown, status = 200, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify(body), { status, headers: { "content-type": "application/json", ...headers } });

describe("createDimmiApi", () => {
  it("sends the token, workspace and path parameters", async () => {
    const { seen, fetch } = server([() => json({ id: "ct_1" })]);
    const api = createDimmiApi({ host: "https://d.test/", token: "pat_x", workspace: "ws_1", fetch });
    const { data } = await api.GET("/contacts/{contact}", { params: { path: { contact: "ct 1" } } });
    expect(data).toEqual({ id: "ct_1" });
    expect(seen[0].url).toBe("https://d.test/api/v1/contacts/ct%201");
    expect(seen[0].headers.get("authorization")).toBe("Bearer pat_x");
    expect(seen[0].headers.get("dimmi-workspace")).toBe("ws_1");
  });

  it("retries a 429 after Retry-After, keeping the idempotency key", async () => {
    const { seen, fetch } = server([
      () => json({ message: "slow down", code: "rate_limited" }, 429, { "retry-after": "0" }),
      () => json({ id: "ct_1" }, 201),
    ]);
    const api = createDimmiApi({ host: "https://d.test", token: "t", fetch, autoIdempotency: true });
    const { response } = await api.POST("/contacts", { body: { email: "a@b.co" } as never });
    expect(response.status).toBe(201);
    expect(seen).toHaveLength(2);
    expect(seen[1].body).toBe('{"email":"a@b.co"}');
    const key = seen[0].headers.get("idempotency-key");
    expect(key).toBeTruthy();
    expect(seen[1].headers.get("idempotency-key")).toBe(key);
  });

  it("gives up after maxRetries and unwrap throws the API error", async () => {
    const limited = () => json({ message: "slow down", code: "rate_limited" }, 429, { "retry-after": "0" });
    const { fetch } = server([limited, limited]);
    const api = createDimmiApi({ host: "https://d.test", token: "t", fetch, maxRetries: 1 });
    const error = await unwrap(api.GET("/contacts")).catch((e: unknown) => e);
    expect(error).toBeInstanceOf(DimmiApiError);
    expect((error as DimmiApiError).code).toBe("rate_limited");
    expect((error as DimmiApiError).status).toBe(429);
  });

  it("needs a token and a host", () => {
    expect(() => createDimmiApi({ token: "", host: "https://d.test" })).toThrow(/token/);
    expect(() => createDimmiApi({ token: "t" })).toThrow(/host/);
  });
});

describe("paginate", () => {
  it("follows cursors", async () => {
    const { seen, fetch } = server([
      () => json({ data: [{ id: "1" }, { id: "2" }], meta: { next_cursor: "c2", per_page: 2 } }),
      () => json({ data: [{ id: "3" }], meta: { per_page: 2 } }),
    ]);
    const api = createDimmiApi({ host: "https://d.test", token: "t", fetch });
    const ids: string[] = [];
    for await (const contact of paginate(api, "/contacts", { params: { query: { limit: 2 } as never } })) {
      ids.push(contact.id);
    }
    expect(ids).toEqual(["1", "2", "3"]);
    expect(new URL(seen[1].url).searchParams.get("cursor")).toBe("c2");
  });

  it("pages by offset where the list does", async () => {
    const { seen, fetch } = server([
      () => json({ data: [{ id: "1" }, { id: "2" }], meta: { total: 3, per_page: 2 } }),
      () => json({ data: [{ id: "3" }], meta: { total: 3, per_page: 2 } }),
    ]);
    const api = createDimmiApi({ host: "https://d.test", token: "t", fetch });
    const items = [];
    for await (const broadcast of paginate(api, "/broadcasts", { params: { query: { limit: 2 } } })) {
      items.push(broadcast);
    }
    expect(items).toHaveLength(3);
    expect(new URL(seen[1].url).searchParams.get("offset")).toBe("2");
  });
});

it("reads Retry-After in seconds or as a date", () => {
  expect(retryAfter("2")).toBe(2000);
  expect(retryAfter(new Date(10_000).toUTCString(), 4_000)).toBe(6000);
  expect(retryAfter(null)).toBeUndefined();
});
