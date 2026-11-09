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
 * A preview, marked data-preview on the snippet, has no server: it loads
 * sample content as a chunk of its own, never fetched on a customer's
 * page, and shows it after a simulated latency, 700 ms unless data-latency
 * says otherwise, so the design can be judged. Dimmi("simulate", ...)
 * changes the latency or delivers a message from the team there.
 * Dimmi("update", { ... }) merges changes over the options and re-renders
 * either way, keeping the panel open on the same tab. Dimmi("ask", text)
 * opens a new conversation and sends the text in it.
 *
 * A signed-in visitor's identity comes from the snippet's
 * data-identity-url, or from Dimmi("identify", fn), where fn answers the
 * token the host app's server minted, or null. Dimmi("identify") with no
 * argument asks again after the visitor signed in without a page load,
 * and Dimmi("logout") forgets the visitor here and on the server. In a preview,
 * Dimmi("configure", config) takes a config as the server would serve it,
 * which is how the messenger settings show unsaved changes.
 */

import { fromFunction, fromUrl, type Source } from "./client/identity";
import type { Content } from "./content";
import { ComposerController } from "./controllers/composer_controller";
import { MessengerController } from "./controllers/messenger_controller";
import { ScrollController } from "./controllers/scroll_controller";
import { SearchController } from "./controllers/search_controller";
import { VoiceController } from "./controllers/voice_controller";
import { Application } from "./lib/controller";
import { Live } from "./lib/live";
import { type Config, fromConfig, links } from "./lib/map";
import { report } from "./lib/report";
import { defaults, merge, type Options, type OptionsUpdate } from "./options";
import { render, style } from "./render";
import type { Command, DimmiWindow, Queue } from "./settings";
import styles from "./styles/index.css?inline";

