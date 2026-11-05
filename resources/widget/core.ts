/*
 * The messenger itself. It mounts in a shadow root, so the host page's CSS
 * never reaches it and its own never leaks out, and styles it through a
 * constructed stylesheet, which a strict content security policy allows.
 * Then it takes over the global Dimmi function: commands queued before it
 * arrived run in order, later ones run at once, and an unknown command
 * says so in the console instead of failing quietly. Elements on the page
 * marked data-dimmi="open", "close" or "toggle" drive it without any code.
 *
 * On a customer's page the messenger connects to its server: the widget's
 * config is merged over the defaults before the launcher shows, and the
 * views draw the visitor's real conversations. A widget the server does
 * not serve here, such as a wrong app id or a site not allowed, shows
 * nothing and says why in the console.
 *
 * A preview, marked data-preview on the snippet, has no server: it shows
 * sample content after a simulated latency, 700 ms unless data-latency
 * says otherwise, so the design can be judged. Dimmi("simulate", ...)
 * changes the latency or delivers a message from the team there.
 * Dimmi("update", { ... }) merges changes over the options and re-renders
 * either way, keeping the panel open on the same tab.
 */
import { type Content, sample } from "./content";
import { ComposerController } from "./controllers/composer_controller";
import { MessengerController } from "./controllers/messenger_controller";
import { ScrollController } from "./controllers/scroll_controller";
import { SearchController } from "./controllers/search_controller";
import { VoiceController } from "./controllers/voice_controller";
import { Application } from "./lib/controller";
import { Live } from "./lib/live";
import { report } from "./lib/report";
import { defaults, merge, type Options, type OptionsUpdate } from "./options";
import { render, style } from "./render";
import type { Command, DimmiWindow, Queue } from "./settings";
import styles from "./styles/index.css?inline";

export interface Widget {
  readonly messenger: MessengerController | undefined;
  readonly options: Options;
  readonly live: Live | undefined;
  update(changes: OptionsUpdate): void;
}

const toggles = ["open", "close", "toggle"] as const;

interface Simulation {
  latency?: number;
  message?: string;
}

/*
 * blank is what a connected messenger draws before the server answers.
 */
const blank: Content = {
  ...sample,
  company: "",
  reply_time: "",
  team: [],
  thread: [],
  incoming: undefined,
  conversations: [],
  links: [],
  collections: [],
  articles: [],
  popular: [],
  changelog: [],
  unread_changelog: 0,
};

