/*
 * The messenger connected to its server. It loads the widget's config and
 * the team's availability first, and only when the visitor already has a
 * session does it load their conversations and open the stream; a first
 * visit costs the config and nothing else until the visitor writes.
 *
 * The views ask through events: "dimmi:converse" to show a conversation, or a
 * new one when none is named, and "dimmi:send" to send what the composer
 * holds. Under the agent's latest answer the visitor can say it helped or
 * ask for a person; either shows at once and is sent in the background.
 * An action card's Confirm or Cancel waits on the server, since the
 * choice decides whether something runs. Answers land in the store, and
 * only the parts of the panel that changed are drawn again, so a
 * half-written message, the scroll position and the open tab survive
 * every update.
 */

import { ApiError, Http } from "../client/http";
import type { Source } from "../client/identity";
import { Outbox, type Pending, type Sent } from "../client/outbox";
import { Realtime } from "../client/realtime";
import { Session, type SessionData, type Storage } from "../client/session";
import { type Conversation as ApiConversation, type Message as ApiMessage, Store } from "../client/store";
import { ConversationRow } from "../components/rows";
import { Whoami } from "../components/whoami";
import type { Block, Collection, Content, Message, VerifyStep } from "../content";

/*
 * HelpIndex is what the server answers for the Help tab.
 */
interface HelpIndex {
  url: string;
  collections: Collection[];
  popular: string[];
  articles: { slug: string; title: string; summary: string; updated: string; body: Block[] }[];
}

import type { Options, OptionsUpdate, Tab } from "../options";
import type { ActionTexts } from "../text";
import { type Availability, type Config, content, fromConfig, reading } from "./map";
import { patch } from "./patch";
import { report } from "./report";
import type { Surface } from "./rules";
import { loadThread, type Parts, thread as threadParts } from "./threads";
import { loadTickets, tickets } from "./tickets";

export interface Host {
  root: ShadowRoot;
  options(): Options;
  update(changes: OptionsUpdate): void;
  notify(message: { conversation: string; body: string; author: string }): void;
  showing(conversation: string): boolean;
}

export interface Browser {
  fetch: typeof fetch;
  storage: Storage;
  eventSource: (url: string) => {
    onmessage: ((e: { data: string }) => void) | null;
    onerror: ((e: unknown) => void) | null;
    close(): void;
  };
  now: () => number;
  uuid: () => string;
  setTimeout: (fn: () => void, ms: number) => unknown;
  clearTimeout: (handle: unknown) => void;
  page: () => { url: string; title?: string; referrer?: string };
  width?: () => number;
  language?: () => string;
  setInterval: (fn: () => void, ms: number) => unknown;
  onVisible?: (fn: (hiddenFor: number) => void) => void;
  openTab?: () => { go: (url: string) => void } | null;
}

/*
 * routeCheck is how often the page's address is compared with the last
 * one, to notice a single-page app moving to another route.
 */
const routeCheck = 1000;

/*
 * How often identity is asked again: once the tab is visible after a
 * minute away, and at most every 30 seconds of navigation while nobody is
 * signed in.
 */
const visibleRecheck = 60_000;
const anonymousRecheck = 30_000;

/*
 * callCheck is how often, in milliseconds, a waiting action card asks
 * where its call stands.
 */
const callCheck = 5000;

/*
 * WidgetCall is a call as the messenger reads it: where it stands, in
 * state and again in status.
 */
interface WidgetCall {
  id: string;
  title: string;
  summary: string;
  state: string;
  status: string;
}

/*
 * draftId names the conversation a visitor writes before the server gave
 * it an id.
 */
const draftId = "draft";

/*
 * Codes that mean the messenger must not show at all here, with the line
 * the page's developer reads in the console.
 */
const fatal: Record<string, string> = {
  app_not_found: "the app id on the snippet names no messenger. Copy the snippet again from your settings.",
  widget_disabled: "this messenger is turned off in its settings.",
  origin_not_allowed: "this site is not on the messenger's allowed sites. Add it in the messenger's settings.",
  workspace_suspended: "this workspace is suspended.",
};

/*
 * followBottom keeps a thread at its latest message while the visitor
 * reads there: a picture that finishes loading would otherwise push the
 * newest messages out of view. Once the visitor scrolls up, the thread
 * stays where they are until they come back down. Opening a conversation
 * jumps straight to its end; a message arriving afterwards glides in.
 */
function followBottom(thread: HTMLElement, at: { pinned: boolean; jump: boolean }) {
  if (!thread.dataset.following) {
    thread.dataset.following = "true";
    thread.addEventListener("scroll", () => {
      thread.dataset.pinned = String(thread.scrollHeight - thread.scrollTop - thread.clientHeight < 48);
    });
    thread.addEventListener(
      "load",
      () => {
        if (thread.dataset.pinned !== "false") {
          thread.style.scrollBehavior = "auto";
          thread.scrollTop = thread.scrollHeight;
          thread.style.scrollBehavior = "";
        }
      },
      true,
    );
  }
  if (!at.pinned) {
    return;
  }
  if (at.jump) {
    thread.style.scrollBehavior = "auto";
  }
  thread.scrollTop = thread.scrollHeight;
  thread.dataset.pinned = "true";
  thread.style.scrollBehavior = "";
}

