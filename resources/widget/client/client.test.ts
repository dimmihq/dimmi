import { ApiError, Http } from "./http";
import { Outbox } from "./outbox";
import { Realtime, type Source } from "./realtime";
import { Session, type Storage } from "./session";
import { type Message, Store } from "./store";

/*
 * The browser's own pieces stand in here: fetch answers from a list of
 * routes, storage is a map, the event source records what it is asked and
 * timers run only when the test says.
 */
type Route = (init: RequestInit) => { status: number; body?: unknown; headers?: Record<string, string> };

function fakeFetch(routes: Record<string, Route>, calls: string[] = []) {
  return vi.fn(async (url: string | URL | Request, init: RequestInit = {}) => {
    const key = `${init.method ?? "GET"} ${String(url).replace("https://app.example/widget/v1", "")}`;
    calls.push(key);
    const route = routes[key] ?? routes[key.split("?")[0] as string];
    if (!route) {
      throw new TypeError("Failed to fetch");
    }
    const { status, body, headers } = route(init);
    return new Response(body === undefined ? "" : JSON.stringify(body), { status, headers });
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

function timers() {
  const queue: { fn: () => void; ms: number }[] = [];
  return {
    queue,
    setTimeout: (fn: () => void, ms: number) => {
      queue.push({ fn, ms });
      return queue.length;
    },
    clearTimeout: () => undefined,
    run: () => {
      for (const t of queue.splice(0)) {
        t.fn();
      }
    },
  };
}

const base = "https://app.example/widget/v1";
const flush = () => new Promise((resolve) => setTimeout(resolve, 0));

describe("http", () => {
  it("sends the bearer, reads the server clock and maps refusals to codes", async () => {
    const seen: RequestInit[] = [];
    const http = new Http(
      {
        fetch: fakeFetch({
          "GET /session": (init) => {
            seen.push(init);
            return {
              status: 401,
              body: { code: "session_revoked", message: "ended" },
              headers: { "Retry-After": "3" },
            };
          },
          "GET /ok": () => ({
            status: 200,
            body: { ok: true },
            headers: { "Dimmi-Server-Time": "2026-01-01T00:00:10Z" },
          }),
        }),
        now: () => Date.parse("2026-01-01T00:00:00Z"),
      },
      base,
    );
    http.token = "dmi_ws_x";

    await expect(http.get("/session")).rejects.toMatchObject({ status: 401, code: "session_revoked", retryAfter: 3 });
    expect((seen[0]?.headers as Record<string, string> | undefined)?.Authorization).toBe("Bearer dmi_ws_x");

    await expect(http.get("/ok")).resolves.toEqual({ ok: true });
    expect(http.serverNow()).toBe(Date.parse("2026-01-01T00:00:10Z"));
    await expect(http.get("/nowhere")).rejects.toMatchObject({ status: 0, code: "network" });
    expect(http.url("https://bucket.example/put")).toBe("https://bucket.example/put");
  });
});

describe("session", () => {
  const session = { id: "wss_1", token: "dmi_ws_new", kind: "anonymous", contact: { id: "ctc_1" } };

  it("makes no request without a stored token and starts once when needed", async () => {
    const calls: string[] = [];
    const http = new Http(
      {
        fetch: fakeFetch({ "POST /sessions": () => ({ status: 201, body: { data: session } }) }, calls),
        now: Date.now,
      },
      base,
    );
    const storage = memory();
    const s = new Session(http, storage, "wgt_1");

    expect(await s.resume()).toBeUndefined();
    expect(calls).toEqual([]);

    const [a, b] = await Promise.all([s.ensure(), s.ensure()]);
    expect(a).toBe(b);
    expect(calls).toEqual(["POST /sessions"]);
    expect(storage.data.get("dimmi:wgt_1:token")).toBe("dmi_ws_new");
    expect(http.token).toBe("dmi_ws_new");
  });

  it("drops a stored token the server no longer knows", async () => {
    const storage = memory();
    storage.set("dimmi:wgt_1:token", "dmi_ws_old");
    const http = new Http(
      {
        fetch: fakeFetch({ "GET /session": () => ({ status: 401, body: { code: "session_expired" } }) }),
        now: Date.now,
      },
      base,
    );
    const s = new Session(http, storage, "wgt_1");

    expect(await s.resume()).toBeUndefined();
    expect(storage.data.has("dimmi:wgt_1:token")).toBe(false);
    expect(http.token).toBeUndefined();
  });

  it("resumes a stored token and signs out", async () => {
    const storage = memory();
    storage.set("dimmi:wgt_1:token", "dmi_ws_old");
    const calls: string[] = [];
    const http = new Http(
      {
        fetch: fakeFetch(
          {
            "GET /session": () => ({ status: 200, body: { data: { ...session, token: undefined } } }),
            "DELETE /session": () => ({ status: 204 }),
          },
          calls,
        ),
        now: Date.now,
      },
      base,
    );
    const s = new Session(http, storage, "wgt_1");

    expect((await s.resume())?.id).toBe("wss_1");
    await s.signOut();
    expect(calls).toEqual(["GET /session", "DELETE /session"]);
    expect(storage.data.size).toBe(0);
  });

  it("passes on errors that are not about the token", async () => {
    const storage = memory();
    storage.set("dimmi:wgt_1:token", "dmi_ws_old");
    const http = new Http({ fetch: fakeFetch({}), now: Date.now }, base);

    await expect(new Session(http, storage, "wgt_1").resume()).rejects.toBeInstanceOf(ApiError);
    expect(storage.data.has("dimmi:wgt_1:token")).toBe(true);
  });
});

describe("outbox", () => {
  const conversation = { id: "conv_1", widget_id: "wgt_1" };
  const message = { id: "msg_1", client_id: "c1" };

  function outbox(routes: Record<string, Route>, storage = memory()) {
    const t = timers();
    const sent: unknown[] = [];
    const refused: string[] = [];
    const http = new Http({ fetch: fakeFetch(routes), now: Date.now }, base);
    let n = 0;
    const box = new Outbox(
      http,
      storage,
      "wgt_1",
      { setTimeout: t.setTimeout, uuid: () => `c${++n}`, now: () => 1 },
      {
        sent: (s) => sent.push(s),
        refused: (_, e) => refused.push(e.code),
        ready: async () => undefined,
      },
    );
    return { box, t, sent, refused, storage };
  }

  it("starts a conversation, then writes into it, keeping nothing once sent", async () => {
    const { box, sent, storage } = outbox({
      "POST /conversations": () => ({ status: 201, body: { data: conversation, meta: { message } } }),
      "POST /conversations/conv_1/messages": () => ({
        status: 201,
        body: { data: { id: "msg_2", conversation_id: "conv_1" } },
      }),
    });

    box.send({ body: "hi", attachmentIds: [] });
    await flush();
    box.send({ conversation: "conv_1", body: "again", attachmentIds: [] });
    await flush();

    expect(sent).toHaveLength(2);
    expect(sent[0]).toMatchObject({ conversationId: "conv_1", message, conversation });
    expect(storage.data.size).toBe(0);
  });

  it("keeps a message through an outage and sends it with the same client id later", async () => {
    let up = false;
    const ids: string[] = [];
    const { box, t, sent, storage } = outbox({
      "POST /conversations": (init) => {
        ids.push(JSON.parse(String(init.body)).client_id);
        return up ? { status: 201, body: { data: conversation, meta: { message } } } : { status: 503, body: {} };
      },
    });

    box.send({ body: "hi", attachmentIds: [] });
    await flush();
    expect(sent).toHaveLength(0);
    expect(JSON.parse(storage.data.get("dimmi:wgt_1:outbox") ?? "[]")).toHaveLength(1);
    expect(t.queue[0]?.ms).toBe(500);

    up = true;
    t.run();
    await flush();
    expect(sent).toHaveLength(1);
    expect(ids).toEqual(["c1", "c1"]);
  });

  it("hands back a refusal that will not change and moves on", async () => {
    const { box, refused } = outbox({
      "POST /conversations": () => ({ status: 422, body: { code: "email_required" } }),
    });

    box.send({ body: "hi", attachmentIds: [] });
    await flush();

    expect(refused).toEqual(["email_required"]);
    expect(box.pending).toHaveLength(0);
  });

  it("sends what an earlier page left behind", async () => {
    const storage = memory();
    storage.set(
      "dimmi:wgt_1:outbox",
      JSON.stringify([{ clientId: "old", body: "left", attachmentIds: [], createdAt: 0 }]),
    );
    const { box, sent } = outbox(
      { "POST /conversations": () => ({ status: 201, body: { data: conversation, meta: { message } } }) },
      storage,
    );

    expect(box.pending).toHaveLength(1);
    await box.flush();
    expect(sent).toHaveLength(1);
  });
});

describe("realtime", () => {
  function realtime(routes: Record<string, Route>) {
    const t = timers();
    const sources: (Source & { url: string })[] = [];
    const events: string[] = [];
    const modes: string[] = [];
    let resyncs = 0;
    let ended = 0;
    const http = new Http({ fetch: fakeFetch(routes), now: Date.now }, base);
    const rt = new Realtime(
      http,
      {
        eventSource: (url) => {
          const source = { url, onmessage: null, onerror: null, close: vi.fn() };
          sources.push(source);
          return source;
        },
        setTimeout: t.setTimeout,
        clearTimeout: t.clearTimeout,
        now: () => 0,
      },
      "wgt_1",
      {
        event: (e) => events.push(e.event),
        resync: () => resyncs++,
        ended: () => ended++,
        mode: (m) => modes.push(m),
      },
    );
    const push = (source: Source, value: unknown) => source.onmessage?.({ data: JSON.stringify(value) });
    return { rt, t, sources, events, modes, push, counts: () => ({ resyncs, ended }) };
  }

  const token = () => ({
    status: 201,
    body: { data: { token: "eyJ", url: "https://app.example/connection/uni_sse", channel: "widget_contact:ctc_1" } },
  });

  it("streams events once each, drops other widgets' and recovers from its position", async () => {
    const { rt, t, sources, events, push, counts } = realtime({ "POST /realtime-tokens": token });

    rt.start();
    await flush();
    const first = sources[0] as Source & { url: string };
    expect(decodeURIComponent(first.url)).toContain('"token":"eyJ"');

    push(first, { connect: { subs: { "widget_contact:ctc_1": { epoch: "e1", offset: 4 } } } });
    const pub = { data: { id: "evt_1", event: "message.created", data: { widget_id: "wgt_1" } }, offset: 5 };
    push(first, { pub });
    push(first, { pub });
    push(first, { pub: { data: { id: "evt_2", event: "message.created", data: { widget_id: "wgt_2" } }, offset: 6 } });
    expect(events).toEqual(["message.created"]);

    first.onerror?.({});
    t.run();
    await flush();
    const again = sources.at(-1) as Source & { url: string };
    expect(decodeURIComponent(again.url)).toContain('"recover":true,"epoch":"e1","offset":6');

    push(again, { connect: { subs: { "widget_contact:ctc_1": { recovered: false, epoch: "e2", offset: 0 } } } });
    expect(counts().resyncs).toBe(1);
  });

  it("ends when the session ends and falls back to polling when replaced", async () => {
    const { rt, sources, push, modes, counts, events } = realtime({
      "POST /realtime-tokens": token,
      "GET /updates": () => ({
        status: 200,
        body: {
          data: [{ event: "message.created", id: "evt_9", payload: {} }],
          meta: { cursor: "c", poll_after_ms: 5000 },
        },
      }),
    });

    rt.start();
    await flush();
    push(sources[0] as Source, { disconnect: { code: 4004 } });
    await flush();
    expect(modes).toContain("polling");
    expect(events).toEqual(["message.created"]);

    rt.stop();
    rt.start();
    await flush();
    expect(counts().ended).toBe(0);
  });

  it("gives the session up when the stream says it ended", async () => {
    const { rt, sources, push, counts } = realtime({ "POST /realtime-tokens": token });

    rt.start();
    await flush();
    push(sources[0] as Source, { disconnect: { code: 4009 } });

    expect(counts().ended).toBe(1);
  });

  it("polls after two failed streams within half a minute", async () => {
    const { rt, t, sources, modes } = realtime({
      "POST /realtime-tokens": token,
      "GET /updates": () => ({ status: 200, body: { data: [], meta: { cursor: "c", poll_after_ms: 5000 } } }),
    });

    rt.start();
    await flush();
    (sources[0] as Source).onerror?.({});
    t.run();
    await flush();
    (sources[1] as Source).onerror?.({});
    await flush();

    expect(modes).toEqual(["polling"]);
  });
});

describe("store", () => {
  const author = { type: "member", name: "Kim", avatar_url: null };
  const conversation = {
    id: "conv_1",
    number: 1,
    status: "open",
    subject: null,
    last_message: null,
    unread_count: 0,
    ai_state: "off",
    customer_status: "with_team",
    widget_id: "wgt_1",
    created_at: "2026-01-01T00:00:00Z",
    last_message_at: null,
  };

  it("shows a sent message at once and swaps in the server's copy", () => {
    const store = new Store(() => 0);
    const local: Message = {
      id: "local:c1",
      conversation_id: "conv_1",
      client_id: "c1",
      author: { type: "contact", name: null, avatar_url: null },
      body: "hi",
      attachments: [],
      created_at: "2026-01-01T00:00:01Z",
      sending: true,
    };
    store.addLocal(local);
    store.putMessage({ ...local, id: "msg_1", sending: false });

    expect(store.messages.get("conv_1")?.map((m) => m.id)).toEqual(["msg_1"]);
  });

  it("counts a teammate's reply as unread, clears typing and shows seen", () => {
    let now = 0;
    const store = new Store(() => now);
    const changes: string[] = [];
    store.subscribe((change) => changes.push(change));
    store.setConversations([conversation]);

    store.apply("typing.started", { conversation_id: "conv_1", name: "Kim", avatar_url: null });
    expect(store.typingIn("conv_1")?.name).toBe("Kim");

    store.apply("message.created", {
      customer_status: "waiting_on_you",
      message: {
        id: "msg_2",
        conversation_id: "conv_1",
        author,
        body: "hello",
        attachments: [],
        created_at: "2026-01-01T00:00:02Z",
      },
    });
    expect(store.unread).toBe(1);
    expect(store.conversations[0]?.customer_status).toBe("waiting_on_you");
    expect(store.typingIn("conv_1")).toBeUndefined();

    store.apply("conversation.seen_by_team", { conversation_id: "conv_1", seen_at: "2026-01-01T00:00:03Z" });
    expect(store.seen.get("conv_1")?.at).toBe("2026-01-01T00:00:03Z");

    store.markRead("conv_1");
    expect(store.unread).toBe(0);

    store.apply("typing.started", { conversation_id: "conv_1", author: { name: "Ada" } });
    now = 7000;
    expect(store.typingIn("conv_1")).toBeUndefined();
    expect(changes).toContain("seen");
  });

  it("counts a reply once when the stream and a poll both bring it", () => {
    const store = new Store(() => 0);
    store.setConversations([conversation]);
    const reply = {
      message: {
        id: "msg_3",
        conversation_id: "conv_1",
        author,
        body: "hello",
        attachments: [],
        created_at: "2026-01-01T00:00:02Z",
      },
    };

    store.apply("message.created", reply);
    store.apply("message.created", reply);

    expect(store.unread).toBe(1);
    expect(store.messages.get("conv_1")?.map((m) => m.id)).toEqual(["msg_3"]);
  });

  it("takes conversation events and moves pending messages to a new conversation", () => {
    const store = new Store(() => 0);
    store.apply("conversation.closed", { conversation: { ...conversation, status: "closed" } });
    expect(store.conversations[0]?.status).toBe("closed");

    store.addLocal({
      id: "local:c1",
      conversation_id: "new",
      client_id: "c1",
      author,
      body: "x",
      attachments: [],
      created_at: "2026-01-01T00:00:00Z",
      sending: true,
    });
    store.moveLocal("new", "conv_1");
    store.failLocal("conv_1", "c1", "email_required");
    expect(store.messages.get("conv_1")?.[0]?.failed).toBe("email_required");
  });
});
