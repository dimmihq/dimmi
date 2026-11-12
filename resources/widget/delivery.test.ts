import "./lib/thread_parts";
import { boot } from "./core";
import type { DimmiWindow } from "./settings";

type TestWindow = Window & DimmiWindow;

const win = window as TestWindow;
const api = "https://app.example.com/widget/v1";

/*
 * serve answers the messenger's requests from a table of routes and
 * records the address of each, query and all.
 */
function serve(routes: Record<string, (body: unknown) => { status: number; body?: unknown }>) {
  const calls: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string | URL | Request, init: RequestInit = {}) => {
      const path = String(url).replace(api, "");
      calls.push(`${init.method ?? "GET"} ${path}`);
      const route = routes[`${init.method ?? "GET"} ${path.split("?")[0]}`];
      if (!route) {
        return new Response(JSON.stringify({ code: "not_found" }), { status: 404 });
      }
      const answer = route(init.body ? JSON.parse(String(init.body)) : undefined);
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
  close() {
    return;
  }
}

const settle = async (times = 4) => {
  for (let i = 0; i < times; i++) {
    await new Promise((resolve) => setTimeout(resolve, 0));
  }
};
const shadow = () => document.getElementById("dimmi-messenger")?.shadowRoot as ShadowRoot;
const delivery = () => shadow().querySelector<HTMLElement>(".thread [data-delivery]");

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
const mine = {
  id: "msg_1",
  conversation_id: "conv_1",
  client_id: "c1",
  author: { type: "contact", name: null, avatar_url: null },
  body: "Export fails",
  attachments: [],
  created_at: "2026-01-01T00:00:01Z",
};
const base = {
  "GET /apps/wgt_1/config": () => ({
    status: 200,
    body: { data: { company: { name: "Shop" }, tabs: ["home", "messages"], texts: { delivery_sent: "Sent" } } },
  }),
  "GET /apps/wgt_1/availability": () => ({ status: 200, body: { data: { online: true, team: [] } } }),
  "POST /realtime-tokens": () => ({ status: 503, body: {} }),
  "GET /updates": () => ({ status: 200, body: { data: [], meta: { cursor: "c", poll_after_ms: 60000 } } }),
};

const intervals: ReturnType<typeof setInterval>[] = [];
const startInterval = window.setInterval.bind(window);

beforeEach(() => {
  vi.spyOn(window, "setInterval").mockImplementation((handler: TimerHandler, ms?: number) => {
    const handle = startInterval(handler, ms);
    intervals.push(handle);
    return handle;
  });
  delete win.Dimmi;
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

describe("delivery and read receipts", () => {
  it("shows Seen under the visitor's latest message once the team read it, and Sent before", async () => {
    localStorage.setItem("dimmi:wgt_1:token", "dmi_ws_1");
    let seen: string | undefined;
    serve({
      ...base,
      "GET /session": () => ({ status: 200, body: { data: { id: "wss_1", kind: "anonymous", contact: { id: "c" } } } }),
      "GET /conversations": () => ({
        status: 200,
        body: { data: [{ ...conversation, ...(seen ? { seen_by_team_at: seen } : {}) }] },
      }),
      "GET /conversations/conv_1/messages": () => ({ status: 200, body: { data: [mine] } }),
      "POST /conversations/conv_1/read": () => ({ status: 204 }),
    });

    const widget = boot(win, document);
    await settle();
    shadow().dispatchEvent(new CustomEvent("dimmi:converse", { detail: { conversation: "conv_1" } }));
    await settle();

    expect(delivery()?.dataset.delivery).toBe("sent");
    expect(delivery()?.textContent).toBe("Sent");

    widget?.live?.store.apply("conversation.seen_by_team", {
      conversation_id: "conv_1",
      seen_at: "2026-01-01T00:00:05Z",
    });
    expect(delivery()?.dataset.delivery).toBe("seen");
    expect(delivery()?.textContent).toBe("Seen");

    seen = "2026-01-01T00:00:05Z";
    widget?.live?.store.seen.clear();
    widget?.live?.store.setConversations([{ ...conversation, seen_by_team_at: seen }]);
    expect(widget?.live?.store.seen.get("conv_1")?.at).toBe(seen);
  });

  it("says a refused message was not delivered and sends it again on Try again", async () => {
    let attempts = 0;
    serve({
      ...base,
      "POST /sessions": () => ({
        status: 201,
        body: { data: { id: "wss_1", token: "dmi_ws_1", kind: "anonymous", contact: { id: "c" } } },
      }),
      "POST /conversations": (body) => {
        attempts += 1;
        if (attempts === 1) {
          return { status: 422, body: { code: "message_rejected", message: "No." } };
        }
        return {
          status: 201,
          body: {
            data: conversation,
            meta: { message: { ...mine, client_id: (body as { client_id: string }).client_id } },
          },
        };
      },
    });

    boot(win, document);
    await settle();
    const root = shadow();
    root.dispatchEvent(new CustomEvent("dimmi:converse", { detail: {} }));
    root.dispatchEvent(new CustomEvent("dimmi:send", { detail: { body: "Export fails", files: [] } }));
    await settle(1);
    expect(["sending", "failed"]).toContain(delivery()?.dataset.delivery);
    await settle(6);

    expect(delivery()?.dataset.delivery).toBe("failed");
    expect(delivery()?.textContent).toContain("Not delivered");
    delivery()?.querySelector<HTMLButtonElement>("[data-retry]")?.click();
    await settle(6);

    expect(attempts).toBe(2);
    expect(delivery()?.dataset.delivery).toBe("sent");
    expect(root.querySelectorAll(".thread .bubble").length).toBe(1);
  });
});