/*
 * place puts the markup into the thread: messages already there stay put
 * and only new ones animate, and the thread lands on its latest message,
 * straight away until the conversation's messages first arrive.
 */
function place(at: { thread: HTMLElement; loaded: HTMLElement; open: string; shown: number }, markup: string) {
  const same = at.loaded.dataset.conversation === at.open;
  const settled = same && at.loaded.dataset.settled === "true";
  at.loaded.dataset.conversation = at.open;
  at.loaded.dataset.settled = String(at.shown > 0);
  patch(at.loaded, markup, same);
  followBottom(at.thread, { pinned: !settled || at.thread.dataset.pinned !== "false", jump: !settled });
}

export class Live {
  readonly store: Store;
  readonly http: Http;
  readonly session: Session;
  readonly outbox: Outbox;
  readonly realtime: Realtime;
  config: Config = {};
  availability: Availability | undefined;
  open: string | undefined;
  private readonly helped = new Set<string>();
  private readonly deciding = new Set<string>();
  private readonly verifying = new Map<string, VerifyStep>();
  private help: Pick<Content, "collections" | "articles" | "popular" | "help_url"> | undefined;
  private changelog: Pick<Content, "changelog" | "unread_changelog"> | undefined;
  private latestEntry = "";
  private readonly rated = new Map<string, "yes" | "no">();
  private ratings: import("./rating").Ratings | undefined;
  private ratingsLoading = false;
  private callTimer: unknown;
  private identity: Source | undefined;
  private checking: Promise<void> | undefined;
  private lastAnonymousCheck = 0;
  private surface: Surface | undefined;
  private rules: typeof import("./rules") | undefined;
  private allTabs: Tab[] = [];
  private readonly events: unknown[][] = [];
  private unread = -1;
  private shown = "";
  private ticketing: Pick<Content, "ticket_types"> = {};
  private openOnly = false;

  constructor(
    private readonly host: Host,
    private readonly browser: Browser,
    private readonly appId: string,
    apiBase: string,
    private readonly base: Content,
  ) {
    this.http = new Http({ fetch: browser.fetch, now: browser.now }, apiBase);
    this.store = new Store(browser.now);
    this.session = new Session(this.http, browser.storage, appId);
    this.outbox = new Outbox(this.http, browser.storage, appId, browser, {
      sent: (sent) => this.sent(sent),
      refused: (pending, error) => this.refused(pending, error),
      ready: async () => {
        await this.session.ensure(this.startDetails());
        this.streaming();
      },
    });
    this.realtime = new Realtime(
      this.http,
      {
        eventSource: browser.eventSource,
        setTimeout: browser.setTimeout,
        clearTimeout: browser.clearTimeout,
        now: browser.now,
      },
      appId,
      {
        event: (e) => this.event(e.event, e.data),
        resync: () => void this.reload(),
        ended: () => this.ended(),
        mode: (mode) => {
          host.root.firstElementChild?.setAttribute("data-realtime", mode);
        },
      },
    );
    this.store.subscribe((change, conversation) => this.changed(change, conversation));
  }

  /*
   * start loads what the panel needs and answers false when the messenger
   * must stay hidden on this page: when the workspace requires signed-in
   * visitors and this one is not. When sign-in is off, the page's
   * identity token is not even asked for.
   */
  async start(): Promise<boolean> {
    try {
      this.config = (await this.http.get<{ data: Config }>(this.localized("config"))).data;
    } catch (error) {
      const code = error instanceof ApiError ? error.code : "network";
      report("error", code, fatal[code] ?? "the messenger's config could not be loaded.");
      return false;
    }

    const mode = this.config.features?.identity_mode;
    if (mode === "off" && this.identity) {
      report("warn", "identity_mode_off", "sign-in is off for this workspace, so the identity token is ignored.");
      this.identity = undefined;
    }

    this.host.update(fromConfig(this.config));
    this.allTabs = this.host.options().tabs;
    this.surface = this.config.surface?.anonymous;
    this.applySurface();
    void this.loadAvailability();
    if (this.config.tabs?.includes("changelog")) {
      void this.loadChangelog();
    }

    const session = await this.handshake().catch(() => undefined);
    this.takeSession(session);
    if (mode === "required" && !this.session.signedIn) {
      report("warn", "identity_required", "this messenger is only shown to signed-in customers.");
      return false;
    }
    if (session) {
      await this.reload();
      this.streaming();
    }
    if (this.outbox.pending.length > 0) {
      void this.outbox.flush();
    }
    this.watchRoutes();
    this.tellUnread();

    return true;
  }

  /*
   * setIdentity sets where the visitor's identity token comes from. Set
   * after the messenger started, it checks who is signed in at once.
   */
  setIdentity(source: Source | undefined) {
    this.identity = source;
  }

  /*
   * handshake picks the visitor's session up on page load: by identity
   * token when the page has an identity source, else by the stored token.
   */
  /*
   * logout ends the visitor's session and forgets everything about it
   * here, so the next person on this browser starts fresh.
   */
  async logout() {
    this.realtime.stop();
    await this.session.signOut();
    this.ended();
  }

  private async handshake() {
    if (!this.identity) {
      return this.session.resume();
    }
    return this.session.identify(await this.identity(), this.startDetails());
  }

