/*
 * @dimmi/browser puts the dimmi messenger on any web page without a
 * framework. loadDimmi adds the loader script once, and every call made
 * before it arrives waits in the same queue the paste-in snippet uses, so
 * nothing is lost and order is kept. Every function is safe to call on a
 * server, where it does nothing.
 */

/* The npm scope every dimmi package is published under. */
export const SCOPE = "@dimmi";

/* Where the loader lives on any dimmi host, cloud or self-hosted. */
export const LOADER_PATH = "/widget/v1/loader.js";

/* The window event the messenger sends when the visitor's unread count changes. */
export const UNREAD_EVENT = "dimmi:unread";

export type Command = [name: string, ...args: unknown[]];

export interface Queue {
  (...command: Command): void;
  q?: Command[];
}

type DimmiWindow = Window & { Dimmi?: Queue };

/*
 * What a signed-in visitor's identity comes from: a function answering
 * the token your server signed with @dimmi/server, or null when nobody is
 * signed in.
 */
export type IdentitySource = () => Promise<string | null> | string | null;

/*
 * Messenger settings you may change at runtime, as Dimmi("update") takes
 * them. Anything you leave out keeps what the dashboard set.
 */
export interface MessengerSettings {
  appearance?: {
    primary_color?: string;
    header_color?: string;
    button_color?: string;
    theme?: "light" | "dark" | "auto";
    position?: "right" | "left";
    padding_x?: number;
    padding_y?: number;
    launcher?: { style?: "bubble" | "tab" | "hidden"; shape?: "circle" | "squircle"; label?: string };
    font?: "system" | "inherit";
    sound?: boolean;
  };
  texts?: Record<string, string>;
  /* The visitor's language, such as "fr" or "pt-BR", over their browser's. */
  language?: string;
  [key: string]: unknown;
}

export interface DimmiOptions {
  /* The messenger's app id, wgt_..., from Settings > Messenger. */
  appId: string;
  /* Where dimmi runs: https://your-dimmi-host, or your self-hosted address. */
  host: string;
  /* Where the messenger's API calls go, when not the host itself (a proxy, say). */
  apiBase?: string;
  /* A nonce for the loader script under a strict CSP with 'strict-dynamic'. */
  nonce?: string;
  /* Answers the signed-in visitor's identity token, or null. */
  identity?: IdentitySource;
  /* A URL on your site answering the token, instead of identity. */
  identityUrl?: string;
  /* Settings applied once the messenger boots. */
  settings?: MessengerSettings;
}

export interface DimmiClient {
  /* Resolves once the loader script has loaded; rejects if it was blocked. */
  readonly ready: Promise<void>;
  /* Sends any raw command, Dimmi(name, ...args). */
  command(name: string, ...args: unknown[]): void;
  /* Merges settings over the messenger's and redraws it. */
  update(settings: MessengerSettings): void;
  /* Opens the messenger. */
  show(): void;
  /* Closes the messenger. */
  hide(): void;
  /* Opens the messenger when closed, closes it when open. */
  toggle(): void;
  /* Opens a new conversation, sending text in it when given. */
  showNewMessage(text?: string): void;
  /* Sets where the identity token comes from and checks who is signed in. */
  identify(source?: IdentitySource): void;
  /* Forgets the visitor here and on the server, for your sign-out. */
  shutdown(): void;
  /* Records an event against the visitor. */
  trackEvent(name: string, metadata?: Record<string, unknown>): void;
  /* Marks a route change in a single-page app; the messenger also notices on its own. */
  page(): void;
  /* Calls fn with the visitor's unread count as it changes; returns an unsubscribe. */
  onUnreadCountChange(fn: (count: number) => void): () => void;
}

/*
 * isBrowser reports whether this runs where a window exists.
 */
export function isBrowser(): boolean {
  return typeof window !== "undefined" && typeof document !== "undefined";
}

/*
 * queue returns the page's Dimmi function, adding the snippet's stub
 * that holds calls until the messenger arrives.
 */
