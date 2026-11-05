import { boot } from "./core";
import { load } from "./loader";
import type { DimmiWindow } from "./settings";

type TestWindow = Window & DimmiWindow & { __DIMMI_BUILD__?: unknown };

const win = window as TestWindow;

function script(attributes: Record<string, string>) {
  const tag = document.createElement("script");
  tag.src = "https://app.example.com/support/widget/v1/loader.js";
  for (const [name, value] of Object.entries(attributes)) {
    tag.setAttribute(name, value);
  }
  return tag;
}

function shadow() {
  return document.getElementById("dimmi-messenger")?.shadowRoot as ShadowRoot;
}

beforeEach(() => {
  delete win.Dimmi;
  delete win.__dimmi;
  document.head.innerHTML = "";
  document.body.innerHTML = "";
  win.__DIMMI_BUILD__ = { core: "core-abc.js", integrity: "sha384-abc", assets: "" };
  vi.stubGlobal("__DIMMI_BUILD__", win.__DIMMI_BUILD__);
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("loader", () => {
  it("reads its tag and adds the core from its own host", () => {
    load(win, document, script({ "data-app-id": "wgt_1", "data-identity-url": "/api/dimmi/token" }));

    expect(win.__dimmi).toEqual({
      appId: "wgt_1",
      apiBase: "https://app.example.com/support",
      identityUrl: "/api/dimmi/token",
      preview: false,
    });
    const core = document.head.querySelector("script") as HTMLScriptElement;
    expect(core.type).toBe("module");
    expect(core.src).toBe("https://app.example.com/support/widget/assets/core-abc.js");
    expect(core.integrity).toBe("sha384-abc");
    expect(core.crossOrigin).toBe("anonymous");
  });

  it("fetches the core from the assets host and calls the API through a proxy", () => {
    vi.stubGlobal("__DIMMI_BUILD__", {
      core: "core-abc.js",
      integrity: "",
      assets: "https://cdn.example.com/widget/assets/",
    });

    load(win, document, script({ "data-app-id": "wgt_1", "data-api-base": "/_dimmi" }));

    expect(win.__dimmi?.apiBase).toBe("/_dimmi");
    expect(document.head.querySelector("script")?.src).toBe("https://cdn.example.com/widget/assets/core-abc.js");
  });

  it("keeps calls made before the core arrives, including the snippet's own", () => {
    const snippet = Object.assign((...command: [string]) => snippet.q.push(command), { q: [["open"]] as [string][] });
    win.Dimmi = snippet;

    load(win, document, script({ "data-app-id": "wgt_1" }));
    win.Dimmi?.("toggle");

    expect(win.Dimmi?.q).toEqual([["open"], ["toggle"]]);
  });

  it("warns and does nothing when the snippet is on the page twice", () => {
    const warn = vi.spyOn(console, "warn").mockReturnValue(undefined);
    load(win, document, script({ "data-app-id": "wgt_1" }));

    load(win, document, script({ "data-app-id": "wgt_1" }));

    expect(document.head.querySelectorAll("script")).toHaveLength(1);
    expect(warn).toHaveBeenCalledWith(expect.stringContaining("[dimmi] boot_twice"));
  });

  it("says so in the console when the core cannot load", () => {
    const error = vi.spyOn(console, "error").mockReturnValue(undefined);
    load(win, document, script({ "data-app-id": "wgt_1" }));

    document.head.querySelector("script")?.dispatchEvent(new Event("error"));

    expect(error).toHaveBeenCalledWith(expect.stringContaining("[dimmi] assets_blocked"));
  });
});

describe("core", () => {
  beforeEach(() => {
    win.__dimmi = { appId: "wgt_1", apiBase: "https://app.example.com", preview: true };
  });

  const launcher = () => shadow().querySelector(".launcher") as HTMLButtonElement;
  const panel = () => shadow().querySelector(".panel") as HTMLElement;
  const tab = (name: string) => shadow().querySelector(`[role=tab][data-tab=${name}]`) as HTMLButtonElement;

  it("mounts in a shadow root and runs the calls queued before it", () => {
    win.Dimmi = Object.assign(() => undefined, { q: [["open"]] as [string][] });

    const widget = boot(win, document);

    expect(widget?.messenger?.isOpen).toBe(true);
    expect(panel().hasAttribute("inert")).toBe(false);
    expect(launcher().getAttribute("aria-expanded")).toBe("true");
    expect(launcher().getAttribute("aria-label")).toBe("Close messages");
  });

  it("answers the global function, the launcher, its close buttons and page elements marked data-dimmi", () => {
    const widget = boot(win, document);
    const toggle = document.createElement("a");
    toggle.dataset.dimmi = "toggle";
    document.body.append(toggle);

    win.Dimmi?.("open");
    expect(widget?.messenger?.isOpen).toBe(true);
    launcher().click();
    expect(widget?.messenger?.isOpen).toBe(false);
    expect(panel().hasAttribute("inert")).toBe(true);
    toggle.click();
    expect(widget?.messenger?.isOpen).toBe(true);
    (shadow().querySelector('[data-action="click->close"]') as HTMLButtonElement).click();
    expect(widget?.messenger?.isOpen).toBe(false);
  });

  it("closes on Escape and hands focus back to the launcher", () => {
    const widget = boot(win, document);
    win.Dimmi?.("open");

    panel().dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));

    expect(widget?.messenger?.isOpen).toBe(false);
    expect(shadow().activeElement).toBe(launcher());
  });

  it("switches views from the tab bar and from the cards on the home view", () => {
    const widget = boot(win, document);

    tab("help").click();
    expect(widget?.messenger?.tab).toBe("help");
    expect(tab("help").getAttribute("aria-selected")).toBe("true");
    expect(tab("home").getAttribute("aria-selected")).toBe("false");

    tab("home").click();
    (shadow().querySelector(".chat strong") as HTMLElement).click();
    expect(widget?.messenger?.tab).toBe("conversation");
  });

  it("grows the panel toward the full window and back", () => {
    boot(win, document);
    const expand = shadow().querySelector('[data-action="click->expand"]') as HTMLButtonElement;

    expand.click();
    expect(shadow().querySelector(".messenger")?.getAttribute("data-expanded")).toBe("true");
    expect(expand.getAttribute("aria-label")).toBe("Shrink messages");
    expand.click();
    expect(shadow().querySelector(".messenger")?.getAttribute("data-expanded")).toBe("false");
  });

  it("opens a conversation over the tabs and goes back to the tab it came from", () => {
    const widget = boot(win, document);
    tab("messages").click();

    (shadow().querySelector('[data-view="messages"] .loaded .row') as HTMLButtonElement).click();
    expect(widget?.messenger?.tab).toBe("conversation");
    expect(shadow().querySelectorAll('[data-view="conversation"] .said').length).toBeGreaterThan(3);

    (shadow().querySelector('[data-action="click->back"]') as HTMLButtonElement).click();
    expect(widget?.messenger?.tab).toBe("messages");
  });

  describe("unread messages", () => {
    const notice = () => shadow().querySelector(".notice") as HTMLElement;
    const badge = () => shadow().querySelector('.launcher [data-target="count"]') as HTMLElement;
    const tabBadge = () => shadow().querySelector('[data-target="tab-count"]') as HTMLElement;
    const rows = () => [...shadow().querySelectorAll<HTMLElement>('[data-conversation="44308"]')];

    it("previews a new message above the closed launcher and counts it as unread", () => {
      boot(win, document);

      win.Dimmi?.("simulate", { message: "The sync finished." });

      expect(notice().hidden).toBe(false);
      expect(notice().querySelector(".notice-body")?.textContent).toBe("The sync finished.");
      expect(badge().hidden).toBe(false);
      expect(badge().textContent).toBe("1");
      expect(tabBadge().textContent).toBe("1");
      expect(launcher().getAttribute("aria-label")).toBe("Open messages, 1 unread");
      expect(rows().every((row) => row.dataset.unread === "true")).toBe(true);
      expect(rows()[0]?.querySelector("small")?.textContent).toBe("The sync finished.");
      expect([...shadow().querySelectorAll(".thread .bubble")].at(-1)?.textContent).toBe("The sync finished.");

      win.Dimmi?.("simulate", { message: "Still there?" });
      expect(badge().textContent).toBe("2");
    });

    it("opens the conversation from the preview and reads everything", () => {
      const widget = boot(win, document);
      win.Dimmi?.("simulate", { message: "The sync finished." });

      (notice().querySelector(".notice-card") as HTMLButtonElement).click();

      expect(widget?.messenger?.isOpen).toBe(true);
      expect(widget?.messenger?.tab).toBe("conversation");
      expect(notice().hidden).toBe(true);
      expect(badge().hidden).toBe(true);
      expect(tabBadge().hidden).toBe(true);
      expect(rows().some((row) => row.dataset.unread === "true")).toBe(false);
    });

    it("dismisses the preview but keeps the message unread", () => {
      boot(win, document);
      win.Dimmi?.("simulate", { message: "The sync finished." });

      (notice().querySelector(".notice-close") as HTMLButtonElement).click();

      expect(notice().hidden).toBe(true);
      expect(badge().textContent).toBe("1");
      win.Dimmi?.("close");
      expect(launcher().getAttribute("aria-label")).toBe("Open messages, 1 unread");
    });

    it("counts nothing for a message the visitor is already reading", () => {
      const widget = boot(win, document);
      win.Dimmi?.("open");
      (shadow().querySelector(".chat") as HTMLElement).click();

      win.Dimmi?.("simulate", { message: "Here you go." });

      expect(widget?.messenger?.tab).toBe("conversation");
      expect(badge().hidden).toBe(true);
      expect(notice().hidden).toBe(true);
    });
  });

  describe("loading", () => {
    const view = (name: string) => shadow().querySelector(`.view[data-view="${name}"]`) as HTMLElement;

    beforeEach(() => {
      vi.useFakeTimers();
      win.__dimmi = { appId: "wgt_1", apiBase: "https://app.example.com", latency: 800, preview: true };
    });

    afterEach(() => {
      vi.useRealTimers();
    });

    it("loads the first view when the panel opens, busy until the content arrives", () => {
      boot(win, document);
      expect(view("home").dataset.state).toBeUndefined();

      win.Dimmi?.("open");
      expect(view("home").dataset.state).toBe("loading");
      expect(view("home").getAttribute("aria-busy")).toBe("true");

      vi.advanceTimersByTime(800);
      expect(view("home").dataset.state).toBe("ready");
      expect(view("home").hasAttribute("aria-busy")).toBe(false);
    });

    it("loads every other view the first time it is shown, and only then", () => {
      boot(win, document);
      win.Dimmi?.("open");
      vi.advanceTimersByTime(800);

      tab("help").click();
      expect(view("help").dataset.state).toBe("loading");
      (shadow().querySelector('[data-open="article:connect-github"]') as HTMLElement).click();
      expect(view("article:connect-github").dataset.state).toBe("loading");
      vi.advanceTimersByTime(800);
      expect(view("help").dataset.state).toBe("ready");
      expect(view("article:connect-github").dataset.state).toBe("ready");

      (shadow().querySelector('[data-current] [data-action="click->back"]') as HTMLElement).click();
      expect(view("help").dataset.state).toBe("ready");
    });

    it("keeps loaded views loaded when the options change", () => {
      boot(win, document);
      win.Dimmi?.("open");
      vi.advanceTimersByTime(800);

      win.Dimmi?.("update", { appearance: { primary_color: "#5b4bff" } });

      expect(view("home").dataset.state).toBe("ready");
    });

    it("loads again with a new latency to preview loading states", () => {
      boot(win, document);
      win.Dimmi?.("open");
      vi.advanceTimersByTime(800);

      win.Dimmi?.("simulate", { latency: 2000 });
      expect(view("home").dataset.state).toBe("loading");
      vi.advanceTimersByTime(1999);
      expect(view("home").dataset.state).toBe("loading");
      vi.advanceTimersByTime(1);
      expect(view("home").dataset.state).toBe("ready");

      win.Dimmi?.("simulate", { latency: 0 });
      expect(view("home").dataset.state).toBe("ready");
    });

    it("posts new messages into the loaded thread", () => {
      boot(win, document);
      win.Dimmi?.("open");
      (shadow().querySelector(".chat") as HTMLElement).click();
      vi.advanceTimersByTime(800);
      const textarea = shadow().querySelector(".composer textarea") as HTMLTextAreaElement;
      textarea.value = "Hello";
      textarea.dispatchEvent(new Event("input", { bubbles: true }));

      (shadow().querySelector(".composer") as HTMLFormElement).requestSubmit();

      const sent = [...shadow().querySelectorAll('.thread > .loaded > [data-from="visitor"]')].at(-1);
      expect(sent?.textContent).toContain("Hello");
      expect(shadow().querySelector(".thread > .loaded > [data-typing]")).not.toBeNull();
    });
  });

  describe("composer", () => {
    const composer = () => shadow().querySelector(".composer") as HTMLFormElement;
    const textarea = () => composer().querySelector("textarea") as HTMLTextAreaElement;
    const sendButton = () => composer().querySelector('[data-target="send"]') as HTMLButtonElement;
    const type = (text: string) => {
      textarea().value = text;
      textarea().dispatchEvent(new Event("input", { bubbles: true }));
    };

    beforeEach(() => {
      vi.useFakeTimers();
    });

    afterEach(() => {
      vi.useRealTimers();
    });

    it("sends on Enter, shows the other side typing, then their reply", () => {
      boot(win, document);
      expect(sendButton().disabled).toBe(true);

      type("Is there an API?");
      expect(sendButton().disabled).toBe(false);
      textarea().dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true }));

      expect([...shadow().querySelectorAll('[data-from="visitor"] .bubble')].at(-1)?.textContent).toBe(
        "Is there an API?",
      );
      expect(shadow().querySelector("[data-typing]")).not.toBeNull();
      expect(textarea().value).toBe("");
      vi.advanceTimersByTime(1600);
      expect(shadow().querySelector("[data-typing]")).toBeNull();
      expect([...shadow().querySelectorAll('[data-from="team"] .bubble')].at(-1)?.textContent).toContain(
        "A teammate will pick this up",
      );
    });

    it("keeps Shift+Enter for a new line", () => {
      boot(win, document);
      type("first line");

      const event = new KeyboardEvent("keydown", { key: "Enter", shiftKey: true, bubbles: true, cancelable: true });
      textarea().dispatchEvent(event);

      expect(event.defaultPrevented).toBe(false);
      expect(textarea().value).toBe("first line");
    });

    it("holds attached files as chips and sends them as file bubbles", () => {
      boot(win, document);
      const picker = composer().querySelector('input[type="file"]') as HTMLInputElement;
      const files = [
        new File(["log"], "error.log", { type: "text/plain" }),
        new File(["x"], "shot.png", { type: "image/png" }),
      ];
      Object.defineProperty(picker, "files", { value: files, configurable: true });

      picker.dispatchEvent(new Event("change", { bubbles: true }));
      expect([...composer().querySelectorAll(".chip > span")].map((chip) => chip.textContent)).toEqual([
        "error.log",
        "shot.png",
      ]);
      expect(sendButton().disabled).toBe(false);

      (composer().querySelector('.chip [data-action="click->unattach"]') as HTMLButtonElement).click();
      expect(composer().querySelectorAll(".chip")).toHaveLength(1);

      composer().requestSubmit();
      expect(composer().querySelectorAll(".chip")).toHaveLength(0);
      expect([...shadow().querySelectorAll('[data-from="visitor"] figcaption')].at(-1)?.textContent).toBe("shot.png");
    });

    it("records a voice note with a running timer, and discards or sends it", () => {
      boot(win, document);
      const press = (action: string) =>
        (composer().querySelector(`[data-action="click->${action}"]`) as HTMLButtonElement).click();

      press("record");
      expect(composer().dataset.recording).toBe("true");
      vi.advanceTimersByTime(3000);
      expect(composer().querySelector('[data-target="timer"]')?.textContent).toBe("0:03");
      press("discard");
      expect(composer().dataset.recording).toBeUndefined();

      const notes = () => shadow().querySelectorAll('[data-from="visitor"] .voice').length;
      const before = notes();
      press("record");
      vi.advanceTimersByTime(5000);
      press("finish");
      expect(notes()).toBe(before + 1);
      expect([...shadow().querySelectorAll('[data-from="visitor"] .voice time')].at(-1)?.textContent).toBe("0:05");
    });

    it("plays a voice note across its length and rewinds", () => {
      boot(win, document);
      const note = shadow().querySelector(".voice") as HTMLElement;
      const played = () => note.querySelectorAll("[data-played]").length;

      (note.querySelector(".play") as HTMLButtonElement).click();
      expect(note.dataset.playing).toBe("true");
      vi.advanceTimersByTime(7000);
      expect(played()).toBeGreaterThan(0);
      expect(played()).toBeLessThan(24);
      vi.advanceTimersByTime(7100);
      expect(note.dataset.playing).toBe("false");
      expect(played()).toBe(0);
    });
  });

  it("walks from a help collection to an article, growing the panel, and back again", () => {
    const widget = boot(win, document);
    const messenger = () => shadow().querySelector(".messenger") as HTMLElement;
    const press = (selector: string) => (shadow().querySelector(selector) as HTMLElement).click();
    tab("help").click();

    press('[data-current] [data-open="collection:integrations"]');
    expect(widget?.messenger?.tab).toBe("collection:integrations");
    expect(messenger().dataset.page).toBe("true");
    expect(widget?.messenger?.expanded).toBe(false);

    press('[data-current] [data-open="article:connect-github"]');
    expect(widget?.messenger?.tab).toBe("article:connect-github");
    expect(widget?.messenger?.expanded).toBe(true);
    expect(shadow().querySelector("[data-current] h1")?.textContent).toBe("Connect a GitHub repository");
    expect(shadow().querySelectorAll("[data-current] .prose ol li")).toHaveLength(3);

    press('[data-current] [data-action="click->back"]');
    expect(widget?.messenger?.tab).toBe("collection:integrations");
    expect(widget?.messenger?.expanded).toBe(false);
    press('[data-current] [data-action="click->back"]');
    expect(widget?.messenger?.tab).toBe("help");
    expect(messenger().dataset.page).toBe("false");
  });

  it("opens a changelog entry full size and keeps a hand-picked size when leaving", () => {
    const widget = boot(win, document);
    tab("changelog").click();

    (shadow().querySelector('[data-current] [data-open="entry:0"]') as HTMLElement).click();
    expect(widget?.messenger?.tab).toBe("entry:0");
    expect(widget?.messenger?.expanded).toBe(true);
    expect(shadow().querySelector("[data-current] h1")?.textContent).toBe("Shared inboxes are here");

    (shadow().querySelector('[data-current] [data-action="click->back"]') as HTMLElement).click();
    expect(widget?.messenger?.expanded).toBe(false);

    (shadow().querySelector('[data-current] [data-action="click->expand"]') as HTMLElement).click();
    (shadow().querySelector('[data-current] [data-open="entry:1"]') as HTMLElement).click();
    (shadow().querySelector('[data-current] [data-action="click->back"]') as HTMLElement).click();
    expect(widget?.messenger?.expanded).toBe(true);
  });

  it("opens popular articles straight from the help tab", () => {
    const widget = boot(win, document);
    tab("help").click();

    (shadow().querySelector('[data-current] .section [data-open="article:install-the-widget"]') as HTMLElement).click();

    expect(widget?.messenger?.tab).toBe("article:install-the-widget");
    expect(shadow().querySelector("[data-current] pre code")?.textContent).toContain("loader.js");
  });

  it("keeps the home view's top row pinned, filling it once the view scrolls", () => {
    boot(win, document);
    const home = shadow().querySelector('.view[data-view="home"]') as HTMLElement;

    const top = home.querySelector(".top") as HTMLElement;
    expect(top.parentElement).toBe(home);
    expect(top.querySelector('[data-action="click->close"]')).not.toBeNull();
    home.scrollTop = 120;
    home.dispatchEvent(new Event("scroll"));
    expect(home.hasAttribute("data-scrolled")).toBe(true);
    home.scrollTop = 0;
    home.dispatchEvent(new Event("scroll"));
    expect(home.hasAttribute("data-scrolled")).toBe(false);
  });

  it("narrows the help collections as the visitor types", () => {
    boot(win, document);
    const input = shadow().querySelector("[data-view=help] input") as HTMLInputElement;
    const shown = () =>
      [...shadow().querySelectorAll<HTMLElement>("[data-target=row]")].filter((row) => !row.hidden).length;

    input.value = "BILL";
    input.dispatchEvent(new Event("input", { bubbles: true }));
    expect(shown()).toBe(1);

    input.value = "nothing like this";
    input.dispatchEvent(new Event("input", { bubbles: true }));
    expect(shown()).toBe(0);
    expect(shadow().querySelector("[data-target=none]")?.hasAttribute("hidden")).toBe(false);
  });

  it("draws only the tabs the customer kept, in their order, and drops the bar for one", () => {
    const widget = boot(win, document);
    tab("changelog").click();

    win.Dimmi?.("update", { tabs: ["help", "home"] });
    expect([...shadow().querySelectorAll<HTMLElement>("[role=tab]")].map((button) => button.dataset.tab)).toEqual([
      "help",
      "home",
    ]);
    expect(shadow().querySelector("[data-view=changelog]")).toBeNull();
    expect(widget?.messenger?.tab).toBe("help");

    win.Dimmi?.("update", { tabs: ["home"] });
    expect(shadow().querySelector(".tabs")).toBeNull();
  });

  it("follows the brand color and keeps the panel open on the same tab when options change", () => {
    const widget = boot(win, document);
    win.Dimmi?.("open");
    tab("changelog").click();

    win.Dimmi?.("update", {
      appearance: { primary_color: "#fde047", position: "left", theme: "dark", launcher: { shape: "squircle" } },
      texts: { greeting: "Hello <b>friend</b>" },
      show_branding: false,
    });

    const host = document.getElementById("dimmi-messenger") as HTMLElement;
    expect(host.dataset.theme).toBe("dark");
    expect(host.dataset.brand).toBe("light");
    expect(host.style.getPropertyValue("--brand")).toBe("#fde047");
    expect(launcher().dataset.shape).toBe("squircle");
    expect(shadow().querySelector(".messenger")?.getAttribute("data-side")).toBe("left");
    expect(shadow().querySelector("h1")?.textContent).toBe("Hey there.Hello <b>friend</b>");
    expect(shadow().querySelector(".credit")).toBeNull();
    expect(widget?.messenger?.isOpen).toBe(true);
    expect(widget?.messenger?.tab).toBe("changelog");
    expect(widget?.options.appearance.launcher.style).toBe("bubble");
  });

  it("hides the launcher when the customer opens the messenger from their own buttons", () => {
    boot(win, document);

    win.Dimmi?.("update", { appearance: { launcher: { style: "hidden" } } });

    expect(launcher().hidden).toBe(true);
  });

  it("shows a label beside the mark when the launcher has one", () => {
    boot(win, document);

    win.Dimmi?.("update", { appearance: { launcher: { label: "Help" } } });

    expect(launcher().querySelector(".label")?.textContent).toBe("Help");
  });

  it("pops on open when sound is on, and never on a silent re-render", () => {
    const notes = vi.fn();
    const param = () => ({ value: 0, setValueAtTime: vi.fn(), exponentialRampToValueAtTime: vi.fn() });
    vi.stubGlobal(
      "AudioContext",
      class {
        currentTime = 0;
        destination = {};
        createBiquadFilter() {
          return { type: "", frequency: param(), connect: (to: unknown) => to };
        }
        createGain() {
          return { gain: param(), connect: (to: unknown) => to };
        }
        createOscillator() {
          notes();
          return { frequency: param(), connect: (to: unknown) => to, start: vi.fn(), stop: vi.fn() };
        }
      },
    );
    boot(win, document);

    win.Dimmi?.("open");
    expect(notes).toHaveBeenCalledTimes(2);
    win.Dimmi?.("update", { texts: { greeting: "Hey" } });
    expect(notes).toHaveBeenCalledTimes(2);
    win.Dimmi?.("close");
    win.Dimmi?.("update", { appearance: { sound: false } });
    win.Dimmi?.("open");
    expect(notes).toHaveBeenCalledTimes(2);
  });

  it("warns about commands it does not know", () => {
    const warn = vi.spyOn(console, "warn").mockReturnValue(undefined);
    boot(win, document);

    win.Dimmi?.("track", "signup");

    expect(warn).toHaveBeenCalledWith(expect.stringContaining('[dimmi] unknown_command: Dimmi("track")'));
  });

  it("refuses to start without the loader", () => {
    const error = vi.spyOn(console, "error").mockReturnValue(undefined);
    delete win.__dimmi;

    expect(boot(win, document)).toBeUndefined();
    expect(error).toHaveBeenCalledWith(expect.stringContaining("[dimmi] loader_missing"));
  });
});
