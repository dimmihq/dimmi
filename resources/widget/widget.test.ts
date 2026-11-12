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

/*
 * started boots a preview and waits for its sample content.
 */
async function started() {
  const widget = boot(win, document);
  await widget?.ready;
  return widget;
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

  it("mounts in a shadow root and runs the calls queued before it", async () => {
    win.Dimmi = Object.assign(() => undefined, { q: [["open"]] as [string][] });

    const widget = await started();

    expect(widget?.messenger?.isOpen).toBe(true);
    expect(panel().hasAttribute("inert")).toBe(false);
    expect(launcher().getAttribute("aria-expanded")).toBe("true");
    expect(launcher().getAttribute("aria-label")).toBe("Close messages");
  });

  it("answers the global function, the launcher, its close buttons and page elements marked data-dimmi", async () => {
    const widget = await started();
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

  it("closes on Escape and hands focus back to the launcher", async () => {
    const widget = await started();
    win.Dimmi?.("open");

    panel().dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));

    expect(widget?.messenger?.isOpen).toBe(false);
    expect(shadow().activeElement).toBe(launcher());
  });

  it("switches views from the tab bar and from the cards on the home view", async () => {
    const widget = await started();

    tab("help").click();
    expect(widget?.messenger?.tab).toBe("help");
    expect(tab("help").getAttribute("aria-selected")).toBe("true");
    expect(tab("home").getAttribute("aria-selected")).toBe("false");

    tab("home").click();
    (shadow().querySelector(".chat strong") as HTMLElement).click();
    expect(widget?.messenger?.tab).toBe("conversation");
  });

  it("grows the panel toward the full window and back", async () => {
    await started();
    const expand = shadow().querySelector('[data-action="click->expand"]') as HTMLButtonElement;

    expand.click();
    expect(shadow().querySelector(".messenger")?.getAttribute("data-expanded")).toBe("true");
    expect(expand.getAttribute("aria-label")).toBe("Shrink messages");
    expand.click();
    expect(shadow().querySelector(".messenger")?.getAttribute("data-expanded")).toBe("false");
  });

  it("opens a conversation over the tabs and goes back to the tab it came from", async () => {
    const widget = await started();
    tab("messages").click();

    (shadow().querySelector('[data-view="messages"] .loaded .row') as HTMLButtonElement).click();
    expect(widget?.messenger?.tab).toBe("conversation");
    expect(shadow().querySelectorAll('[data-view="conversation"] .said').length).toBeGreaterThan(3);

    (shadow().querySelector('[data-action="click->back"]') as HTMLButtonElement).click();
    expect(widget?.messenger?.tab).toBe("messages");
  });

  describe("picture viewer", () => {
    const messenger = () => shadow().querySelector(".messenger") as HTMLElement;
    const viewer = () => shadow().querySelector(".viewer") as HTMLElement;

    /*
     * picture puts a picture with a link into the thread, the way a sent
     * one shows, and returns its link.
     */
    function picture(): HTMLAnchorElement {
      const thread = shadow().querySelector(".thread") as HTMLElement;
      thread.insertAdjacentHTML(
        "beforeend",
        '<a class="shot picture" href="https://files.example.com/shot.png" data-action="click->view"><img alt="shot.png"></a>',
      );
      return thread.lastElementChild as HTMLAnchorElement;
    }

    it("opens a clicked picture across the expanded messenger", async () => {
      await started();
      launcher().click();
      const link = picture();

      const click = new MouseEvent("click", { bubbles: true, cancelable: true });
      (link.querySelector("img") as HTMLImageElement).dispatchEvent(click);

      expect(click.defaultPrevented).toBe(true);
      expect(viewer().hidden).toBe(false);
      expect(messenger().dataset.expanded).toBe("true");
      expect((viewer().querySelector("img") as HTMLImageElement).src).toBe("https://files.example.com/shot.png");
    });

    it("closes on Escape from its own buttons and shrinks the messenger back", async () => {
      await started();
      launcher().click();
      (picture().querySelector("img") as HTMLImageElement).click();

      const close = viewer().querySelector("button[aria-label='Close picture']") as HTMLButtonElement;
      close.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));

      expect(viewer().hidden).toBe(true);
      expect(messenger().dataset.expanded).toBe("false");
      expect(messenger().dataset.open).toBe("true");
    });

    it("closes on a click around the picture but not on the picture", async () => {
      await started();
      launcher().click();
      (picture().querySelector("img") as HTMLImageElement).click();

      (viewer().querySelector(".viewer-stage img") as HTMLImageElement).click();
      expect(viewer().hidden).toBe(false);

      (viewer().querySelector(".viewer-stage") as HTMLElement).click();
      expect(viewer().hidden).toBe(true);
    });

    it("keeps a messenger the visitor expanded expanded", async () => {
      await started();
      launcher().click();
      (shadow().querySelector("[data-action='click->expand']") as HTMLButtonElement).click();
      (picture().querySelector("img") as HTMLImageElement).click();

      (viewer().querySelector("button[aria-label='Close picture']") as HTMLButtonElement).click();

      expect(messenger().dataset.expanded).toBe("true");
    });
  });

  describe("unread messages", () => {
    const notice = () => shadow().querySelector(".notice") as HTMLElement;
    const badge = () => shadow().querySelector('.launcher [data-target="count"]') as HTMLElement;
    const tabBadge = () => shadow().querySelector('[data-target="tab-count"]') as HTMLElement;
    const rows = () => [...shadow().querySelectorAll<HTMLElement>('[data-conversation="44308"]')];

    it("previews a new message above the closed launcher and counts it as unread", async () => {
      await started();

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

    it("opens the conversation from the preview and reads everything", async () => {
      const widget = await started();
      win.Dimmi?.("simulate", { message: "The sync finished." });

      (notice().querySelector(".notice-card") as HTMLButtonElement).click();

      expect(widget?.messenger?.isOpen).toBe(true);
      expect(widget?.messenger?.tab).toBe("conversation");
      expect(notice().hidden).toBe(true);
      expect(badge().hidden).toBe(true);
      expect(tabBadge().hidden).toBe(true);
      expect(rows().some((row) => row.dataset.unread === "true")).toBe(false);
    });

    it("dismisses the preview but keeps the message unread", async () => {
      await started();
      win.Dimmi?.("simulate", { message: "The sync finished." });

      (notice().querySelector(".notice-close") as HTMLButtonElement).click();

      expect(notice().hidden).toBe(true);
      expect(badge().textContent).toBe("1");
      win.Dimmi?.("close");
      expect(launcher().getAttribute("aria-label")).toBe("Open messages, 1 unread");
    });

    it("counts nothing for a message the visitor is already reading", async () => {
      const widget = await started();
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

    it("loads the first view when the panel opens, busy until the content arrives", async () => {
      await started();
      expect(view("home").dataset.state).toBeUndefined();

      win.Dimmi?.("open");
      expect(view("home").dataset.state).toBe("loading");
      expect(view("home").getAttribute("aria-busy")).toBe("true");

      vi.advanceTimersByTime(800);
      expect(view("home").dataset.state).toBe("ready");
      expect(view("home").hasAttribute("aria-busy")).toBe(false);
    });

    it("loads every other view the first time it is shown, and only then", async () => {
      await started();
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

    it("keeps loaded views loaded when the options change", async () => {
      await started();
      win.Dimmi?.("open");
      vi.advanceTimersByTime(800);

      win.Dimmi?.("update", { appearance: { primary_color: "#5b4bff" } });

      expect(view("home").dataset.state).toBe("ready");
    });

    it("loads again with a new latency to preview loading states", async () => {
      await started();
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

    it("posts new messages into the loaded thread", async () => {
      await started();
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

    it("asks a question in a new conversation, opening the messenger", async () => {
      const widget = await started();

      win.Dimmi?.("ask", "  How do I reset my password?  ");
      vi.advanceTimersByTime(800);

      expect(widget?.messenger?.isOpen).toBe(true);
      expect(widget?.messenger?.tab).toBe("conversation");
      const sent = [...shadow().querySelectorAll('.thread > .loaded > [data-from="visitor"]')].at(-1);
      expect(sent?.textContent).toContain("How do I reset my password?");
    });
  });

  describe("voice notes", () => {
    const composer = () => shadow().querySelector(".composer") as HTMLFormElement;
    const record = () => composer().querySelector("[data-action='click->record']") as HTMLButtonElement;
    const finish = () => composer().querySelector("[data-action='click->finish']") as HTMLButtonElement;
    const note = () => composer().querySelector("[data-target='note']") as HTMLElement;
    const tracks: { stop: ReturnType<typeof vi.fn> }[] = [];

    /*
     * FakeRecorder stands in for the browser's MediaRecorder: it hands back
     * one chunk of audio when stopped.
     */
    class FakeRecorder extends EventTarget {
      static isTypeSupported = (type: string) => type.startsWith("audio/webm");
      mimeType = "audio/webm;codecs=opus";
      state = "inactive";
      start() {
        this.state = "recording";
      }
      stop() {
        this.state = "inactive";
        this.dispatchEvent(
          Object.assign(new Event("dataavailable"), { data: new Blob(["voice"], { type: "audio/webm" }) }),
        );
        this.dispatchEvent(new Event("stop"));
      }
    }

    /*
     * liveComposer boots the messenger, marks it connected, opens a new
     * conversation and returns what it sends.
     */
    async function liveComposer(): Promise<{ body: string; files: File[]; voice?: string }[]> {
      await started();
      (shadow().querySelector(".messenger") as HTMLElement).dataset.live = "true";
      const sent: { body: string; files: File[]; voice?: string }[] = [];
      shadow().addEventListener("dimmi:send", (event) => sent.push((event as CustomEvent).detail));
      return sent;
    }

    beforeEach(() => {
      tracks.length = 0;
      vi.stubGlobal("MediaRecorder", FakeRecorder);
      vi.stubGlobal(
        "AudioContext",
        class {
          createAnalyser() {
            return { fftSize: 512, getByteTimeDomainData: (samples: Uint8Array) => samples.fill(128) };
          }
          createMediaStreamSource() {
            return { connect: () => undefined };
          }
          close() {
            return Promise.resolve();
          }
        },
      );
    });

    afterEach(() => {
      vi.unstubAllGlobals();
      Object.defineProperty(navigator, "mediaDevices", { value: undefined, configurable: true });
    });

    it("records from the microphone and sends the note as an audio file, letting the microphone go", async () => {
      const track = { stop: vi.fn() };
      tracks.push(track);
      Object.defineProperty(navigator, "mediaDevices", {
        value: { getUserMedia: vi.fn().mockResolvedValue({ getTracks: () => [track] }) },
        configurable: true,
      });
      const sent = await liveComposer();

      record().click();
      await vi.waitFor(() => expect(composer().dataset.recording).toBe("true"));
      finish().click();

      await vi.waitFor(() => expect(sent).toHaveLength(1));
      const [file] = sent[0]?.files ?? [];
      expect(file?.type).toBe("audio/webm");
      expect(file?.name).toBe("Voice note.webm");
      expect(track.stop).toHaveBeenCalled();
      expect(composer().dataset.recording).toBeUndefined();
    });

    it("says so when the microphone is refused, and records nothing", async () => {
      Object.defineProperty(navigator, "mediaDevices", {
        value: { getUserMedia: vi.fn().mockRejectedValue(new Error("NotAllowedError")) },
        configurable: true,
      });
      const sent = await liveComposer();

      record().click();

      await vi.waitFor(() => expect(note().hidden).toBe(false));
      expect(note().textContent).toContain("Allow the microphone");
      expect(composer().dataset.recording).toBeUndefined();
      expect(sent).toHaveLength(0);
    });

    it("throws a note away without sending it", async () => {
      const track = { stop: vi.fn() };
      Object.defineProperty(navigator, "mediaDevices", {
        value: { getUserMedia: vi.fn().mockResolvedValue({ getTracks: () => [track] }) },
        configurable: true,
      });
      const sent = await liveComposer();

      record().click();
      await vi.waitFor(() => expect(composer().dataset.recording).toBe("true"));
      (composer().querySelector("[data-action='click->discard']") as HTMLButtonElement).click();

      expect(composer().dataset.recording).toBeUndefined();
      expect(track.stop).toHaveBeenCalled();
      expect(sent).toHaveLength(0);
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

    it("sends on Enter, shows the other side typing, then their reply", async () => {
      await started();
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

    it("keeps Shift+Enter for a new line", async () => {
      await started();
      type("first line");

      const event = new KeyboardEvent("keydown", { key: "Enter", shiftKey: true, bubbles: true, cancelable: true });
      textarea().dispatchEvent(event);

      expect(event.defaultPrevented).toBe(false);
      expect(textarea().value).toBe("first line");
    });

    it("puts a picked emoji where the cursor was and closes the picker", async () => {
      await started();
      const picker = () => composer().querySelector(".emoji-picker") as HTMLElement;
      const toggle = composer().querySelector('[data-action="click->emoji"]') as HTMLButtonElement;
      type("Thanks !");
      textarea().setSelectionRange(7, 7);

      toggle.click();
      await vi.dynamicImportSettled();
      expect(picker().hidden).toBe(false);
      expect(toggle.getAttribute("aria-expanded")).toBe("true");

      [...picker().querySelectorAll<HTMLButtonElement>("[data-emoji]")].find((b) => b.dataset.emoji === "🙏")?.click();
      expect(textarea().value).toBe("Thanks 🙏!");
      expect(textarea().selectionStart).toBe(7 + "🙏".length);
      expect(picker().hidden).toBe(true);
      expect(sendButton().disabled).toBe(false);

      toggle.click();
      await vi.dynamicImportSettled();
      picker().dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
      expect(picker().hidden).toBe(true);
    });

    it("holds attached files as chips and sends them as file bubbles", async () => {
      await started();
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

    it("records a voice note with a running timer, and discards or sends it", async () => {
      await started();
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

    it("plays a voice note across its length and rewinds", async () => {
      await started();
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

  it("walks from a help collection to an article, growing the panel, and back again", async () => {
    const widget = await started();
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

  it("opens a changelog entry full size and keeps a hand-picked size when leaving", async () => {
    const widget = await started();
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

  it("marks the face picked under an article with a thank-you, and lets the visitor change it", async () => {
    await started();
    tab("help").click();
    (shadow().querySelector('[data-current] [data-open="collection:integrations"]') as HTMLElement).click();
    (shadow().querySelector('[data-current] [data-open="article:connect-github"]') as HTMLElement).click();
    const box = shadow().querySelector("[data-current] .feedback") as HTMLElement;
    const face = (answer: string) => box.querySelector(`[data-helpful="${answer}"]`) as HTMLButtonElement;
    const rated = vi.fn();
    shadow().addEventListener("dimmi:rate", (event) => rated((event as CustomEvent).detail));

    face("no").click();
    expect(box.dataset.rated).toBe("no");
    expect(face("no").getAttribute("aria-pressed")).toBe("true");
    face("yes").click();
    expect(box.dataset.rated).toBe("yes");
    expect(face("no").getAttribute("aria-pressed")).toBe("false");
    expect(rated).toHaveBeenLastCalledWith({ article: "connect-github", helpful: true });
  });

  it("opens popular articles straight from the help tab", async () => {
    const widget = await started();
    tab("help").click();

    (shadow().querySelector('[data-current] .section [data-open="article:install-the-widget"]') as HTMLElement).click();

    expect(widget?.messenger?.tab).toBe("article:install-the-widget");
    expect(shadow().querySelector("[data-current] pre code")?.textContent).toContain("loader.js");
  });

  it("keeps the home view's top row pinned, filling it once the view scrolls", async () => {
    await started();
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

  it("narrows the help collections as the visitor types", async () => {
    await started();
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

  it("draws only the tabs the customer kept, in their order, and drops the bar for one", async () => {
    const widget = await started();
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

  it("follows the brand color and keeps the panel open on the same tab when options change", async () => {
    const widget = await started();
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

  it("takes a server config in a preview, the way a live messenger reads it", async () => {
    await started();
    win.Dimmi?.("open");

    win.Dimmi?.("configure", {
      company: { name: "Parakeet", logo_url: null },
      appearance: { primary_color: "#5b5bd6", position: "left", launcher: { style: "bubble", shape: "squircle" } },
      texts: { greeting_intro: "Hi from Parakeet.", greeting: "", nope: "x", home_link_1: "Écrivez-nous" },
      home_links: [
        { title: "Book a call", url: "https://cal.example.com", icon: "calendar" },
        { title: "Write to us", url: "mailto:help@example.com", icon: "link" },
      ],
    });

    const host = document.getElementById("dimmi-messenger") as HTMLElement;
    expect(host.style.getPropertyValue("--brand")).toBe("#5b5bd6");
    expect(launcher().dataset.shape).toBe("squircle");
    expect(shadow().querySelector(".messenger")?.getAttribute("data-side")).toBe("left");
    expect(shadow().querySelector("h1")?.textContent).toContain("Hi from Parakeet.");
    expect(shadow().textContent).toContain("Parakeet");
    const rows = [...shadow().querySelectorAll<HTMLAnchorElement>('.view[data-view="home"] a.row')];
    expect(rows.map((row) => [row.textContent, row.getAttribute("href")])).toEqual([
      ["Book a call", "https://cal.example.com"],
      ["Écrivez-nous", "mailto:help@example.com"],
    ]);
  });

  it("hides the launcher when the customer opens the messenger from their own buttons", async () => {
    await started();

    win.Dimmi?.("update", { appearance: { launcher: { style: "hidden" } } });

    expect(launcher().hidden).toBe(true);
  });

  it("shows a label beside the mark when the launcher has one", async () => {
    await started();

    win.Dimmi?.("update", { appearance: { launcher: { label: "Help" } } });

    expect(launcher().querySelector(".label")?.textContent).toBe("Help");
  });

  it("pops on open when sound is on, and never on a silent re-render", async () => {
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
    await started();

    win.Dimmi?.("open");
    expect(notes).toHaveBeenCalledTimes(2);
    win.Dimmi?.("update", { texts: { greeting: "Hey" } });
    expect(notes).toHaveBeenCalledTimes(2);
    win.Dimmi?.("close");
    win.Dimmi?.("update", { appearance: { sound: false } });
    win.Dimmi?.("open");
    expect(notes).toHaveBeenCalledTimes(2);
  });

  it("warns about commands it does not know", async () => {
    const warn = vi.spyOn(console, "warn").mockReturnValue(undefined);
    await started();

    win.Dimmi?.("track", "signup");

    expect(warn).toHaveBeenCalledWith(expect.stringContaining('[dimmi] unknown_command: Dimmi("track")'));
  });

  it("refuses to start without the loader", async () => {
    const error = vi.spyOn(console, "error").mockReturnValue(undefined);
    delete win.__dimmi;

    expect(boot(win, document)).toBeUndefined();
    expect(error).toHaveBeenCalledWith(expect.stringContaining("[dimmi] loader_missing"));
  });
});