  /*
   * checkIdentity asks the identity source again, mid-session: signing
   * the visitor in, switching to another customer, or signing them out.
   * Nothing is sent when the same customer is still signed in. When the
   * session changes, the messenger shows the new visitor's conversations
   * only. Several checks at once share one.
   */
  checkIdentity(): Promise<void> {
    if (!this.identity) {
      return Promise.resolve();
    }
    this.checking ??= this.recheck().finally(() => {
      this.checking = undefined;
    });
    return this.checking;
  }

  private async recheck() {
    const source = this.identity;
    if (!source) {
      return;
    }
    const token = await source();
    if (token && this.session.signedIn && (await this.session.sameCustomer(token))) {
      return;
    }
    if (!token && !this.session.signedIn) {
      return;
    }
    const before = this.session.current?.id;
    const after = await this.session.identify(token, this.startDetails()).catch(() => undefined);
    this.takeSession(after);
    if (after?.id === before) {
      return;
    }
    this.realtime.stop();
    this.ended(after);
    if (after) {
      await this.reload();
      this.streaming();
    }
  }

  /*
   * watchRoutes tells the server where the visitor is once a page that
   * never reloads, such as a single-page app, moves to another address,
   * and checks who is signed in: on every move while signed in, so a
   * sign-out shows at once, and at most every 30 seconds otherwise.
   */
  private watchRoutes() {
    let last = this.browser.page().url;
    this.browser.setInterval(() => {
      const page = this.browser.page();
      if (page.url === last) {
        return;
      }
      last = page.url;
      this.applySurface();
      if (this.session.current) {
        void this.http.post("/pings", { page }).catch(() => undefined);
      }
      this.routeIdentityCheck();
    }, routeCheck);
    this.browser.onVisible?.((hiddenFor) => {
      if (hiddenFor >= visibleRecheck) {
        void this.checkIdentity();
      }
    });
  }

  private routeIdentityCheck() {
    const now = this.browser.now();
    if (!this.session.signedIn && now - this.lastAnonymousCheck < anonymousRecheck) {
      return;
    }
    this.lastAnonymousCheck = now;
    void this.checkIdentity();
  }

  /*
   * content is what the views draw now.
   */
  content(): Content {
    const out = content(this.base, this.config, this.availability, this.store, this.open, this.browser.now());
    const help = this.help
      ? {
          ...this.help,
          articles: this.help.articles.map((a) => {
            const rated = this.rated.get(a.slug);
            return rated ? { ...a, rated } : a;
          }),
        }
      : {};

    return { ...out, ...help, ...this.changelog, ...this.ticketing };
  }

  /*
   * viewed tells the server the visitor opened an article, which it
   * counts for the team's analytics. Nothing waits on it and a failure
   * is only reported. A visitor without a session is counted through the
   * app's own address, so reading never starts one.
   */
  async viewed(article: string) {
    try {
      await this.http.post(this.session.current ? "/article-views" : `/apps/${this.appId}/article-views`, { article });
    } catch (error) {
      const code = error instanceof ApiError ? error.code : "network";
      report("warn", code, "the article view could not be counted.");
    }
  }

  /*
   * rate tells the server whether an article helped, starting the
   * visitor's session first when they have none. The answer already
   * shows; when the server refuses it, the article goes back to what the
   * visitor answered before.
   */
  async rate(article: string, helpful: boolean) {
    const before = this.rated.get(article);
    this.rated.set(article, helpful ? "yes" : "no");
    try {
      await this.session.ensure(this.startDetails());
      await this.http.post("/article-ratings", { article, helpful });
    } catch (error) {
      if (before) {
        this.rated.set(article, before);
      } else {
        this.rated.delete(article);
      }
      const code = error instanceof ApiError ? error.code : "network";
      report("warn", code, "the answer about the article could not be sent.");
      this.host.update({});
    }
  }

  /*
   * loadChangelog reads the newest published entries of the changelog
   * the messenger links to and draws the What's new tab with them. The
   * tab carries a dot while an entry is newer than the newest one the
   * visitor saw there on this browser.
   */
  private async loadChangelog() {
    try {
      const read = await import("./changelog");
      const tab = await read.changelog(this.http, this.appId, this.browser.storage.get(this.seenKey));
      this.latestEntry = tab.latest;
      this.changelog = tab;
      this.host.update({});
    } catch {
      this.changelog = undefined;
    }
  }

  /*
   * readChangelog remembers the newest entry as seen once the visitor
   * opens the What's new tab, and takes the tab's dot away.
   */
  readChangelog() {
    if (!this.changelog || this.changelog.unread_changelog === 0) {
      return;
    }
    this.browser.storage.set(this.seenKey, this.latestEntry);
    this.changelog = { ...this.changelog, unread_changelog: 0 };
    this.host.root.querySelector('[data-tab="changelog"] .dot')?.remove();
  }

  private get seenKey(): string {
    return `dimmi:${this.appId}:changelog_seen`;
  }

  /*
   * loadHelp reads the help center the messenger links to: its
   * collections and published articles, each body as plain blocks, and
   * draws the Help tab with them. A visitor with a session reads it as
   * themselves, so articles kept to their audiences show too. Without a
   * help center the tab stays empty.
   */
  private async loadHelp() {
    const path = this.session.current ? "/help" : `/apps/${this.appId}/help`;
    try {
      const res = await this.http.get<{ data: HelpIndex }>(path);
      this.help = {
        help_url: res.data.url || undefined,
        collections: res.data.collections,
        popular: res.data.popular,
        articles: res.data.articles.map((a) => ({
          slug: a.slug,
          title: a.title,
          summary: a.summary,
          author: "",
          updated: new Date(a.updated).toLocaleDateString(reading.locale),
          body: a.body,
        })),
      };
      this.host.update({});
    } catch {
      this.help = undefined;
    }
  }

