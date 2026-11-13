import "./lib/thread_parts";
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
    return new Response(answer.body === undefined ? "" : JSON.stringify(answer.body), {
      status: answer.status,
      headers: { "Content-Type": "application/json" },
    });
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

  it("fills the Help tab from the linked help center", async () => {
    const calls = serve({
      "GET /apps/wgt_1/config": () => ({
        status: 200,
        body: { data: { ...config.data, tabs: ["home", "messages", "help"] } },
      }),
      "GET /apps/wgt_1/availability": () => ({ status: 200, body: availability }),
      "GET /apps/wgt_1/help": () => ({
        status: 200,
        body: {
          data: {
            collections: [{ slug: "start", title: "Getting started", description: "", articles: ["invite"] }],
            popular: ["invite"],
            articles: [
              {
                slug: "invite",
                title: "Invite your team",
                summary: "",
                updated: "2026-01-01T00:00:00Z",
                body: [{ kind: "p", text: "Open settings." }],
              },
            ],
          },
        },
      }),
    });

    boot(win, document);
    await settle();
    await settle();
    await settle();

    expect(calls).toContain("GET /apps/wgt_1/help");
    expect(shadow()?.textContent).toContain("Getting started");
    expect(shadow()?.textContent).toContain("Invite your team");
  });

  it("reads the Help tab as a known visitor and opens an article in the help center signed in", async () => {
    localStorage.setItem("dimmi:wgt_1:token", "dmi_ws_1");
    const help = {
      data: {
        url: "https://acme.dimmi.help",
        collections: [],
        popular: ["priority"],
        articles: [
          { slug: "priority", title: "Priority support", summary: "", updated: "2026-01-01T00:00:00Z", body: [] },
        ],
      },
    };
    const calls = serve({
      "GET /apps/wgt_1/config": () => ({
        status: 200,
        body: { data: { ...config.data, tabs: ["home", "messages", "help"] } },
      }),
      "GET /apps/wgt_1/availability": () => ({ status: 200, body: availability }),
      "GET /session": () => ({ status: 200, body: session }),
      "GET /conversations": () => ({ status: 200, body: { data: [] } }),
      "GET /help": () => ({ status: 200, body: help }),
      "POST /help-passes": () => ({
        status: 201,
        body: { data: { object: "help_pass", url: "https://acme.dimmi.help/sso?next=%2F#pass=dmi_hp_1" } },
      }),
      "POST /realtime-tokens": () => ({ status: 503, body: {} }),
      "GET /updates": () => ({ status: 200, body: { data: [], meta: { cursor: "c", poll_after_ms: 5000 } } }),
    });
    const tab = { opener: {}, location: { href: "about:blank" } };
    vi.spyOn(window, "open").mockImplementation(() => tab as unknown as Window);

    boot(win, document);
    for (let i = 0; i < 4; i++) {
      await settle();
    }

    expect(calls).toContain("GET /help");
    expect(calls).not.toContain("GET /apps/wgt_1/help");
    const link = shadow()?.querySelector<HTMLAnchorElement>('a[data-help-article="priority"]');
    expect(link?.href).toBe("https://acme.dimmi.help/articles/priority");
    link?.click();
    await settle();
    await settle();

    expect(calls).toContain("POST /help-passes");
    expect(tab.opener).toBeNull();
    expect(tab.location.href).toBe("https://acme.dimmi.help/sso?next=%2F#pass=dmi_hp_1");
  });

  it("fills the What's new tab from the changelog and drops its dot once the visitor reads it", async () => {
    localStorage.setItem("dimmi:wgt_1:changelog_seen", "2026-02-01T00:00:00Z");
    const calls = serve({
      "GET /apps/wgt_1/config": () => ({
        status: 200,
        body: { data: { ...config.data, tabs: ["home", "messages", "changelog"] } },
      }),
      "GET /apps/wgt_1/availability": () => ({ status: 200, body: availability }),
      "GET /apps/wgt_1/changelog": () => ({
        status: 200,
        body: {
          data: {
            url: "https://acme.dimmi.help/changelog",
            entries: [
              {
                slug: "dark-mode",
                title: "Dark mode",
                summary: "Easy on the eyes.",
                label: "New",
                published_at: "2026-03-01T00:00:00Z",
                body: [{ kind: "p", text: "Switch it on." }],
              },
              {
                slug: "exports",
                title: "Exports",
                summary: "",
                label: "",
                published_at: "2026-01-01T00:00:00Z",
                body: [],
              },
            ],
          },
        },
      }),
    });

    boot(win, document);
    // The changelog loads after the config; a busy machine needs more turns.
    for (let i = 0; i < 20 && !shadow()?.textContent?.includes("Switch it on."); i++) {
      await settle();
    }

    expect(calls).toContain("GET /apps/wgt_1/changelog");
    const tab = shadow()?.querySelector<HTMLElement>('[role=tab][data-tab="changelog"]');
    expect(tab?.querySelector(".dot")).not.toBeNull();
    expect(shadow()?.textContent).toContain("Dark mode");
    expect(shadow()?.textContent).toContain("Switch it on.");

    tab?.click();
    expect(tab?.querySelector(".dot")).toBeNull();
    expect(localStorage.getItem("dimmi:wgt_1:changelog_seen")).toBe("2026-03-01T00:00:00Z");
  });

  it("sends the visitor's answer about an article, starting their session first", async () => {
    const ratings: unknown[] = [];
    const help = {
      data: {
        collections: [],
        popular: ["invite"],
        articles: [{ slug: "invite", title: "Invite", summary: "", updated: "2026-01-01T00:00:00Z", body: [] }],
      },
    };
    serve({
      "GET /apps/wgt_1/config": () => ({
        status: 200,
        body: { data: { ...config.data, tabs: ["home", "messages", "help"] } },
      }),
      "GET /apps/wgt_1/availability": () => ({ status: 200, body: availability }),
      "GET /apps/wgt_1/help": () => ({ status: 200, body: help }),
      "POST /sessions": () => ({ status: 201, body: session }),
      "GET /help": () => ({ status: 200, body: help }),
      "POST /article-ratings": (body) => {
        ratings.push(body);
        return { status: 200 };
      },
    });

    boot(win, document);
    for (let i = 0; i < 3; i++) {
      await settle();
    }
    shadow()?.querySelector<HTMLElement>('[data-view="article:invite"] [data-helpful="no"]')?.click();
    for (let i = 0; i < 3; i++) {
      await settle();
    }

    expect(ratings).toEqual([{ article: "invite", helpful: false }]);
    expect(shadow()?.querySelector<HTMLElement>('[data-view="article:invite"] .feedback')?.dataset.rated).toBe("no");
  });

  it("shows nothing and says why when the server does not serve the app here", async () => {
    serve({ "GET /apps/wgt_1/config": () => ({ status: 403, body: { code: "origin_not_allowed" } }) });
    const error = vi.spyOn(console, "error").mockImplementation(() => undefined);

    boot(win, document);
    await settle();

    expect(shadow()).toBeUndefined();
    expect(error.mock.calls.flat().join(" ")).toContain("origin_not_allowed");
  });

  it("stays hidden from visitors who are not signed in when sign-in is required", async () => {
    serve({
      "GET /apps/wgt_1/config": () => ({
        status: 200,
        body: { data: { ...config.data, features: { identity_mode: "required" } } },
      }),
      "GET /apps/wgt_1/availability": () => ({ status: 200, body: availability }),
    });
    const warn = vi.spyOn(console, "warn").mockImplementation(() => undefined);

    boot(win, document);
    await settle();
    await settle();

    expect(shadow()).toBeUndefined();
    expect(warn.mock.calls.flat().join(" ")).toContain("identity_required");
  });

  it("keeps the new session when a visitor signs in mid-visit, so the next requests carry it", async () => {
    win.__dimmi = { appId: "wgt_1", apiBase: "https://app.example.com", identityUrl: "/me/dimmi-token" };
    const payload = btoa(JSON.stringify({ sub: "ctc_9" })).replace(/=+$/, "");
    const token: { value: string | null } = { value: null };
    const bearers: string[] = [];
    serve({
      "GET /apps/wgt_1/config": () => ({ status: 200, body: config }),
      "GET /apps/wgt_1/availability": () => ({ status: 200, body: availability }),
      "GET /me/dimmi-token": () => ({ status: 200, body: { token: token.value } }),
      "POST /sessions": () => ({
        status: 201,
        body: { data: { id: "wss_9", token: "dmi_ws_9", kind: "identified", contact: { id: "ctc_9" } } },
      }),
      "GET /conversations": () => ({ status: 200, body: { data: [], has_more: false } }),
      "POST /realtime-tokens": () => ({ status: 201, body: { data: { token: "t", url: "https://rt", channel: "c" } } }),
    });
    const fetch = globalThis.fetch as unknown as ReturnType<typeof vi.fn>;

    boot(win, document);
    await settle();
    await settle();
    token.value = `e30.${payload}.sig`;
    win.Dimmi?.("identify");
    await vi.waitFor(() => expect(localStorage.getItem("dimmi:wgt_1:token")).toBe("dmi_ws_9"));
    await vi.waitFor(() => expect(fetch.mock.calls.some(([url]) => String(url).includes("/conversations"))).toBe(true));

    for (const [url, init] of fetch.mock.calls as [string, RequestInit][]) {
      if (String(url).includes("/conversations")) {
        bearers.push((init.headers as Record<string, string>).Authorization ?? "none");
      }
    }
    expect(bearers).not.toHaveLength(0);
    expect(bearers.every((b) => b === "Bearer dmi_ws_9")).toBe(true);
    expect(localStorage.getItem("dimmi:wgt_1:token")).toBe("dmi_ws_9");
  });

  it("does not ask who is signed in when sign-in is off", async () => {
    win.__dimmi = { appId: "wgt_1", apiBase: "https://app.example.com", identityUrl: "/me/dimmi-token" };
    const calls = serve({
      "GET /apps/wgt_1/config": () => ({
        status: 200,
        body: { data: { ...config.data, features: { identity_mode: "off" } } },
      }),
      "GET /apps/wgt_1/availability": () => ({ status: 200, body: availability }),
    });
    const warn = vi.spyOn(console, "warn").mockImplementation(() => undefined);

    boot(win, document);
    await settle();
    await settle();

    expect(shadow()?.textContent).toContain("Ask us anything");
    expect(calls.some((call) => call.includes("dimmi-token"))).toBe(false);
    expect(warn.mock.calls.flat().join(" ")).toContain("identity_mode_off");
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
    expect(thread.querySelector(".byline")?.textContent).toContain("AI");
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

  it("confirms an action card, then checks on it until the team decides", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    localStorage.setItem("dimmi:wgt_1:token", "dmi_ws_1");
    let state = "pending_approval";
    const card = {
      id: "msg_2",
      conversation_id: "conv_1",
      author: { type: "agent", name: "Ada", avatar_url: null },
      body: "Refund $144.00 for invoice inv_1001?",
      attachments: [],
      created_at: "2026-01-01T00:00:02Z",
      ai: {
        generated: true,
        confirmation: {
          action_call_id: "aax_1",
          title: "Refund an invoice",
          text: "Refund $144.00 for invoice inv_1001?",
          status: "awaiting_confirmation",
          details: [],
        },
      },
    };
    const call = () => ({
      status: 200,
      body: { data: { id: "aax_1", title: "Refund", summary: "", state, status: state } },
    });
    const calls = serve({
      "GET /apps/wgt_1/config": () => ({ status: 200, body: config }),
      "GET /apps/wgt_1/availability": () => ({ status: 200, body: availability }),
      "GET /session": () => ({ status: 200, body: session }),
      "GET /conversations": () => ({ status: 200, body: { data: [{ ...conversation, ai_state: "active" }] } }),
      "GET /conversations/conv_1/messages": () => ({ status: 200, body: { data: [card] } }),
      "PATCH /conversations/conv_1/read-state": () => ({ status: 204 }),
      "PATCH /conversations/conv_1/action-calls/aax_1": call,
      "GET /conversations/conv_1/action-calls/aax_1": call,
      "POST /realtime-tokens": () => ({ status: 503, body: {} }),
      "GET /updates": () => ({ status: 200, body: { data: [], meta: { cursor: "c", poll_after_ms: 60000 } } }),
    });

    boot(win, document);
    await settle();
    await settle();
    const root = shadow() as ShadowRoot;
    root.dispatchEvent(new CustomEvent("dimmi:converse", { detail: { conversation: "conv_1" } }));
    for (let i = 0; i < 4; i++) {
      await settle();
    }
    const messenger = root.querySelector<HTMLElement>("[data-tab]");
    if (messenger) {
      messenger.dataset.open = "true";
      messenger.dataset.tab = "conversation";
    }

    expect(root.querySelector(".bubble")).toBeNull();
    root.querySelector<HTMLButtonElement>('[data-decision="confirmed"]')?.click();
    await settle();
    await settle();
    expect(calls).toContain("PATCH /conversations/conv_1/action-calls/aax_1");
    expect(root.querySelector(".action-state")?.textContent).toBe("Waiting for the team to review this");

    state = "succeeded";
    await vi.advanceTimersByTimeAsync(5100);
    await settle();
    expect(root.querySelector(".action-state")?.textContent).toBe("Done");

    const checks = calls.filter((c) => c === "GET /conversations/conv_1/action-calls/aax_1").length;
    await vi.advanceTimersByTimeAsync(15000);
    expect(calls.filter((c) => c === "GET /conversations/conv_1/action-calls/aax_1")).toHaveLength(checks);
    vi.useRealTimers();
  });

  it("confirms the visitor's email with a code before an action runs, never as a message", async () => {
    localStorage.setItem("dimmi:wgt_1:token", "dmi_ws_1");
    let state = "awaiting_verification";
    const sent: Record<string, unknown> = {};
    const card = {
      id: "msg_2",
      conversation_id: "conv_1",
      author: { type: "agent", name: "Ada", avatar_url: null },
      body: "I can look that order up once you confirm your email.",
      attachments: [],
      created_at: "2026-01-01T00:00:02Z",
      ai: {
        generated: true,
        confirmation: {
          action_call_id: "aax_1",
          title: "Look up an order",
          text: "Look up an order?",
          status: "awaiting_verification",
          details: [],
        },
      },
    };
    const calls = serve({
      "GET /apps/wgt_1/config": () => ({ status: 200, body: config }),
      "GET /apps/wgt_1/availability": () => ({ status: 200, body: availability }),
      "GET /session": () => ({ status: 200, body: session }),
      "GET /conversations": () => ({ status: 200, body: { data: [{ ...conversation, ai_state: "active" }] } }),
      "GET /conversations/conv_1/messages": () => ({ status: 200, body: { data: [card] } }),
      "PATCH /conversations/conv_1/read-state": () => ({ status: 204 }),
      "POST /email-verifications": (body) => {
        sent.email = body;
        return { status: 202, body: { data: { email: "a***@acme.test" } } };
      },
      "POST /email-verifications/checks": (body) => {
        sent.code = body;
        state = "awaiting_confirmation";
        return { status: 200, body: session };
      },
      "GET /conversations/conv_1/action-calls/aax_1": () => ({
        status: 200,
        body: { data: { id: "aax_1", title: "Look up", summary: "", state, status: state } },
      }),
      "POST /realtime-tokens": () => ({ status: 503, body: {} }),
      "GET /updates": () => ({ status: 200, body: { data: [], meta: { cursor: "c", poll_after_ms: 60000 } } }),
    });

    boot(win, document);
    await settle();
    await settle();
    const root = shadow() as ShadowRoot;
    root.dispatchEvent(new CustomEvent("dimmi:converse", { detail: { conversation: "conv_1" } }));
    for (let i = 0; i < 4; i++) {
      await settle();
    }

    const emailForm = root.querySelector<HTMLFormElement>('[data-verify-step="email"]');
    const emailInput = emailForm?.querySelector<HTMLInputElement>("input");
    if (emailInput) {
      emailInput.value = "ada@acme.test";
    }
    emailForm?.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    await settle();
    await settle();
    expect(sent.email).toEqual({ email: "ada@acme.test" });
    expect(root.textContent).toContain("a***@acme.test");

    const codeForm = root.querySelector<HTMLFormElement>('[data-verify-step="code"]');
    const codeInput = codeForm?.querySelector<HTMLInputElement>("input");
    if (codeInput) {
      codeInput.value = "123456";
    }
    codeForm?.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    for (let i = 0; i < 4; i++) {
      await settle();
    }
    expect(sent.code).toEqual({ code: "123456" });
    expect(calls.filter((c) => c.startsWith("POST /conversations/conv_1/messages"))).toHaveLength(0);
    expect(root.querySelector('[data-decision="confirmed"]')).not.toBeNull();
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
