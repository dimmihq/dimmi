/*
 * The messenger itself. It mounts in a shadow root, so the host page's CSS
 * never reaches it and its own never leaks out, and styles it through a
 * constructed stylesheet, which a strict content security policy allows.
 * Then it takes over the global Dimmi function: commands queued before it
 * arrived run in order, later ones run at once, and an unknown command
 * says so in the console instead of failing quietly. Elements on the page
 * marked data-dimmi="open", "close" or "toggle" drive it without any code.
 *
 * The options start from the defaults and the content from a sample until
 * the server sends both; Dimmi("update", { ... }) merges changes over the
 * options and re-renders, keeping the panel open on the same tab and the
 * views that already loaded. Until the server is wired, content arrives
 * after a simulated latency, 700 ms unless the loader's data-latency says
 * otherwise; Dimmi("simulate", { latency }) changes it and loads every
 * view again, to preview loading states, and Dimmi("simulate", { message })
 * delivers a message from the team. On a real page the sample's message
 * arrives a few seconds after load, so the unread states can be seen.
 */
import { sample } from "./content";
import { ComposerController } from "./controllers/composer_controller";
import { MessengerController } from "./controllers/messenger_controller";
import { ScrollController } from "./controllers/scroll_controller";
import { SearchController } from "./controllers/search_controller";
import { VoiceController } from "./controllers/voice_controller";
import { Application } from "./lib/controller";
import { report } from "./lib/report";
import { defaults, merge, type Options, type OptionsUpdate } from "./options";
import { render, style } from "./render";
import type { Command, DimmiWindow, Queue } from "./settings";
import styles from "./styles/index.css?inline";

export interface Widget {
  readonly messenger: MessengerController | undefined;
  readonly options: Options;
  update(changes: OptionsUpdate): void;
}

const toggles = ["open", "close", "toggle"] as const;

interface Simulation {
  latency?: number;
  message?: string;
}

export function boot(win: Window & DimmiWindow, doc: Document): Widget | undefined {
  if (!win.__dimmi) {
    report("error", "loader_missing", "load the widget with /widget/v1/loader.js, not the core file.");
    return undefined;
  }

  const host = doc.createElement("div");
  host.id = "dimmi-messenger";
  const root = host.attachShadow({ mode: "open" });
  const sheet = new CSSStyleSheet();
  sheet.replaceSync(styles);
  root.adoptedStyleSheets = [sheet];
  doc.body.append(host);

  const app = new Application(root)
    .register("messenger", MessengerController)
    .register("search", SearchController)
    .register("composer", ComposerController)
    .register("voice", VoiceController)
    .register("scroll", ScrollController);
  const find = () => app.controller<MessengerController>(root.firstElementChild as HTMLElement, "messenger");
  let options = defaults;
  let latency = win.__dimmi.latency ?? 700;

  const draw = () => {
    const before = find();
    const [wasOpen, tab] = [before?.isOpen, before?.tab ?? ""];
    const loaded = [...root.querySelectorAll<HTMLElement>('.view[data-state="ready"]')].map(
      (view) => view.dataset.view,
    );
    app.stop();
    style(host, options);
    root.innerHTML = render(options, sample);
    (root.firstElementChild as HTMLElement).dataset.latency = String(latency);
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
    update(changes) {
      options = merge(options, changes);
      draw();
    },
  };

  /*
   * simulate stands in for the server while it is not wired: a new latency
   * loads every view again, and a message arrives as if the team sent it.
   */
  const simulate = ({ latency: next, message }: Simulation = {}) => {
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
    } else {
      report("warn", "unknown_command", `Dimmi("${String(name)}") is not supported yet.`);
    }
  };

  draw();
  app.start();
  root.addEventListener("dimmi:connect", () => app.connect(root));
  const queued = win.Dimmi?.q ?? [];
  const dimmi: Queue = (...command) => run(command);
  win.Dimmi = dimmi;
  queued.forEach(run);

  const incoming = sample.incoming;
  if (win.__dimmi.demo && incoming) {
    setTimeout(() => find()?.receive(incoming), 4000);
  }

  doc.addEventListener("click", (event) => {
    const trigger = event.target instanceof Element ? event.target.closest<HTMLElement>("[data-dimmi]") : null;
    if (trigger?.dataset.dimmi) {
      run([trigger.dataset.dimmi]);
    }
  });

  return widget;
}
