import { boot } from "./core";
import { when } from "./lib/map";
import type { DimmiWindow } from "./settings";

type TestWindow = Window & DimmiWindow;

const win = window as TestWindow;
const api = "https://app.example.com/widget/v1";

/*
 * serve answers the config in the language asked for, and records the
 * addresses asked, query and all.
 */
function serve() {
  const calls: string[] = [];
  const words: Record<string, { texts: Record<string, string>; dir: string }> = {
    fr: { texts: { greeting: "Comment pouvons-nous vous aider ?", home: "Accueil" }, dir: "ltr" },
    ar: { texts: { greeting: "كيف يمكننا مساعدتك؟", home: "الرئيسية" }, dir: "rtl" },
  };
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string | URL | Request) => {
      const address = new URL(String(url));
      calls.push(String(url).replace(api, ""));
      const locale = address.searchParams.get("locale") ?? "en";
      const body = address.pathname.endsWith("/config")
        ? {
            data: {
              company: { name: "Shop" },
              tabs: ["home", "messages"],
              locale: words[locale] ? locale : "en",
              dir: words[locale]?.dir ?? "ltr",
              texts: words[locale]?.texts ?? {},
            },
          }
        : { data: { online: true, team: [] } };
      return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } });
    }),
  );

  return calls;
}

const settle = async (times = 4) => {
  for (let i = 0; i < times; i++) {
    await new Promise((resolve) => setTimeout(resolve, 0));
  }
};
const panel = () =>
  document.getElementById("dimmi-messenger")?.shadowRoot?.querySelector<HTMLElement>(".panel") as HTMLElement;

beforeEach(() => {
  delete win.Dimmi;
  win.__dimmi = { appId: "wgt_1", apiBase: "https://app.example.com" };
  document.body.innerHTML = "";
  localStorage.clear();
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("the messenger's language", () => {
  it("asks for the browser's language and reads left to right in it", async () => {
    const calls = serve();
    vi.spyOn(navigator, "language", "get").mockReturnValue("fr-FR");

    boot(win, document);
    await settle();

    expect(calls).toContain("/apps/wgt_1/config?locale=fr-FR");
    expect(calls).toContain("/apps/wgt_1/availability?locale=fr-FR");
  });

  it("takes the language a page queued with boot, and turns right to left for Arabic", async () => {
    const calls = serve();
    const queue = Object.assign((...args: unknown[]) => queue.q.push(args as never), { q: [] as never[] });
    win.Dimmi = queue as unknown as TestWindow["Dimmi"];
    win.Dimmi?.("boot", { language: "ar" });

    boot(win, document);
    await settle();

    expect(calls[0]).toBe("/apps/wgt_1/config?locale=ar");
    expect(panel().dir).toBe("rtl");
    expect(panel().lang).toBe("ar");
    expect(panel().textContent).toContain("كيف يمكننا مساعدتك؟");
  });

  it("loads the words again when the page changes the language later", async () => {
    const calls = serve();
    vi.spyOn(navigator, "language", "get").mockReturnValue("en-US");

    boot(win, document);
    await settle();
    expect(panel().dir).toBe("ltr");

    win.Dimmi?.("update", { language: "fr" });
    await settle();

    expect(calls).toContain("/apps/wgt_1/config?locale=fr");
    expect(panel().lang).toBe("fr");
    expect(panel().textContent).toContain("Comment pouvons-nous vous aider ?");

    win.Dimmi?.("update", { language: "ar" });
    await settle();
    expect(panel().dir).toBe("rtl");
  });

  it("writes times and dates the way the visitor's language does", async () => {
    serve();
    win.Dimmi = Object.assign(() => undefined, { q: [["boot", { language: "ar" }]] }) as unknown as TestWindow["Dimmi"];

    boot(win, document);
    await settle();

    const now = Date.parse("2026-03-04T12:00:00");
    expect(when("2026-03-04T09:05:00", now)).toBe(
      new Date("2026-03-04T09:05:00").toLocaleTimeString("ar", {
        hour: "2-digit",
        minute: "2-digit",
        hourCycle: "h23",
      }),
    );
    expect(when("2026-02-01T09:05:00", now)).toBe(
      new Date("2026-02-01T09:05:00").toLocaleDateString("ar", { month: "short", day: "numeric" }),
    );
  });
});