  /*
   * knowsVisitor says the visitor has a session the help center could
   * be opened for.
   */
  knowsVisitor(): boolean {
    return Boolean(this.session.current);
  }

  /*
   * openInHelpCenter opens an article in the full help center, signed in
   * when the messenger knows the visitor. The new tab opens at once, so
   * no popup blocker stops it, and moves to the address with its pass
   * once the server answers; the plain address is the fallback.
   */
  async openInHelpCenter(slug: string, plain: string) {
    const opened = this.browser.openTab?.();
    if (!opened) {
      return;
    }
    try {
      const res = await this.http.post<{ data: { url: string } }>("/help-passes", { article: slug });
      opened.go(res.data.url);
    } catch {
      opened.go(plain);
    }
  }

  /*
   * openConversation shows the conversation, or a new one when none is
   * named, and marks what the team wrote there read.
   */
  async openConversation(id: string | undefined) {
    this.open = id ?? draftId;
    this.drawThread();
    if (!id) {
      return;
    }

    try {
      const res = await this.http.get<{ data: ApiMessage[] }>(`/conversations/${id}/messages`);
      this.store.setMessages(id, res.data);
    } catch {
      return;
    }
    void this.markRead(id);
  }

  /*
   * send queues what the visitor wrote in the open conversation and shows
   * it at once. choice names the path of a button they tapped, so the
   * workflow takes it whatever the button reads.
   */
  async send(body: string, files: File[], choice?: string) {
    const conversation = this.open && this.open !== draftId ? this.open : undefined;
    let attachmentIds: string[] = [];
    if (files.length > 0) {
      attachmentIds = await this.upload(files);
    }

    const pending = this.outbox.send({ conversation, body, attachmentIds, choice });
    this.store.addLocal({
      id: `local:${pending.clientId}`,
      conversation_id: conversation ?? draftId,
      client_id: pending.clientId,
      author: { type: "contact", name: null, avatar_url: null },
      body,
      attachments: [],
      created_at: new Date(this.browser.now()).toISOString(),
      sending: true,
    });
  }

  /*
   * typing tells the team the visitor is writing, or stopped.
   */
  typing(started: boolean) {
    if (!this.open || this.open === draftId || !this.session.current) {
      return;
    }
    void this.http
      .post(`/conversations/${this.open}/typing-indicators`, { state: started ? "started" : "stopped" })
      .catch(() => undefined);
  }

  /*
   * loadTicketing reads the request types Home offers, once, the first
   * time the panel opens, through the app's address before the visitor
   * has a session. A visitor who
   * never opens the messenger never asks.
   */
  async loadTicketing() {
    if (this.ticketing.ticket_types) {
      return;
    }
    this.ticketing = { ticket_types: [] };
    const base = this.session.current ? "" : `/apps/${this.appId}`;
    const get = (path: string) =>
      this.http.get<{ data: [] }>(base + path).then(
        (res) => res.data,
        () => [],
      );
    const ticket_types = await get("/ticket-types");
    if (ticket_types.length) {
      await loadTickets(this.host.root).catch(() => undefined);
    }
    this.ticketing = { ticket_types };
    this.host.update({});
  }

  /*
   * filterOpen narrows the Messages list to the open requests, or shows
   * every conversation again.
   */
  filterOpen(open: boolean) {
    this.openOnly = open;
    for (const button of this.host.root.querySelectorAll("[data-ticket-filter]")) {
      button.setAttribute("aria-pressed", String((button.getAttribute("data-ticket-filter") === "open") === open));
    }
    this.drawConversations();
  }

  /*
   * ensure starts the visitor's session when they have none.
   */
  async ensure() {
    await this.session.ensure(this.startDetails());
  }

  /*
   * reloadConversations reads the visitor's conversations again.
   */
  reloadConversations() {
    return this.reload();
  }

  async upload(files: File[]): Promise<string[]> {
    await this.session.ensure(this.startDetails());
    const ids: string[] = [];
    for (const file of files) {
      try {
        ids.push(await this.uploadOne(file));
      } catch (error) {
        const code = error instanceof ApiError ? error.code : "network";
        report("warn", code, `${file.name} could not be attached.`);
      }
    }
    return ids;
  }

  private async uploadOne(file: File): Promise<string> {
    const start = await this.http.post<{
      data: { id: string; upload: { method: string; url: string; headers: Record<string, string> } };
    }>("/attachments", {
      filename: file.name,
      content_type: file.type || "application/octet-stream",
      size_bytes: file.size,
    });
    const { id, upload } = start.data;

    if (upload.url.includes("/widget/v1/attachments/")) {
      await this.http.request("PUT", upload.url, { raw: file, contentType: upload.headers["Content-Type"] });
    } else {
      const put = await this.browser.fetch(upload.url, {
        method: "PUT",
        body: file,
        headers: { "Content-Type": upload.headers["Content-Type"] ?? file.type },
      });
      if (!put.ok) {
        throw new ApiError(put.status, "upload_failed", "The file could not be uploaded.");
      }
    }

    await this.http.patch(`/attachments/${id}`, { status: "uploaded" });
    return id;
  }

