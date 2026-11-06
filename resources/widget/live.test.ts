import { boot } from "./core";
import type { DimmiWindow } from "./settings";

type TestWindow = Window & DimmiWindow;

const win = window as TestWindow;
const api = "https://app.example.com/widget/v1";

/*
 * serve answers the messenger's requests from a table of routes and
 * records what it was asked.
 */
function serve(routes: Record<string, (body: unknown) => { status: number; body?: unknown }>) {
  const calls: string[] = [];
  const fetch = vi.fn(async (url: string | URL | Request, init: RequestInit = {}) => {
    const path = String(url).replace(api, "");
    const key = `${init.method ?? "GET"} ${path.split("?")[0]}`;
    calls.push(key);
    const route = routes[key];
    if (!route) {
      return new Response(JSON.stringify({ code: "not_found" }), { status: 404 });
    }
    const answer = route(init.body ? JSON.parse(String(init.body)) : undefined);
    return new Response(answer.body === undefined ? "" : JSON.stringify(answer.body), { status: answer.status });
  });
  vi.stubGlobal("fetch", fetch);

  return calls;
}

class FakeEventSource {
  onmessage: ((event: { data: string }) => void) | null = null;
  onerror: ((event: unknown) => void) | null = null;
  closed = false;
  close() {
    this.closed = true;
  }
}

const config = {
  data: {
    company: { name: "Shop" },
    appearance: { primary_color: "#5b5bd6" },
    texts: { greeting: "Ask us anything", seen: "Seen" },
    tabs: ["home", "messages"],
    branding: { show: false },
  },
};
const availability = {
  data: { online: true, reply_time_text: "Back soon", team: [{ name: "Kim", avatar_url: null }] },
};
const session = { data: { id: "wss_1", token: "dmi_ws_1", kind: "anonymous", contact: { id: "ctc_1" } } };
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

const settle = () => new Promise((resolve) => setTimeout(resolve, 0));
const shadow = () => document.getElementById("dimmi-messenger")?.shadowRoot as ShadowRoot | undefined;

