/*
 * The messenger connected to its server. It loads the widget's config and
 * the team's availability first, and only when the visitor already has a
 * session does it load their conversations and open the stream; a first
 * visit costs the config and nothing else until the visitor writes.
 *
 * The views ask through events: "dimmi:open" to show a conversation, or a
 * new one when none is named, and "dimmi:send" to send what the composer
 * holds. Answers land in the store, and only the parts of the panel that
 * changed are drawn again, so a half-written message, the scroll position
 * and the open tab survive every update.
 */

import { ApiError, Http } from "../client/http";
import { Outbox, type Pending, type Sent } from "../client/outbox";
import { Realtime } from "../client/realtime";
import { Session, type Storage } from "../client/session";
import { type Conversation as ApiConversation, type Message as ApiMessage, Store } from "../client/store";
import { ConversationRow } from "../components/rows";
import { Thread, Typing } from "../components/thread";
import { Whoami } from "../components/whoami";
import type { Content } from "../content";
import type { Options, OptionsUpdate } from "../options";
import { type Availability, type Config, content, fromConfig } from "./map";
import { patch } from "./patch";
import { report } from "./report";

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
  setInterval: (fn: () => void, ms: number) => unknown;
}

/*
 * routeCheck is how often the page's address is compared with the last
 * one, to notice a single-page app moving to another route.
 */
const routeCheck = 1000;

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
   * must stay hidden on this page.
   */
  async start(): Promise<boolean> {
    try {
      this.config = (await this.http.get<{ data: Config }>(`/apps/${this.appId}/config`)).data;
    } catch (error) {
      const code = error instanceof ApiError ? error.code : "network";
      report("error", code, fatal[code] ?? "the messenger's config could not be loaded.");
      return false;
    }

    this.host.update(fromConfig(this.config, this.host.options().texts));
    void this.loadAvailability();

    const session = await this.session.resume().catch(() => undefined);
    if (session) {
      await this.reload();
      this.streaming();
    }
    if (this.outbox.pending.length > 0) {
      void this.outbox.flush();
    }
    this.watchRoutes();

    return true;
  }

  /*
   * watchRoutes tells the server where the visitor is once a page that
   * never reloads, such as a single-page app, moves to another address.
   */
  private watchRoutes() {
    let last = this.browser.page().url;
    this.browser.setInterval(() => {
      const page = this.browser.page();
      if (page.url === last) {
        return;
      }
      last = page.url;
      if (this.session.current) {
        void this.http.post("/pings", { page }).catch(() => undefined);
      }
    }, routeCheck);
  }

  /*
   * content is what the views draw now.
   */
  content(): Content {
    return content(this.base, this.config, this.availability, this.store, this.open, this.browser.now());
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
   * it at once.
   */
  async send(body: string, files: File[]) {
    const conversation = this.open && this.open !== draftId ? this.open : undefined;
    let attachmentIds: string[] = [];
    if (files.length > 0) {
      attachmentIds = await this.upload(files);
    }

    const pending = this.outbox.send({ conversation, body, attachmentIds });
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

  private async upload(files: File[]): Promise<string[]> {
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

  private async loadAvailability() {
    try {
      this.availability = (await this.http.get<{ data: Availability }>(`/apps/${this.appId}/availability`)).data;
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
    }
  }

  private event(event: string, data: Record<string, unknown>) {
    this.store.apply(event, data);
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

  private ended() {
    this.session.forget();
    this.store.setConversations([]);
    this.store.messages.clear();
    this.open = undefined;
    this.drawThread();
  }

  private startDetails() {
    const page = this.browser.page();
    return {
      page,
      locale: typeof navigator === "undefined" ? undefined : navigator.language,
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    };
  }

  private changed(change: string, conversation?: string) {
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
    const list = this.content().conversations;
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

    const texts = this.host.options().texts;
    const open = this.open;
    const messages = this.content().thread;
    const typing = open ? this.store.typingIn(open) : undefined;
    const seen = open ? this.store.seen.get(open) : undefined;
    const last = messages.at(-1);
    const parts = [this.statusLine(), String(Thread({ messages }))];

    if (typing) {
      parts.push(String(Typing()));
    }
    if (seen && last?.from === "visitor" && !last.state) {
      parts.push(`<p class="seen">${escapeHtml(texts.seen)}</p>`);
    }
    if (last?.from === "visitor" && this.config.delay_notice !== false && this.availability?.online === false) {
      parts.push(`<p class="delay">${escapeHtml(texts.delay_notice)}</p>`);
    }
    if (open && (this.store.messages.get(open) ?? []).some((m) => m.failed === "email_required")) {
      parts.push(
        `<form class="capture" data-capture><p>${escapeHtml(texts.email_prompt)}</p><span class="row">` +
          '<label class="field"><input type="email" name="email" required autocomplete="email"></label>' +
          `<button class="button primary" type="submit">${escapeHtml(texts.send_message)}</button></span></form>`,
      );
    }

    place({ thread, loaded, open: open ?? "", shown: messages.length }, parts.join(""));
    this.host.root.dispatchEvent(new CustomEvent("dimmi:connect"));
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
 * escape makes text safe to place in markup.
 */
function escapeHtml(text: string): string {
  return text.replace(/[&<>"']/g, (c) => `&#${c.charCodeAt(0)};`);
}