export interface Widget {
  readonly messenger: MessengerController | undefined;
  readonly options: Options;
  readonly live: Live | undefined;
  readonly ready: Promise<void>;
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
  let preview: typeof import("./preview") | undefined;
  let sample: Content = blank;
  let ready: Promise<void> = Promise.resolve();

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
    get ready() {
      return ready;
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
      find()?.receive({ conversation, from: "team", author: preview?.teammate ?? "", body: message, at: "now" });
    }
  };

  /*
   * configure shows a config as the server would serve it, in a preview:
   * its look and words, and the company it speaks for.
   */
  const configure = (config: Config = {}) => {
    if (live) {
      report("warn", "configure_live", 'Dimmi("configure") works only in a preview.');
      return;
    }
    sample = {
      ...sample,
      company: config.company?.name || sample.company,
      logo: config.company?.logo_url ?? null,
      links: config.home_links ? links(config) : sample.links,
    };
    widget.update(fromConfig(config, options.texts));
  };

  /*
   * identify takes a new identity function, when given one, and asks who
   * is signed in now.
   */
  const identify = (argument: unknown) => {
    if (typeof argument === "function") {
      live?.setIdentity(fromFunction(argument as () => unknown));
    }
    void live?.checkIdentity();
  };

  const commands: Record<string, (argument: unknown) => void> = {
    update: (argument) => widget.update(argument as OptionsUpdate),
    boot: (argument) => widget.update(argument as OptionsUpdate),
    simulate: (argument) => simulate(argument as Simulation),
    configure: (argument) => configure(argument as Config),
    shutdown: () => void live?.logout(),
    logout: () => void live?.logout(),
    ask: (argument) => find()?.ask(typeof argument === "string" ? argument : ""),
    identify,
  };

  const run = ([name, argument]: Command) => {
    const toggle = toggles.find((known) => known === name);
    const command = Object.hasOwn(commands, name) ? commands[name] : undefined;
    if (toggle) {
      find()?.[toggle]();
    } else if (command) {
      command(argument);
    } else {
      report("warn", "unknown_command", `Dimmi("${String(name)}") is not supported yet.`);
    }
  };

  /*
   * verifyClick handles the links of an email code card: send a new code,
   * or use another address.
   */
  const verifyClick = (target: Element | null) => {
    const again = target?.closest<HTMLElement>("[data-verify-again]");
    const how = again?.dataset.verifyAgain;
    if (again?.dataset.verifyCall && (how === "resend" || how === "other")) {
      live?.verifyAgain(again.dataset.verifyCall, how);
    }
  };

  /*
   * articleClick opens an article in the full help center signed in,
   * when the messenger knows the visitor; anyone else follows the link.
   */
  const articleClick = (event: Event, target: Element | null) => {
    const article = target?.closest<HTMLAnchorElement>("a[data-help-article]");
    if (article?.dataset.helpArticle && live?.knowsVisitor()) {
      event.preventDefault();
      void live.openInHelpCenter(article.dataset.helpArticle, article.href);
    }
  };

  /*
   * verifySubmit sends an email code card's form, reporting whether the
   * event was one.
   */
  const verifySubmit = (event: Event): boolean => {
    const form =
      event.target instanceof HTMLFormElement && event.target.matches("[data-verify-call]") ? event.target : null;
    const step = form?.dataset.verifyStep;
    if (!form?.dataset.verifyCall || (step !== "email" && step !== "code")) {
      return false;
    }
    event.preventDefault();
    void live?.verifyEmail(form.dataset.verifyCall, step, String(new FormData(form).get("value") ?? ""));
    return true;
  };

  const listen = () => {
    root.addEventListener("dimmi:connect", () => app.connect(root));
    root.addEventListener("dimmi:converse", (event) => {
      void live?.openConversation((event as CustomEvent<{ conversation?: string }>).detail?.conversation);
    });
    root.addEventListener("dimmi:send", (event) => {
      const sent = (event as CustomEvent<{ body: string; files: File[]; voice?: string }>).detail;
      if (preview) {
        preview.answer(root, sent);
      } else {
        void live?.send(sent.body, sent.files);
      }
    });
    root.addEventListener("dimmi:typing", () => live?.typing(true));
    root.addEventListener("click", (event) => {
      const target = event.target instanceof Element ? event.target : null;
      const kind = target?.closest<HTMLElement>("[data-ai]")?.dataset.ai;
      if (kind === "helped" || kind === "handoff") {
        void live?.aiAction(kind);
      }
      verifyClick(target);
      articleClick(event, target);
      const card = target?.closest<HTMLElement>("[data-call]");
      const decision = card?.dataset.decision;
      if (card?.dataset.call && (decision === "confirmed" || decision === "canceled")) {
        void live?.decideAction(card.dataset.call, decision);
      }
    });
    root.addEventListener("submit", (event) => {
      if (verifySubmit(event)) {
        return;
      }
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
    ready = import("./preview").then((loaded) => {
      preview = loaded;
      sample = loaded.sample;
      mount();
    });
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

  live.setIdentity(identitySource(win, settings.identityUrl));

  ready = live.start().then((shown) => {
    if (shown) {
      mount();
    }
  });

  return widget;
}

/*
 * identitySource is where the visitor's identity token comes from: a
 * function queued with Dimmi("identify", fn) before the messenger loaded,
 * else the snippet's identity URL, else nowhere.
 */
function identitySource(win: Window & DimmiWindow, url: string | undefined): Source | undefined {
  const queued = (win.Dimmi?.q ?? []).find(([name, arg]) => name === "identify" && typeof arg === "function");
  if (queued) {
    return fromFunction(queued[1] as () => unknown);
  }
  return url ? fromUrl(win.fetch.bind(win), url) : undefined;
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
    setInterval: (fn: () => void, ms: number) => win.setInterval(fn, ms),
    page: () => ({ url: win.location.href, title: win.document.title, referrer: win.document.referrer }),
    width: () => win.innerWidth,
    language: () => win.navigator.language,
    onVisible: (fn: (hiddenFor: number) => void) => {
      let hiddenAt = 0;
      win.document.addEventListener("visibilitychange", () => {
        if (win.document.visibilityState === "hidden") {
          hiddenAt = Date.now();
        } else if (hiddenAt) {
          fn(Date.now() - hiddenAt);
        }
      });
    },
    openTab: () => {
      const tab = win.open("about:blank", "_blank");
      if (!tab) {
        return null;
      }
      tab.opener = null;
      return {
        go: (url: string) => {
          tab.location.href = url;
        },
      };
    },
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