export function queue(): Queue | undefined {
  if (!isBrowser()) {
    return undefined;
  }
  const win = window as DimmiWindow;
  if (!win.Dimmi) {
    const stub: Queue = (...command: Command) => {
      stub.q = stub.q ?? [];
      stub.q.push(command);
    };
    win.Dimmi = stub;
  }
  return win.Dimmi;
}

/*
 * dimmi sends one command to the messenger, queued until it loads.
 */
export function dimmi(name: string, ...args: unknown[]): void {
  queue()?.(name, ...args);
}

/*
 * loaderUrl is the loader's address on a host.
 */
export function loaderUrl(host: string): string {
  return `${host.replace(/\/+$/, "")}${LOADER_PATH}`;
}

let loading: Promise<void> | undefined;

/*
 * inject adds the loader script, once per page.
 */
function inject(options: DimmiOptions): Promise<void> {
  const existing = document.querySelector<HTMLScriptElement>("script[data-dimmi-loader]");
  if (existing) {
    loading = loading ?? Promise.resolve();
    return loading;
  }
  const script = document.createElement("script");
  script.src = loaderUrl(options.host);
  script.async = true;
  script.dataset.appId = options.appId;
  script.dataset.dimmiLoader = "";
  if (options.apiBase) {
    script.dataset.apiBase = options.apiBase.replace(/\/+$/, "");
  }
  if (options.identityUrl) {
    script.dataset.identityUrl = options.identityUrl;
  }
  if (options.nonce) {
    script.nonce = options.nonce;
  }
  loading = new Promise<void>((resolve, reject) => {
    script.addEventListener("load", () => resolve(), { once: true });
    script.addEventListener(
      "error",
      () => {
        loading = undefined;
        reject(new Error(`dimmi: ${script.src} did not load. An ad blocker or CSP may block it.`));
      },
      { once: true },
    );
  });
  loading.catch(() => undefined);
  (document.body ?? document.head).append(script);
  return loading;
}

const noop = () => undefined;

/*
 * client is the typed API over the page's Dimmi function.
 */
function client(ready: Promise<void>): DimmiClient {
  return {
    ready,
    command: dimmi,
    update: (settings) => dimmi("update", settings),
    show: () => dimmi("open"),
    hide: () => dimmi("close"),
    toggle: () => dimmi("toggle"),
    showNewMessage: (text) => (text ? dimmi("ask", text) : dimmi("open")),
    identify: (source) => (source ? dimmi("identify", source) : dimmi("identify")),
    shutdown: () => dimmi("logout"),
    trackEvent: (name, metadata) => dimmi("track", name, metadata ?? {}),
    // The messenger compares the address itself every few seconds and
    // tells the server; page() is the hook routers call, kept so a later
    // messenger can react at once without app changes.
    page: noop,
    onUnreadCountChange: (fn) => {
      if (!isBrowser()) {
        return noop;
      }
      const listener = (event: Event) => {
        const count = Number((event as CustomEvent<{ count?: number }>).detail?.count ?? 0);
        fn(Number.isFinite(count) ? count : 0);
      };
      window.addEventListener(UNREAD_EVENT, listener);
      return () => window.removeEventListener(UNREAD_EVENT, listener);
    },
  };
}

/*
 * loadDimmi loads the messenger once and answers its client. Calling it
 * again, from another component say, adds no second script; it only
 * applies the new identity and settings.
 */
export function loadDimmi(options: DimmiOptions): DimmiClient {
  if (!isBrowser()) {
    return client(Promise.resolve());
  }
  if (!options.appId || !options.host) {
    throw new Error("dimmi: loadDimmi needs appId and host.");
  }
  queue();
  if (options.identity) {
    const source = options.identity;
    dimmi("identify", async () => source());
  }
  if (options.settings) {
    dimmi("boot", options.settings);
  }
  return client(inject(options));
}

/*
 * getDimmi answers the client for a messenger loaded elsewhere, such as
 * by the paste-in snippet; calls queue until it arrives.
 */
export function getDimmi(): DimmiClient {
  return client(loading ?? Promise.resolve());
}

/*
 * resetDimmi forgets the loaded state, for tests.
 */
export function resetDimmi(): void {
  loading = undefined;
}
