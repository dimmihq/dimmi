// @vitest-environment jsdom
import { getDimmi, loadDimmi, loaderUrl, resetDimmi, UNREAD_EVENT } from "./index";

type Queued = Window & { Dimmi?: ((...command: unknown[]) => void) & { q?: unknown[][] } };
const win = window as Queued;

beforeEach(() => {
  document.body.innerHTML = "";
  delete win.Dimmi;
  resetDimmi();
});

describe("loadDimmi", () => {
  it("adds the loader once with the app id, API base and nonce", () => {
    loadDimmi({ appId: "wgt_1", host: "https://dimmi.acme.com/", apiBase: "https://proxy.acme.com/", nonce: "abc" });
    loadDimmi({ appId: "wgt_1", host: "https://dimmi.acme.com" });
    const scripts = document.querySelectorAll<HTMLScriptElement>("script[data-dimmi-loader]");
    expect(scripts).toHaveLength(1);
    expect(scripts[0].src).toBe("https://dimmi.acme.com/widget/v1/loader.js");
    expect(scripts[0].dataset.appId).toBe("wgt_1");
    expect(scripts[0].dataset.apiBase).toBe("https://proxy.acme.com");
    expect(scripts[0].nonce).toBe("abc");
  });

  it("queues identity and settings before the messenger arrives", async () => {
    const identity = () => "token";
    loadDimmi({ appId: "wgt_1", host: "https://d.test", identity, settings: { appearance: { position: "left" } } });
    const q = win.Dimmi?.q ?? [];
    expect(q.map(([name]) => name)).toEqual(["identify", "boot"]);
    await expect((q[0][1] as () => Promise<string>)()).resolves.toBe("token");
    expect(q[1][1]).toEqual({ appearance: { position: "left" } });
  });

  it("resolves ready on load and rejects when blocked", async () => {
    const dimmi = loadDimmi({ appId: "wgt_1", host: "https://d.test" });
    document.querySelector("script")?.dispatchEvent(new Event("load"));
    await expect(dimmi.ready).resolves.toBeUndefined();

    document.body.innerHTML = "";
    resetDimmi();
    const blocked = loadDimmi({ appId: "wgt_1", host: "https://d.test" });
    document.querySelector("script")?.dispatchEvent(new Event("error"));
    await expect(blocked.ready).rejects.toThrow(/did not load/);
  });

  it("needs an app id and host", () => {
    expect(() => loadDimmi({ appId: "", host: "https://d.test" })).toThrow(/appId and host/);
  });
});

describe("the client", () => {
  it("sends the messenger's commands", () => {
    const dimmi = getDimmi();
    dimmi.show();
    dimmi.hide();
    dimmi.toggle();
    dimmi.showNewMessage();
    dimmi.showNewMessage("Hi");
    dimmi.update({ appearance: { theme: "dark" } });
    dimmi.identify();
    dimmi.trackEvent("signed_up", { plan: "pro" });
    dimmi.page();
    dimmi.shutdown();
    expect(win.Dimmi?.q).toEqual([
      ["open"],
      ["close"],
      ["toggle"],
      ["open"],
      ["ask", "Hi"],
      ["update", { appearance: { theme: "dark" } }],
      ["identify"],
      ["track", "signed_up", { plan: "pro" }],
      ["logout"],
    ]);
  });

  it("reports the unread count until unsubscribed", () => {
    const counts: number[] = [];
    const stop = getDimmi().onUnreadCountChange((n) => counts.push(n));
    window.dispatchEvent(new CustomEvent(UNREAD_EVENT, { detail: { count: 3 } }));
    stop();
    window.dispatchEvent(new CustomEvent(UNREAD_EVENT, { detail: { count: 4 } }));
    expect(counts).toEqual([3]);
  });

  it("hands calls to a messenger that already loaded", () => {
    const calls: unknown[][] = [];
    win.Dimmi = (...command: unknown[]) => void calls.push(command);
    getDimmi().show();
    expect(calls).toEqual([["open"]]);
  });
});

it("builds the loader URL for a self-hosted install", () => {
  expect(loaderUrl("http://localhost:8080//")).toBe("http://localhost:8080/widget/v1/loader.js");
});