  private async reload() {
    try {
      const res = await this.http.get<{ data: ApiConversation[] }>("/conversations");
      this.store.setConversations(res.data);
    } catch {
      return;
    }
    if (this.open && this.open !== draftId) {
      await this.openConversation(this.open);
    }
  }

  /*
   * localized is the address of the widget's config or availability in
   * the visitor's language: the one the page chose, else the browser's.
   */
  private localized(path: string): string {
    const language = this.browser.language?.();
    return `/apps/${this.appId}/${path}${language ? `?locale=${encodeURIComponent(language)}` : ""}`;
  }

  /*
   * relocalize loads the config again after the page chose another
   * language, and redraws the messenger's words and direction with it.
   */
  async relocalize() {
    try {
      this.config = (await this.http.get<{ data: Config }>(this.localized("config"))).data;
    } catch {
      return;
    }
    this.host.update(fromConfig(this.config));
    this.redraw();
    void this.loadAvailability();
  }

  /*
   * redraw puts back what only the connection draws, such as the open
   * thread's delivery line and the visitor's conversations, after the
   * whole messenger was drawn again.
   */
  redraw() {
    this.drawConversations();
    this.drawThread();
  }

  private async loadAvailability() {
    try {
      this.availability = (await this.http.get<{ data: Availability }>(this.localized("availability"))).data;
      this.drawConversations();
      this.drawHeader();
    } catch {
      this.availability = undefined;
    }
  }

  private async markRead(conversation: string) {
    const list = this.store.messages.get(conversation) ?? [];
    const last = [...list].reverse().find((m) => m.author.type !== "contact" && !m.id.startsWith("local:"));
    if (!last) {
      return;
    }
    this.store.markRead(conversation);
    await this.http
      .patch(`/conversations/${conversation}/read-state`, { last_read_message_id: last.id })
      .catch(() => undefined);
  }

  private streaming() {
    if (this.session.current) {
      this.realtime.start();
      this.sendEvents();
    }
  }

  /*
   * tellUnread tells the page the visitor's unread total, once at start
   * and whenever it changes, as a window "dimmi:unread" event.
   */
  private tellUnread() {
    const count = this.store.unread;
    if (count !== this.unread) {
      this.unread = count;
      globalThis.dispatchEvent?.(new CustomEvent("dimmi:unread", { detail: { count } }));
    }
  }

  /*
   * track keeps an event the page reported, Dimmi("track", name,
   * metadata), until the visitor has a session; the sender is its own
   * chunk.
   */
  track(name: unknown, metadata: unknown) {
    this.events.push([name, metadata, this.browser.uuid()]);
    this.sendEvents();
  }

  private sendEvents() {
    if (this.session.current && this.events.length) {
      void import("./events").then((sender) => sender.send(this.http, this.events));
    }
  }

  private event(event: string, data: Record<string, unknown>) {
    this.store.apply(event, data);
    const asked = data.conversation_id;
    if ((event === "conversation.rating_requested" || event === "conversation.reopened") && typeof asked === "string") {
      this.ratings?.forget(asked);
      if (this.open === asked) {
        this.drawThread();
      }
    }
    const message = data.message as ApiMessage | undefined;
    if (event !== "message.created" || !message || message.author.type === "contact") {
      return;
    }
    if (this.host.showing(message.conversation_id)) {
      void this.markRead(message.conversation_id);
      return;
    }
    this.host.notify({
      conversation: message.conversation_id,
      body: message.body ?? "",
      author: message.author.name ?? "",
    });
  }

  private sent({ pending, message, conversation, conversationId }: Sent) {
    if (conversation) {
      this.store.upsertConversation(conversation as unknown as ApiConversation);
    }
    if (!pending.conversation) {
      this.store.moveLocal(draftId, conversationId);
      if (this.open === draftId) {
        this.open = conversationId;
      }
    }
    this.store.putMessage(message as unknown as ApiMessage);
    this.streaming();
  }

  private refused(pending: Pending, error: ApiError) {
    this.store.failLocal(pending.conversation ?? draftId, pending.clientId, error.code);
    if (error.code !== "email_required") {
      report("warn", error.code, error.message);
    }
  }

  /*
   * takeSession takes what the messenger's rules give a known visitor,
   * or what they give anyone when there is no session.
   */
  private takeSession(session: SessionData | undefined) {
    this.surface = session?.surface ?? this.config.surface?.anonymous;
    this.applySurface();
    if (this.config.tabs?.includes("help")) {
      void this.loadHelp();
    }
  }

  /*
   * applySurface shows the launcher and tabs the rules allow on this
   * page, drawing again only when that changed. The rule checker is its
   * own chunk, fetched only by a messenger that has rules.
   */
  private applySurface() {
    if (this.surface && !this.rules) {
      void import("./rules").then((rules) => {
        this.rules = rules;
        this.applySurface();
      });
      return;
    }
    const visit = { ...this.browser.page(), width: this.browser.width?.(), language: this.browser.language?.() };
    const decide = (d: Parameters<typeof import("./rules").decide>[0]) => this.rules?.decide(d, visit) ?? true;
    const tabs = this.allTabs.filter((tab) => decide(this.surface?.tabs?.[tab]));
    const ruledOut = !decide(this.surface?.launcher);
    const key = `${ruledOut}|${tabs.join(",")}`;
    if (key === this.shown) {
      return;
    }
    this.shown = key;
    this.host.update({ launcher_ruled_out: ruledOut, tabs });
  }

