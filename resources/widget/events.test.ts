import { boot } from "./core";
import type { DimmiWindow } from "./settings";

type TestWindow = Window & DimmiWindow;

const win = window as TestWindow;
const api = "https://app.example.com/widget/v1";

/*
 * serve answers the messenger's requests from a table of routes and
 * records what it was asked, with the bodies sent.
 */
function serve(routes: Record<string, (body: unknown) => { status: number; body?: unknown }>) {
  const calls: { key: string; body: unknown }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string | URL | Request, init: RequestInit = {}) => {
      const key = `${init.method ?? "GET"} ${String(url).replace(api, "").split("?")[0]}`;
      const body = init.body ? JSON.parse(String(init.body)) : undefined;
      calls.push({ key, body });
      const route = routes[key];
      if (!route) {
        return new Response(JSON.stringify({ code: "not_found" }), { status: 404 });
      }
      const answer = route(body);
      return new Response(answer.body === undefined ? "" : JSON.stringify(answer.body), {
        status: answer.status,
        headers: { "Content-Type": "application/json" },
      });
    }),
  );

  return calls;
}

class FakeEventSource {
  onmessage = null;
  onerror = null;
  closed = false;
  close() {
    this.closed = true;
  }
}

const config = {
  data: { company: { name: "Shop" }, tabs: ["home", "messages"], branding: { show: false } },
};
const conversation = {
  id: "conv_1",
  number: 1,
  status: "open",
  subject: null,
  last_message: null,
  unread_count: 2,
  ai_state: "off",
  customer_status: "with_team",
  widget_id: "wgt_1",
  created_at: "2026-01-01T00:00:00Z",
  last_message_at: null,
};
const base = {
  "GET /apps/wgt_1/config": () => ({ status: 200, body: config }),
  "GET /apps/wgt_1/availability": () => ({ status: 200, body: { data: { online: true, team: [] } } }),
};
const known = {
  ...base,
  "GET /session": () => ({ status: 200, body: { data: { id: "wss_1", kind: "anonymous", contact: { id: "c" } } } }),
  "GET /conversations": () => ({ status: 200, body: { data: [conversation] } }),
  "POST /realtime-tokens": () => ({ status: 503, body: {} }),
  "GET /updates": () => ({ status: 200, body: { data: [], meta: { cursor: "c", poll_after_ms: 60000 } } }),
};

const settle = async (times = 6) => {
  for (let i = 0; i < times; i++) {
    await new Promise((resolve) => setTimeout(resolve, 0));
  }
};

const intervals: ReturnType<typeof setInterval>[] = [];
const startInterval = window.setInterval.bind(window);

beforeEach(() => {
  vi.spyOn(window, "setInterval").mockImplementation((handler: TimerHandler, ms?: number) => {
    const handle = startInterval(handler, ms);
    intervals.push(handle);
    return handle;
  });
  win.Dimmi = Object.assign(
    (...command: [string, ...unknown[]]) => {
      win.Dimmi?.q?.push(command);
    },
    { q: [] as [string, ...unknown[]][] },
  );
  win.__dimmi = { appId: "wgt_1", apiBase: "https://app.example.com" };
  document.body.innerHTML = "";
  localStorage.clear();
  vi.stubGlobal("EventSource", FakeEventSource);
});

afterEach(() => {
  for (const handle of intervals.splice(0)) {
    clearInterval(handle);
  }
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("tracked events", () => {
  it("waits for a session before sending anything", async () => {
    const calls = serve(base);
    win.Dimmi?.("track", "plan.viewed", { plan: "pro" });

    boot(win, document);
    await settle();

    expect(calls.map((c) => c.key)).not.toContain("POST /events");
  });

  it("sends events queued before and after boot, in one batch each, with ids", async () => {
    localStorage.setItem("dimmi:wgt_1:token", "dmi_ws_1");
    const sent: unknown[] = [];
    const calls = serve({
      ...known,
      "POST /events": (body) => {
        sent.push(body);
        const events = (body as { events: unknown[] }).events;
        return {
          status: 200,
          body: { data: { accepted: events.length, events: events.map(() => ({ status: "accepted" })) } },
        };
      },
    });
    win.Dimmi?.("track", "plan.viewed", { plan: "pro" });

    boot(win, document);
    await settle();
    win.Dimmi?.("track", "video.played");
    await settle();

    expect(calls.filter((c) => c.key === "POST /events")).toHaveLength(2);
    const first = sent[0] as { events: { name: string; metadata: unknown; id: string }[] };
    expect(first.events[0]).toMatchObject({ name: "plan.viewed", metadata: { plan: "pro" } });
    expect(first.events[0]?.id).toBeTruthy();
    expect(sent[1]).toMatchObject({ events: [{ name: "video.played", metadata: {} }] });
  });

  it("keeps events that never reached the server for the next try", async () => {
    localStorage.setItem("dimmi:wgt_1:token", "dmi_ws_1");
    let fail = true;
    const sent: unknown[] = [];
    serve({
      ...known,
      "POST /events": (body) => {
        if (fail) {
          return { status: 429, body: { code: "rate_limited", message: "Slow down." } };
        }
        sent.push(body);
        return {
          status: 200,
          body: { data: { accepted: 1, events: [{ status: "accepted" }, { status: "accepted" }] } },
        };
      },
    });
    win.Dimmi?.("track", "a");
    boot(win, document);
    await settle();

    fail = false;
    win.Dimmi?.("track", "b");
    await settle();

    expect(sent).toEqual([
      { events: [expect.objectContaining({ name: "a" }), expect.objectContaining({ name: "b" })] },
    ]);
  });
});

describe("the unread count", () => {
  it("tells the page its unread total once at boot", async () => {
    localStorage.setItem("dimmi:wgt_1:token", "dmi_ws_1");
    serve(known);
    const counts: number[] = [];
    window.addEventListener("dimmi:unread", (event) =>
      counts.push((event as CustomEvent<{ count: number }>).detail.count),
    );

    boot(win, document);
    await settle();

    expect(counts).toEqual([2]);
  });

  it("tells a first visit's page zero", async () => {
    serve(base);
    const counts: number[] = [];
    window.addEventListener("dimmi:unread", (event) =>
      counts.push((event as CustomEvent<{ count: number }>).detail.count),
    );

    boot(win, document);
    await settle();

    expect(counts).toEqual([0]);
  });
});
