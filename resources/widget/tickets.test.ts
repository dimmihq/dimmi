import "./lib/thread_parts";
import "./lib/ticket_parts";
import { boot } from "./core";
import { Strip } from "./lib/ticket_parts";
import type { DimmiWindow } from "./settings";

type TestWindow = Window & DimmiWindow;

const win = window as TestWindow;
const api = "https://app.example.com/widget/v1";

/*
 * serve answers the messenger's requests from a table of routes and
 * records what it was asked, with the bodies it was sent.
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
  onmessage: ((event: { data: string }) => void) | null = null;
  onerror: ((event: unknown) => void) | null = null;
  close() {
    return;
  }
}

const config = {
  data: { company: { name: "Shop" }, texts: { greeting: "Ask us anything" }, tabs: ["home", "messages"] },
};
const bug = {
  id: "tktt_1",
  name: "Bug report",
  icon: "bug",
  description: "Something is broken",
  fields: [
    {
      key: "steps",
      label: "Steps to reproduce",
      data_type: "long_text",
      options: [],
      required: true,
      editable: true,
      show_when: null,
    },
    {
      key: "browser",
      label: "Browser",
      data_type: "short_text",
      options: [],
      required: false,
      editable: true,
      show_when: { field: "steps" },
    },
  ],
};

const settle = async (times = 6) => {
  for (let i = 0; i < times; i++) {
    await new Promise((resolve) => setTimeout(resolve, 0));
  }
};
const shadow = () => document.getElementById("dimmi-messenger")?.shadowRoot as ShadowRoot;

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

describe("requests in the messenger", () => {
  it("shows the request types on Home once the panel opens", async () => {
    const calls = serve({
      "GET /apps/wgt_1/config": () => ({ status: 200, body: config }),
      "GET /apps/wgt_1/ticket-types": () => ({ status: 200, body: { data: [bug] } }),
    });

    const widget = boot(win, document);
    await widget?.ready;
    await settle();
    expect(calls.map((c) => c.key)).not.toContain("GET /apps/wgt_1/ticket-types");

    widget?.messenger?.open();
    await settle();

    const home = shadow().querySelector('.view[data-view="home"]');
    expect(home?.textContent).toContain("Get help with");
    expect(home?.querySelector("[data-ticket-type]")?.textContent).toContain("Bug report");
    expect(calls.map((c) => c.key)).not.toContain("GET /apps/wgt_1/incidents");
  });

  it("checks the required fields, then sends the form once with its client id", async () => {
    const sent: unknown[] = [];
    serve({
      "GET /apps/wgt_1/config": () => ({ status: 200, body: config }),
      "GET /apps/wgt_1/ticket-types": () => ({ status: 200, body: { data: [bug] } }),
      "POST /sessions": () => ({
        status: 201,
        body: { data: { id: "wss_1", token: "dmi_ws_1", kind: "anonymous", contact: { id: "ctc_1" } } },
      }),
      "POST /tickets": (body) => {
        sent.push(body);
        return { status: 201, body: { data: { id: "tkt_1", conversation_id: "conv_1" } } };
      },
      "GET /conversations": () => ({ status: 200, body: { data: [] } }),
    });

    const widget = boot(win, document);
    await widget?.ready;
    widget?.messenger?.open();
    await settle();
    shadow().querySelector<HTMLElement>("[data-ticket-type]")?.click();
    await settle();

    const form = shadow().querySelector<HTMLFormElement>("[data-ticket-form]");
    expect(form).toBeTruthy();
    expect(form?.querySelector<HTMLElement>('[data-field="browser"]')?.hidden).toBe(true);

    form?.requestSubmit();
    await settle();
    expect(sent).toHaveLength(0);
    expect(form?.querySelectorAll("[data-invalid]").length).toBe(2);

    (form?.querySelector('[name="title"]') as HTMLInputElement).value = "Export is broken";
    (form?.querySelector('[name="field.steps"]') as HTMLTextAreaElement).value = "Open reports";
    form?.dispatchEvent(new Event("input"));
    expect(form?.querySelector<HTMLElement>('[data-field="browser"]')?.hidden).toBe(false);
    form?.requestSubmit();
    await settle(10);

    expect(sent).toEqual([
      expect.objectContaining({ type_id: "tktt_1", title: "Export is broken", fields: { steps: "Open reports" } }),
    ]);
    expect((sent[0] as { client_id: string }).client_id).toMatch(/^[0-9a-f-]{36}$/);
  });

  it("draws the progress strip with the current stop marked", () => {
    const markup = String(
      Strip({
        ticket: { reference: "T-482", title: "Refund", state: { label: "With billing", group: "waiting_team" } },
        texts: {},
      }),
    );

    expect(markup).toContain("T-482");
    expect(markup).toContain("With billing");
    expect(markup.match(/data-done="true"/g)).toHaveLength(2);
    expect(markup).toContain('aria-current="step"><i></i><span>We&#39;re on it');
  });
});