  /*
   * ended clears the visitor who left: their conversations and thread,
   * and their session unless the messenger already holds the next
   * visitor's, which it keeps.
   */
  private ended(kept?: SessionData) {
    if (!kept) {
      this.session.forget();
    }
    this.takeSession(kept);
    this.store.setConversations([]);
    this.store.messages.clear();
    this.open = undefined;
    this.drawThread();
  }

  private startDetails() {
    const page = this.browser.page();
    return {
      page,
      locale: this.browser.language?.(),
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    };
  }

  private changed(change: string, conversation?: string) {
    this.tellUnread();
    if (change === "conversations" || change === "unread") {
      this.drawConversations();
    }
    if ((change === "messages" || change === "typing" || change === "seen") && conversation === this.open) {
      this.drawThread();
    }
  }

  /*
   * drawHeader redraws who the visitor talks to once the team's
   * availability is known.
   */
  private drawHeader() {
    const markup = String(Whoami({ content: this.content() }));
    for (const whoami of this.host.root.querySelectorAll<HTMLElement>('[data-target="whoami"]')) {
      whoami.outerHTML = markup;
    }
  }

  /*
   * drawConversations redraws the visitor's list of conversations.
   */
  private drawConversations() {
    const view = this.host.root.querySelector<HTMLElement>('.view[data-view="messages"] .loaded');
    if (!view) {
      return;
    }
    const list = this.content().conversations.filter(
      (c) => !this.openOnly || (c.ticket && c.ticket.state.group !== "resolved"),
    );
    const rows = view.querySelector<HTMLElement>(".list");
    const empty = view.querySelector<HTMLElement>(".empty");
    if (list.length === 0) {
      rows?.remove();
      if (empty) {
        empty.hidden = false;
      }
      return;
    }
    if (empty) {
      empty.hidden = true;
    }
    const markup = list.map((c) => String(ConversationRow({ conversation: c }))).join("");
    if (rows) {
      rows.innerHTML = markup;
    } else {
      view.insertAdjacentHTML("afterbegin", `<div class="list">${markup}</div>`);
    }
    this.host.root.dispatchEvent(new CustomEvent("dimmi:connect"));
  }

  /*
   * drawThread redraws the open conversation: where it stands, its
   * messages, who is typing, Seen once the team read the latest, that the
   * team is away, and a field for an email when the team needs one to
   * reply.
   */
  private drawThread() {
    const thread = this.host.root.querySelector<HTMLElement>(".thread");
    const loaded = thread?.querySelector<HTMLElement>(".loaded");
    if (!thread || !loaded) {
      return;
    }

    const t = threadParts();
    if (!t) {
      void loadThread(this.host.root).then(() => this.drawThread());
      return;
    }
    const texts = this.host.options().texts;
    const open = this.open;
    const messages = this.content().thread.map((m) => this.withDecision(m));
    const typing = open ? this.store.typingIn(open) : undefined;
    const last = messages.at(-1);
    const parts = [this.statusLine(), String(t.Thread({ messages, texts: this.actionTexts() }))];
    const ticket = this.store.conversations.find((c) => c.id === open)?.ticket;
    const strip = tickets()?.Strip;
    if (ticket && strip) {
      parts.unshift(String(strip({ ticket, texts })));
    } else if (ticket) {
      void loadTickets(this.host.root).then(() => this.drawThread());
    }

    if (typing) {
      parts.push(String(t.Typing()));
    } else {
      parts.push(this.aiActions(last, t));
    }
    parts.push(this.delivery(open));
    if (last?.from === "visitor" && this.config.delay_notice !== false && this.availability?.online === false) {
      parts.push(`<p class="delay">${escapeHtml(texts.delay_notice)}</p>`);
    }
    parts.push(this.ratingCard());
    if (open && (this.store.messages.get(open) ?? []).some((m) => m.failed === "email_required")) {
      parts.push(
        `<form class="capture" data-capture><p>${escapeHtml(texts.email_prompt)}</p><span class="row">` +
          '<label class="field"><input type="email" name="email" required autocomplete="email"></label>' +
          `<button class="button primary" type="submit">${escapeHtml(texts.send_message)}</button></span></form>`,
      );
    }

    this.lockComposer(choicesOnly(last));
    place({ thread, loaded, open: open ?? "", shown: messages.length }, parts.join(""));
    this.host.root.dispatchEvent(new CustomEvent("dimmi:connect"));
    this.watchCalls(messages);
  }

  /*
   * lockComposer puts the composer away while a question takes its
   * buttons alone, and brings it back once one is tapped.
   */
  private lockComposer(locked: boolean) {
    const panel = this.host.root.firstElementChild;
    if (panel) {
      panel.toggleAttribute("data-choices-only", locked);
    }
  }

