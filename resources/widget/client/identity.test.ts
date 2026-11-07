import { Http } from "./http";
import { fingerprint, fromFunction, fromUrl, subjectOf } from "./identity";
import { Session, type Storage } from "./session";

/*
 * Fetch answers from a list of routes and records each request with its
 * body; storage is a map.
 */
type Route = (init: RequestInit) => { status: number; body?: unknown; headers?: Record<string, string> };

function fakeFetch(routes: Record<string, Route>, calls: { key: string; body?: unknown }[] = []) {
  return vi.fn(async (url: string | URL | Request, init: RequestInit = {}) => {
    const key = `${init.method ?? "GET"} ${String(url).replace("https://app.example/widget/v1", "")}`;
    calls.push({ key, body: init.body ? JSON.parse(String(init.body)) : undefined });
    const route = routes[key];
    if (!route) {
      throw new TypeError("Failed to fetch");
    }
    const { status, body, headers } = route(init);
    return new Response(body === undefined ? "" : JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json", ...headers },
    });
  }) as unknown as typeof fetch;
}

function memory(): Storage & { data: Map<string, string> } {
  const data = new Map<string, string>();
  return {
    data,
    get: (key) => data.get(key) ?? null,
    set: (key, value) => void data.set(key, value),
    remove: (key) => void data.delete(key),
  };
}

const base = "https://app.example/widget/v1";

/*
 * token builds an unsigned token naming the subject; only the server
 * checks signatures.
 */
function token(sub: string): string {
  const part = (value: object) => btoa(JSON.stringify(value)).replace(/=+$/, "");
  return `${part({ alg: "HS256" })}.${part({ sub })}.sig`;
}

const signedIn = { id: "wss_2", token: "dmi_ws_signed", kind: "identified", contact: { id: "ctc_2" } };

describe("identity sources", () => {
  it("reads the token from the identity url, and anything unclear as signed out", async () => {
    const ok = fakeFetch({ "GET /api/dimmi/token": () => ({ status: 200, body: { token: "t.o.k" } }) });
    expect(await fromUrl(ok, "/api/dimmi/token")()).toBe("t.o.k");

    const out = fakeFetch({ "GET /api/dimmi/token": () => ({ status: 200, body: { token: null } }) });
    expect(await fromUrl(out, "/api/dimmi/token")()).toBeNull();

    const error = vi.spyOn(console, "error").mockImplementation(() => undefined);
    const html = fakeFetch({
      "GET /api/dimmi/token": () => ({ status: 302, body: "<html>", headers: { "Content-Type": "text/html" } }),
    });
    expect(await fromUrl(html, "/api/dimmi/token")()).toBeNull();
    expect(error).toHaveBeenCalledWith(expect.stringContaining("identity_endpoint_invalid"));
    error.mockRestore();
  });

  it("treats a throwing identity function as signed out", async () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => undefined);
    expect(
      await fromFunction(() => {
        throw new Error("boom");
      })(),
    ).toBeNull();
    expect(await fromFunction(async () => "a.b.c")()).toBe("a.b.c");
    warn.mockRestore();
  });

  it("reads the subject without trusting the token", () => {
    expect(subjectOf(token("user_42"))).toBe("user_42");
    expect(subjectOf("nonsense")).toBeUndefined();
  });
});

describe("signed-in sessions", () => {
  it("signs a new visitor in and remembers only a hash of who", async () => {
    const calls: { key: string; body?: unknown }[] = [];
    const http = new Http(
      {
        fetch: fakeFetch({ "POST /sessions": () => ({ status: 201, body: { data: signedIn } }) }, calls),
        now: Date.now,
      },
      base,
    );
    const storage = memory();
    const s = new Session(http, storage, "wgt_1");

    expect((await s.identify(token("user_42")))?.id).toBe("wss_2");
    expect(calls[0]?.body).toMatchObject({ app_id: "wgt_1", user_jwt: token("user_42") });
    expect(storage.data.get("dimmi:wgt_1:token")).toBe("dmi_ws_signed");
    expect(storage.data.get("dimmi:wgt_1:sub")).toBe(await fingerprint("user_42"));
    expect([...storage.data.values()].join()).not.toContain("user_42");
  });

  it("hands an anonymous session over when the visitor signs in", async () => {
    const calls: { key: string; body?: unknown }[] = [];
    const http = new Http(
      {
        fetch: fakeFetch({ "POST /sessions": () => ({ status: 201, body: { data: signedIn } }) }, calls),
        now: Date.now,
      },
      base,
    );
    const storage = memory();
    storage.set("dimmi:wgt_1:token", "dmi_ws_anon");
    await new Session(http, storage, "wgt_1").identify(token("user_42"));
    expect(calls[0]?.body).toMatchObject({ previous_token: "dmi_ws_anon" });
  });

  it("refreshes the session of the same customer", async () => {
    const calls: { key: string; body?: unknown }[] = [];
    const http = new Http(
      {
        fetch: fakeFetch(
          { "PATCH /session": () => ({ status: 200, body: { data: { ...signedIn, token: undefined } } }) },
          calls,
        ),
        now: Date.now,
      },
      base,
    );
    const storage = memory();
    storage.set("dimmi:wgt_1:token", "dmi_ws_signed");
    storage.set("dimmi:wgt_1:sub", await fingerprint("user_42"));

    expect((await new Session(http, storage, "wgt_1").identify(token("user_42")))?.id).toBe("wss_2");
    expect(calls.map((c) => c.key)).toEqual(["PATCH /session"]);
  });

  it("signs out at once when the page says nobody is signed in", async () => {
    const calls: { key: string; body?: unknown }[] = [];
    const http = new Http(
      { fetch: fakeFetch({ "DELETE /session": () => ({ status: 204 }) }, calls), now: Date.now },
      base,
    );
    const storage = memory();
    storage.set("dimmi:wgt_1:token", "dmi_ws_signed");
    storage.set("dimmi:wgt_1:sub", await fingerprint("user_42"));

    expect(await new Session(http, storage, "wgt_1").identify(null)).toBeUndefined();
    expect(calls.map((c) => c.key)).toEqual(["DELETE /session"]);
    expect(storage.data.size).toBe(0);
  });

  it("keeps an anonymous visitor as they were when their token is refused", async () => {
    const error = vi.spyOn(console, "error").mockImplementation(() => undefined);
    const http = new Http(
      {
        fetch: fakeFetch({
          "POST /sessions": () => ({
            status: 401,
            body: { code: "token_expired", message: "Token expired 3 minutes ago.", hint: "Mint per request." },
          }),
          "GET /session": () => ({ status: 200, body: { data: { id: "wss_1", kind: "anonymous", contact: {} } } }),
        }),
        now: Date.now,
      },
      base,
    );
    const storage = memory();
    storage.set("dimmi:wgt_1:token", "dmi_ws_anon");

    expect((await new Session(http, storage, "wgt_1").identify(token("user_42")))?.id).toBe("wss_1");
    expect(error).toHaveBeenCalledWith("[dimmi] token_expired: Token expired 3 minutes ago. Mint per request.");
    expect(storage.data.get("dimmi:wgt_1:token")).toBe("dmi_ws_anon");
    error.mockRestore();
  });
});