export function boot(win: Window & DimmiWindow, doc: Document): Widget | undefined {
  const settings = win.__dimmi;
  if (!settings) {
    report("error", "loader_missing", "load the widget with /widget/v1/loader.js, not the core file.");
    return undefined;
  }

  const host = doc.createElement("div");
  host.id = "dimmi-messenger";
  const root = host.attachShadow({ mode: "open" });
  const sheet = new CSSStyleSheet();
  sheet.replaceSync(styles);
  root.adoptedStyleSheets = [sheet];

  const app = new Application(root)
    .register("messenger", MessengerController)
    .register("search", SearchController)
    .register("composer", ComposerController)
    .register("voice", VoiceController)
    .register("scroll", ScrollController);
  const find = () => app.controller<MessengerController>(root.firstElementChild as HTMLElement, "messenger");
  let options = defaults;
  let latency = settings.preview ? (settings.latency ?? 700) : 0;
  let live: Live | undefined;

  const draw = () => {
    const before = find();
    const [wasOpen, tab] = [before?.isOpen, before?.tab ?? ""];
    const loaded = [...root.querySelectorAll<HTMLElement>('.view[data-state="ready"]')].map(
      (view) => view.dataset.view,
    );
    app.stop();
    style(host, options);
    root.innerHTML = render(options, live ? live.content() : sample);
    const messenger = root.firstElementChild as HTMLElement;
    messenger.dataset.latency = String(latency);
    messenger.dataset.live = String(Boolean(live));
    for (const view of root.querySelectorAll<HTMLElement>(".view")) {
      if (loaded.includes(view.dataset.view)) {
        view.dataset.state = "ready";
      }
    }
    app.connect(root);
    if (wasOpen) {
      find()?.restore(tab);
    } else {
      find()?.show(tab);
    }
  };

  const widget: Widget = {
    get messenger() {
      return find();
    },
    get options() {
      return options;
    },
    get live() {
      return live;
    },
    update(changes) {
      options = merge(options, changes);
      draw();
    },
  };

  /*
   * simulate stands in for the server in a preview: a new latency loads
   * every view again, and a message arrives as if the team sent it.
   */
  const simulate = ({ latency: next, message }: Simulation = {}) => {
    if (live) {
      report("warn", "simulate_live", 'Dimmi("simulate") works only in a preview.');
      return;
    }
    if (next !== undefined) {
      latency = Math.max(0, Number(next) || 0);
      for (const view of root.querySelectorAll<HTMLElement>(".view")) {
        delete view.dataset.state;
      }
      draw();
    }
    if (message) {
      const conversation = sample.conversations[0]?.number ?? 0;
      find()?.receive({ conversation, from: "team", author: "Maya Kerr", body: message, at: "now" });
    }
  };

  const run = ([name, argument]: Command) => {
    const toggle = toggles.find((known) => known === name);
    if (toggle) {
      find()?.[toggle]();
    } else if (name === "update" || name === "boot") {
      widget.update(argument as OptionsUpdate);
    } else if (name === "simulate") {
      simulate(argument as Simulation);
    } else if (name === "shutdown" || name === "logout") {
      void live?.session.signOut();
    } else {
      report("warn", "unknown_command", `Dimmi("${String(name)}") is not supported yet.`);
    }
  };

  const listen = () => {
    root.addEventListener("dimmi:connect", () => app.connect(root));
    root.addEventListener("dimmi:open", (event) => {
      void live?.openConversation((event as CustomEvent<{ conversation?: string }>).detail?.conversation);
    });
    root.addEventListener("dimmi:send", (event) => {
      const { body, files } = (event as CustomEvent<{ body: string; files: File[] }>).detail;
      void live?.send(body, files);
    });
    root.addEventListener("dimmi:typing", () => live?.typing(true));
    root.addEventListener("submit", (event) => {
      const form =
        event.target instanceof HTMLFormElement && event.target.matches("[data-capture]") ? event.target : null;
      if (form) {
        event.preventDefault();
        void live?.capture(String(new FormData(form).get("email") ?? ""));
      }
    });
    doc.addEventListener("click", (event) => {
      const trigger = event.target instanceof Element ? event.target.closest<HTMLElement>("[data-dimmi]") : null;
      if (trigger?.dataset.dimmi) {
        run([trigger.dataset.dimmi]);
      }
    });
  };

  const mount = () => {
    doc.body.append(host);
    draw();
    app.start();
    listen();
    const queued = win.Dimmi?.q ?? [];
    const dimmi: Queue = (...command) => run(command);
    win.Dimmi = dimmi;
    queued.forEach(run);
  };

  if (settings.preview) {
    mount();
    return widget;
  }

  live = new Live(
    {
      root,
      options: () => options,
      update: (changes) => widget.update(changes),
      notify: ({ conversation, body, author }) => {
        find()?.alert({ conversation, from: "team", author, body, at: "" });
      },
      showing: (conversation) => {
        const messenger = find();
        return Boolean(messenger?.isOpen && messenger.tab === "conversation" && live?.open === conversation);
      },
    },
    browserOf(win),
    settings.appId,
    `${settings.apiBase}/widget/v1`,
    blank,
  );

  void live.start().then((shown) => {
    if (shown) {
      mount();
    }
  });

  return widget;
}

/*
 * browserOf gathers what the connection needs from the page.
 */
function browserOf(win: Window) {
  const storage = {
    get: (key: string) => {
      try {
        return win.localStorage.getItem(key);
      } catch {
        return null;
      }
    },
    set: (key: string, value: string) => {
      try {
        win.localStorage.setItem(key, value);
      } catch {
        return;
      }
    },
    remove: (key: string) => {
      try {
        win.localStorage.removeItem(key);
      } catch {
        return;
      }
    },
  };

  return {
    fetch: win.fetch.bind(win),
    storage,
    eventSource: (url: string) => stream(url),
    now: () => Date.now(),
    uuid: () => win.crypto.randomUUID(),
    setTimeout: (fn: () => void, ms: number) => win.setTimeout(fn, ms),
    clearTimeout: (handle: unknown) => win.clearTimeout(handle as number),
    page: () => ({ url: win.location.href, title: win.document.title, referrer: win.document.referrer }),
  };
}

/*
 * stream opens an event source and hands on each event's data.
 */
function stream(url: string) {
  const source = new EventSource(url);
  const out: {
    onmessage: ((event: { data: string }) => void) | null;
    onerror: ((event: unknown) => void) | null;
    close(): void;
  } = { onmessage: null, onerror: null, close: () => source.close() };
  source.onmessage = (event) => out.onmessage?.({ data: String(event.data) });
  source.onerror = (event) => out.onerror?.(event);

  return out;
}