  /*
   * aiActions offers the visitor the actions under the agent's latest
   * reply while the agent still holds the conversation: That helped only
   * under an answer, Talk to a person under any reply, and nothing once
   * the conversation is with the team.
   */
  private aiActions(last: { from: string; id?: string; outcome?: string } | undefined, t: Parts): string {
    const found = this.store.conversations.find((c) => c.id === this.open);
    if (last?.from !== "agent" || !last.id || found?.ai_state !== "active") {
      return "";
    }
    const texts = this.host.options().texts;

    return String(
      t.AiActions({
        texts: { helped: texts.ai_helped, handoff: texts.talk_to_human, thanks: texts.ai_thanks },
        helped: this.helped.has(last.id),
        answered: last.outcome === "answered" || last.outcome === "partial",
      }),
    );
  }

  /*
   * delivery says where the visitor's latest message stands, under it:
   * sending, not delivered with a way to send it again, seen once a
   * teammate read it, or else sent. A conversation the team spoke in last
   * needs no line. One refused for want of an email asks for the email
   * instead.
   */
  private delivery(open: string | undefined): string {
    const mine = open ? (this.store.messages.get(open) ?? []) : [];
    const last = mine.at(-1);
    if (last?.author.type !== "contact" || last.failed === "email_required") {
      return "";
    }
    const t = threadParts();
    const texts = this.host.options().texts as unknown as Record<string, string>;
    const seen = open ? this.store.seen.get(open) : undefined;
    const state = last.sending
      ? "sending"
      : last.failed
        ? "failed"
        : seen && Date.parse(seen.at) >= Date.parse(last.created_at)
          ? "seen"
          : "sent";

    return String(t?.Delivery({ state, texts, retry: last.client_id ?? undefined }) ?? "");
  }

  /*
   * retry sends again a message of the visitor's the server refused.
   */
  retry(clientId: string) {
    const open = this.open;
    const failed = open ? this.store.dropLocal(open, clientId) : undefined;
    if (failed?.failed) {
      void this.send(failed.body ?? "", []);
    }
  }

  /*
   * aiAction tells the server the agent's answer helped, or hands the
   * conversation to the team. The thread changes at once; a refusal puts
   * it back.
   */
  async aiAction(kind: "helped" | "handoff") {
    const open = this.open;
    const found = this.store.conversations.find((c) => c.id === open);
    const last = this.content().thread.at(-1);
    if (!open || open === draftId || !found || !last?.id) {
      return;
    }
    const answer = last.id;
    if (kind === "helped") {
      this.helped.add(answer);
      this.drawThread();
    } else {
      this.store.upsertConversation({ ...found, ai_state: "handed_off" });
      this.drawThread();
    }

    try {
      await this.http.post(`/conversations/${open}/ai/${kind}`);
    } catch (error) {
      const code = error instanceof ApiError ? error.code : "network";
      report(
        "warn",
        code,
        kind === "helped" ? "the answer could not be marked helpful." : "the team could not be asked.",
      );
      this.helped.delete(answer);
      if (kind === "handoff") {
        this.store.upsertConversation(found);
      }
      this.drawThread();
    }
  }

  /*
   * withDecision shows an action card whose choice is on its way as
   * sending.
   */
  private withDecision(message: Message): Message {
    const action = message.action;
    if (action?.state === "verify") {
      const email = this.session.current?.contact.email ?? "";
      return { ...message, action: { ...action, verify: this.verifying.get(action.id) ?? { step: "email", email } } };
    }
    if (!action || !this.deciding.has(action.id)) {
      return message;
    }

    return { ...message, action: { ...action, state: "sending" } };
  }

  /*
   * verifyEmail takes one step of confirming the visitor's email for an
   * action: sending a code to the address, or checking the code typed.
   * Once the code checks, the server moves the action on and the card
   * shows where it stands. The code is posted from the card's form only,
   * never as a message.
   */
  async verifyEmail(call: string, step: "email" | "code", value: string) {
    const open = this.open;
    const current = this.verifying.get(call) ?? { step: "email", email: "" };
    if (!open || current.busy) {
      return;
    }
    this.verifying.set(call, { ...current, busy: true, error: undefined });
    this.drawThread();
    try {
      if (step === "email") {
        const res = await this.http.post<{ data: { email: string } }>("/email-verifications", { email: value.trim() });
        this.verifying.set(call, { step: "code", email: value.trim(), masked: res.data.email });
      } else {
        await this.http.post("/email-verifications/checks", { code: value.replace(/\D/g, "") });
        this.verifying.delete(call);
        await this.session.resume();
        this.takeSession(this.session.current);
        await this.refreshCall(open, call);
      }
    } catch (error) {
      const message = error instanceof ApiError ? error.message : "Something went wrong. Try again.";
      const restart = error instanceof ApiError && ["email_code_expired", "email_code_locked"].includes(error.code);
      this.verifying.set(call, { ...current, ...(restart ? { step: "email" } : {}), busy: false, error: message });
    }
    this.drawThread();
  }

  /*
   * verifyAgain sends a new code to the same address, or goes back to
   * typing another one.
   */
  verifyAgain(call: string, how: "resend" | "other") {
    const current = this.verifying.get(call);
    if (!current) {
      return;
    }
    if (how === "resend") {
      void this.verifyEmail(call, "email", current.email);
      return;
    }
    this.verifying.set(call, { step: "email", email: current.email });
    this.drawThread();
  }