beforeEach(() => {
  delete win.Dimmi;
  win.__dimmi = { appId: "wgt_1", apiBase: "https://app.example.com" };
  document.body.innerHTML = "";
  localStorage.clear();
  vi.stubGlobal("EventSource", FakeEventSource);
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("a connected messenger", () => {
  it("shows the server's config and makes no request beyond it and availability on a first visit", async () => {
    const calls = serve({
      "GET /apps/wgt_1/config": () => ({ status: 200, body: config }),
      "GET /apps/wgt_1/availability": () => ({ status: 200, body: availability }),
    });

    boot(win, document);
    await settle();
    await settle();

    const root = shadow();
    expect(root?.textContent).toContain("Ask us anything");
    expect(root?.querySelectorAll("[role=tab]")).toHaveLength(2);
    expect(calls).toEqual(["GET /apps/wgt_1/config", "GET /apps/wgt_1/availability"]);
  });

  it("shows nothing and says why when the server does not serve the app here", async () => {
    serve({ "GET /apps/wgt_1/config": () => ({ status: 403, body: { code: "origin_not_allowed" } }) });
    const error = vi.spyOn(console, "error").mockImplementation(() => undefined);

    boot(win, document);
    await settle();

    expect(shadow()).toBeUndefined();
    expect(error.mock.calls.flat().join(" ")).toContain("origin_not_allowed");
  });

  it("starts the session and the conversation with the first message, then lists it", async () => {
    const sent: unknown[] = [];
    serve({
      "GET /apps/wgt_1/config": () => ({ status: 200, body: config }),
      "GET /apps/wgt_1/availability": () => ({ status: 200, body: availability }),
      "POST /sessions": () => ({ status: 201, body: session }),
      "POST /realtime-tokens": () => ({ status: 503, body: {} }),
      "GET /updates": () => ({ status: 200, body: { data: [], meta: { cursor: "c", poll_after_ms: 5000 } } }),
      "POST /conversations": (body) => {
        sent.push(body);
        return {
          status: 201,
          body: {
            data: conversation,
            meta: {
              message: {
                id: "msg_1",
                conversation_id: "conv_1",
                client_id: (body as { client_id: string }).client_id,
                author: { type: "contact", name: null, avatar_url: null },
                body: "Export fails",
                attachments: [],
                created_at: "2026-01-01T00:00:01Z",
              },
            },
          },
        };
      },
    });

    const widget = boot(win, document);
    await settle();
    await settle();
    const root = shadow() as ShadowRoot;

    root.dispatchEvent(new CustomEvent("dimmi:converse", { detail: {} }));
    root.dispatchEvent(new CustomEvent("dimmi:send", { detail: { body: "Export fails", files: [] } }));
    expect(root.querySelector(".thread")?.textContent).toContain("Export fails");

    await settle();
    await settle();
    await settle();

    expect(sent).toHaveLength(1);
    expect(localStorage.getItem("dimmi:wgt_1:token")).toBe("dmi_ws_1");
    expect(widget?.live?.open).toBe("conv_1");
    const launcher = root.querySelector<HTMLButtonElement>(".launcher");
    launcher?.click();
    launcher?.click();
    expect(widget?.live?.open).toBe("conv_1");
    expect(widget?.live?.store.conversations.map((c) => c.id)).toEqual(["conv_1"]);
    expect(root.querySelector('.view[data-view="messages"] .loaded .list')?.textContent).toContain("#1");
  });

  it("asks for an email when the team is away and sends the message once it has one", async () => {
    let email = "";
    const calls = serve({
      "GET /apps/wgt_1/config": () => ({ status: 200, body: config }),
      "GET /apps/wgt_1/availability": () => ({ status: 200, body: availability }),
      "POST /sessions": () => ({ status: 201, body: session }),
      "POST /realtime-tokens": () => ({ status: 503, body: {} }),
      "GET /updates": () => ({ status: 200, body: { data: [], meta: { cursor: "c", poll_after_ms: 5000 } } }),
      "PATCH /contact": (body) => {
        email = (body as { email: string }).email;
        return { status: 200, body: { data: {} } };
      },
      "POST /conversations": (body) =>
        email
          ? {
              status: 201,
              body: {
                data: conversation,
                meta: {
                  message: {
                    id: "msg_1",
                    conversation_id: "conv_1",
                    client_id: (body as { client_id: string }).client_id,
                    author: { type: "contact", name: null, avatar_url: null },
                    body: "Hello?",
                    attachments: [],
                    created_at: "2026-01-01T00:00:01Z",
                  },
                },
              },
            }
          : { status: 422, body: { code: "email_required", message: "Leave an email." } },
    });

    boot(win, document);
    await settle();
    await settle();
    const root = shadow() as ShadowRoot;
    root.dispatchEvent(new CustomEvent("dimmi:converse", { detail: {} }));
    root.dispatchEvent(new CustomEvent("dimmi:send", { detail: { body: "Hello?", files: [] } }));
    for (let i = 0; i < 5; i++) {
      await settle();
    }

    const form = root.querySelector<HTMLFormElement>("form[data-capture]");
    expect(form).not.toBeNull();
    (form?.querySelector("input") as HTMLInputElement).value = "grace@shop.example";
    form?.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    for (let i = 0; i < 6; i++) {
      await settle();
    }

    expect(email).toBe("grace@shop.example");
    expect(calls.filter((c) => c === "POST /conversations")).toHaveLength(2);
    expect(root.querySelector("form[data-capture]")).toBeNull();
  });

  it("shows the agent's answer with its sources and the two actions under it", async () => {
    localStorage.setItem("dimmi:wgt_1:token", "dmi_ws_1");
    const answering = { ...conversation, ai_state: "active" };
    const answer = {
      id: "msg_2",
      conversation_id: "conv_1",
      author: { type: "agent", name: "Ada", avatar_url: null },
      body: "Open Settings, then Billing, and choose Download.",
      attachments: [],
      created_at: "2026-01-01T00:00:02Z",
      ai: {
        generated: true,
        outcome: "answered",
        sources: [
          { title: "Invoices", url: "https://help.shop.example/invoices" },
          { title: "Unsafe", url: "javascript:alert(1)" },
        ],
      },
    };
    const calls = serve({
      "GET /apps/wgt_1/config": () => ({ status: 200, body: config }),
      "GET /apps/wgt_1/availability": () => ({ status: 200, body: availability }),
      "GET /session": () => ({ status: 200, body: session }),
      "GET /conversations": () => ({ status: 200, body: { data: [answering] } }),
      "GET /conversations/conv_1/messages": () => ({ status: 200, body: { data: [answer] } }),
      "PATCH /conversations/conv_1/read-state": () => ({ status: 204 }),
      "POST /conversations/conv_1/ai/helped": () => ({ status: 202 }),
      "POST /conversations/conv_1/ai/handoff": () => ({ status: 202 }),
      "POST /realtime-tokens": () => ({ status: 503, body: {} }),
      "GET /updates": () => ({ status: 200, body: { data: [], meta: { cursor: "c", poll_after_ms: 5000 } } }),
    });

    boot(win, document);
    await settle();
    await settle();
    const root = shadow() as ShadowRoot;
    root.dispatchEvent(new CustomEvent("dimmi:converse", { detail: { conversation: "conv_1" } }));
    for (let i = 0; i < 4; i++) {
      await settle();
    }

    const thread = root.querySelector(".thread") as HTMLElement;
    expect(thread.querySelector(".who")?.textContent).toContain("AI");
    const link = thread.querySelector<HTMLAnchorElement>("a.source");
    expect(link?.href).toBe("https://help.shop.example/invoices");
    expect(link?.target).toBe("_blank");
    expect(thread.querySelectorAll("a.source")).toHaveLength(1);
    expect(thread.querySelector(".ai-actions")?.textContent).toContain("That helped");

    thread.querySelector<HTMLButtonElement>('[data-ai="helped"]')?.click();
    await settle();
    expect(calls).toContain("POST /conversations/conv_1/ai/helped");
    expect(thread.querySelector(".ai-actions")).toBeNull();
    expect(thread.querySelector(".ai-thanks")?.textContent).toContain("Glad it helped.");
  });

  it("hands the conversation to the team when the visitor asks for a person", async () => {
    localStorage.setItem("dimmi:wgt_1:token", "dmi_ws_1");
    const calls = serve({
      "GET /apps/wgt_1/config": () => ({ status: 200, body: config }),
      "GET /apps/wgt_1/availability": () => ({ status: 200, body: availability }),
      "GET /session": () => ({ status: 200, body: session }),
      "GET /conversations": () => ({ status: 200, body: { data: [{ ...conversation, ai_state: "active" }] } }),
      "GET /conversations/conv_1/messages": () => ({
        status: 200,
        body: {
          data: [
            {
              id: "msg_2",
              conversation_id: "conv_1",
              author: { type: "agent", name: "Ada", avatar_url: null },
              body: "I am not sure about that one.",
              attachments: [],
              created_at: "2026-01-01T00:00:02Z",
            },
          ],
        },
      }),
      "PATCH /conversations/conv_1/read-state": () => ({ status: 204 }),
      "POST /conversations/conv_1/ai/handoff": () => ({ status: 202 }),
      "POST /realtime-tokens": () => ({ status: 503, body: {} }),
      "GET /updates": () => ({ status: 200, body: { data: [], meta: { cursor: "c", poll_after_ms: 5000 } } }),
    });

    const widget = boot(win, document);
    await settle();
    await settle();
    const root = shadow() as ShadowRoot;
    root.dispatchEvent(new CustomEvent("dimmi:converse", { detail: { conversation: "conv_1" } }));
    for (let i = 0; i < 4; i++) {
      await settle();
    }

    expect(root.querySelector('[data-ai="helped"]')).toBeNull();
    expect(root.querySelector(".ai-actions")?.textContent).toContain("Talk to a person");
    root.querySelector<HTMLButtonElement>('[data-ai="handoff"]')?.click();
    await settle();

    expect(calls).toContain("POST /conversations/conv_1/ai/handoff");
    expect(widget?.live?.store.conversations[0]?.ai_state).toBe("handed_off");
    expect(root.querySelector(".ai-actions")).toBeNull();
  });

  it("tells the server when a single-page app moves to another route", async () => {
    localStorage.setItem("dimmi:wgt_1:token", "dmi_ws_1");
    const pings: unknown[] = [];
    serve({
      "GET /apps/wgt_1/config": () => ({ status: 200, body: config }),
      "GET /apps/wgt_1/availability": () => ({ status: 200, body: availability }),
      "GET /session": () => ({ status: 200, body: session }),
      "GET /conversations": () => ({ status: 200, body: { data: [] } }),
      "POST /realtime-tokens": () => ({ status: 503, body: {} }),
      "GET /updates": () => ({ status: 200, body: { data: [], meta: { cursor: "c", poll_after_ms: 5000 } } }),
      "POST /pings": (body) => {
        pings.push(body);
        return { status: 202 };
      },
    });

    boot(win, document);
    await settle();
    await settle();
    history.pushState({}, "", "/pricing?plan=pro");
    await new Promise((resolve) => setTimeout(resolve, 1100));

    expect(pings).toHaveLength(1);
    expect(JSON.stringify(pings[0])).toContain("/pricing?plan=pro");
  });
});