  private actionTexts(): ActionTexts {
    const t = this.host.options().texts;

    return {
      confirm: t.action_confirm,
      cancel: t.action_cancel,
      verifyIntro: t.verify_intro,
      verifyEmail: t.verify_email,
      verifySend: t.verify_send,
      verifySent: t.verify_sent,
      verifyCode: t.verify_code,
      verifySubmit: t.verify_submit,
      verifyResend: t.verify_resend,
      verifyOther: t.verify_other,
      waiting: t.action_waiting,
      done: t.action_done,
      declined: t.action_declined,
      canceled: t.action_canceled,
      expired: t.action_expired,
      failed: t.action_failed,
    };
  }

  /*
   * decideAction sends the visitor's Confirm or Cancel on an action card.
   * The card waits with a spinner until the server answers, then shows
   * where the call stands: confirmed, waiting for the team, or canceled.
   * A refusal, such as an expired card, puts the server's state in place.
   */
  async decideAction(call: string, decision: "confirmed" | "canceled") {
    const open = this.open;
    if (!open || open === draftId || this.deciding.has(call)) {
      return;
    }
    this.deciding.add(call);
    this.drawThread();
    try {
      const res = await this.http.patch<{ data: WidgetCall }>(`/conversations/${open}/action-calls/${call}`, {
        status: decision,
      });
      this.store.setCallStatus(open, call, res.data.state ?? res.data.status);
    } catch (error) {
      const code = error instanceof ApiError ? error.code : "network";
      report("warn", code, "the choice could not be sent.");
      if (code === "action_call_expired") {
        this.store.setCallStatus(open, call, "expired");
      } else if (code === "action_call_not_pending") {
        await this.refreshCall(open, call);
      }
    } finally {
      this.deciding.delete(call);
      this.drawThread();
    }
  }

  /*
   * refreshCall reads where one call stands and keeps it on its card.
   */
  private async refreshCall(conversation: string, call: string) {
    try {
      const res = await this.http.get<{ data: WidgetCall }>(`/conversations/${conversation}/action-calls/${call}`);
      this.store.setCallStatus(conversation, call, res.data.state ?? res.data.status);
    } catch {
      return;
    }
  }

  /*
   * watchCalls checks the open conversation's waiting calls again every
   * few seconds while the visitor can see them, since the team's decision
   * is not pushed yet. It stops once every call has ended or the
   * conversation is out of sight, and starts again on the next redraw.
   */
  private watchCalls(messages: Message[]) {
    const open = this.open;
    const waiting = messages
      .map((m) => m.action)
      .filter(
        (a): a is NonNullable<Message["action"]> =>
          a !== undefined && (a.state === "ask" || a.state === "waiting" || a.state === "verify"),
      )
      .map((a) => a.id);
    if (this.callTimer !== undefined || !open || open === draftId || waiting.length === 0) {
      return;
    }
    this.callTimer = this.browser.setTimeout(async () => {
      this.callTimer = undefined;
      if (this.open !== open || !this.host.showing(open)) {
        return;
      }
      await Promise.all(waiting.map((call) => this.refreshCall(open, call)));
      this.drawThread();
    }, callCheck);
  }

  /*
   * ratingCard asks the visitor to rate the open conversation once it
   * closed and its ask went out. The card's chunk loads the first time
   * a closed conversation opens.
   */
  private ratingCard(): string {
    const open = this.open;
    const found = this.store.conversations.find((c) => c.id === open);
    if (!open || found?.status !== "closed") {
      return "";
    }
    if (!this.ratings) {
      if (!this.ratingsLoading) {
        this.ratingsLoading = true;
        void import("./rating").then(({ Ratings }) => {
          this.ratings = new Ratings(this.http, this.host.root, (c) => this.open === c && this.drawThread());
          this.drawThread();
        });
      }
      return "";
    }
    const t = this.host.options().texts;

    return this.ratings.card(open, {
      ask: t.rating_ask,
      comment: t.rating_comment,
      send: t.rating_send,
      thanks: t.rated_thanks,
    });
  }

  /*
   * rateConversation sends the face or the comment the visitor gave on
   * the open conversation's rating card.
   */
  rateConversation(answer: { score?: number; comment?: string }) {
    if (this.open) {
      void this.ratings?.rate(this.open, answer);
    }
  }

  /*
   * statusLine says where the open conversation stands, when the widget
   * shows it.
   */
  private statusLine(): string {
    const found = this.store.conversations.find((c) => c.id === this.open);
    if (!found || this.config.customer_status === false) {
      return "";
    }
    const texts = this.host.options().texts as unknown as Record<string, string>;
    const label = texts[`status_${found.customer_status}`];

    return label ? `<p class="status">${escapeHtml(label)}</p>` : "";
  }

  /*
   * capture keeps the email the visitor gave and sends again what waited
   * for it.
   */
  async capture(email: string) {
    try {
      await this.http.patch("/contact", { email });
    } catch (error) {
      const code = error instanceof ApiError ? error.code : "network";
      report("warn", code, "the email could not be saved.");
      return;
    }

    const waiting = this.store.dropFailed(this.open ?? draftId, "email_required");
    for (const message of waiting) {
      void this.send(message.body ?? "", []);
    }
  }
}

/*
 * choicesOnly reports whether the thread ends on a question that takes
 * its buttons alone.
 */
function choicesOnly(last: Message | undefined): boolean {
  return last?.choicesOnly === true && (last.choices ?? []).length > 0;
}

/*
 * escape makes text safe to place in markup.
 */
function escapeHtml(text: string): string {
  return text.replace(/[&<>"']/g, (c) => `&#${c.charCodeAt(0)};`);
}
