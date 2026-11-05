/*
 * The messenger's frame: the launcher, the panel it opens and the views
 * inside it. Open and closed are one data attribute the stylesheet
 * animates; the panel stays in the page but inert while closed, so it can
 * fade out instead of vanishing, and nothing inside it takes focus or
 * clicks. Opening plays the sound when the customer left it on; restore
 * reopens it silently after the options re-render it.
 *
 * Exactly one view is current. The tab bar switches between the tabs;
 * pages opened from them (a collection, an article, a changelog entry, a
 * conversation) stack on top and cover the tab bar, and back walks the
 * stack down again. Pages marked for it expand the panel when they open
 * and shrink it when the visitor leaves them, unless the visitor expanded
 * it by hand. Escape closes the panel and hands focus back to the
 * launcher.
 *
 * A view whose content comes from the server loads the first time it is
 * seen with the panel open: it shows its skeleton and announces that it is
 * busy until the content arrives, then reveals it. A connected messenger
 * draws its content as answers come, so it waits for none; a preview waits
 * the latency on the messenger's data-latency attribute, zero meaning at
 * once. Opening a conversation tells the connection which one, through a
 * "dimmi:converse" event naming it, or none for a new one.
 *
 * A message from the team lands in its thread straight away. Unless the
 * visitor is reading that conversation, it counts as unread: a badge on
 * the launcher and on the Messages tab, the conversation's row in bold,
 * and, while the messenger is closed, a preview card above the launcher
 * that opens the conversation when clicked. Opening the conversation reads
 * everything; dismissing the preview leaves the message unread.
 */
import { Notice } from "../components/notice";
import { Said } from "../components/thread";
import type { Incoming } from "../content";
import { Controller } from "../lib/controller";
import { pop } from "../lib/sound";

export class MessengerController extends Controller {
  #stack: string[] = [];
  #autoExpanded = false;
  #viewerExpanded = false;
  #alerted: string | undefined;

  get isOpen(): boolean {
    return this.element.dataset.open === "true";
  }

  get tab(): string {
    return this.element.dataset.tab ?? "";
  }

  get expanded(): boolean {
    return this.element.dataset.expanded === "true";
  }

  open(): void {
    this.#toggle(true);
  }

  close(): void {
    this.#toggle(false);
  }

  toggle(): void {
    this.#toggle(!this.isOpen);
  }

  restore(view: string): void {
    this.#toggle(true, false);
    this.show(view);
  }

  select(event: Event): void {
    this.#stack = [];
    this.show((event.target as Element).closest<HTMLElement>("[data-tab]")?.dataset.tab ?? "");
  }

  visit(event: Event): void {
    this.#push((event.target as Element).closest<HTMLElement>("[data-open]")?.dataset.open ?? "");
  }

  receive(message: Incoming): void {
    const list = this.element.querySelector(".thread .loaded") ?? this.element.querySelector(".thread");
    list?.querySelector(".seen")?.remove();
    list?.insertAdjacentHTML("beforeend", String(Said({ messages: [message] })));
    this.#preview(message);

    if (this.isOpen && this.tab === "conversation") {
      return;
    }

    this.alert(message);
  }

  /*
   * alert counts a message from the team as unread, shows its preview card
   * while the messenger is closed and plays the sound, without touching a
   * thread the connection draws itself.
   */
  alert(message: Incoming): void {
    this.#alerted = String(message.conversation);
    this.#unread(this.#count + 1);
    const notice = this.target("notice");
    if (notice && !this.isOpen) {
      notice.innerHTML = String(Notice({ message }));
      notice.hidden = false;
    }
    if (this.element.dataset.sound === "true") {
      pop();
    }
  }

  read(): void {
    this.#toggle(true);
    this.#converse(this.#alerted);
  }

  ignore(): void {
    const notice = this.target("notice");
    if (notice) {
      notice.hidden = true;
    }
  }

  converse(event?: Event): void {
    const row = event?.target instanceof Element ? event.target.closest<HTMLElement>("[data-conversation]") : null;
    this.#converse(row?.dataset.conversation);
  }

  /*
   * converse opens the conversation, a new one when the id names none the
   * server knows.
   */
  #converse(id: string | undefined): void {
    this.#unread(0);
    this.ignore();
    this.#push("conversation");
    this.dispatch("converse", { conversation: id?.startsWith("conv_") ? id : undefined });
    this.#latest();
  }

  back(): void {
    this.show(this.#stack.pop() ?? this.element.querySelector<HTMLElement>("[role=tab]")?.dataset.tab ?? "home");
  }

  show(name: string): void {
    const view = this.#view(name);
    if (!view) {
      return;
    }

    this.element.dataset.tab = name;
    this.element.dataset.page = String(!view.hasAttribute("role"));
    for (const each of this.element.querySelectorAll<HTMLElement>(".view")) {
      each.toggleAttribute("data-current", each === view);
    }
    for (const button of this.element.querySelectorAll<HTMLElement>("[role=tab]")) {
      button.setAttribute("aria-selected", String(button.dataset.tab === name));
    }
    view.scrollTop = 0;
    this.#fit(view.hasAttribute("data-expand"));
    this.#load(view);
  }

  expand(): void {
    this.#autoExpanded = false;
    this.#expand(!this.expanded);
  }

  /*
   * view opens a clicked picture across the messenger, expanding the panel
   * first so it has room, and remembers whether it did so it can shrink
   * the panel again on the way out.
   */
  view(event: Event): void {
    const link = (event.target as Element | null)?.closest<HTMLAnchorElement>("a[href]");
    const viewer = this.target("viewer");
    const image = this.target<HTMLImageElement>("viewer-image");
    if (!link || !viewer || !image) {
      return;
    }
    event.preventDefault();
    this.#viewerExpanded = !this.expanded;
    if (this.#viewerExpanded) {
      this.#expand(true);
    }
    image.src = link.href;
    image.alt = link.querySelector("img")?.alt ?? "";
    viewer.hidden = false;
    requestAnimationFrame(() => {
      viewer.dataset.shown = "true";
    });
    viewer.querySelector<HTMLElement>('[data-action="click->unview"]')?.focus();
  }

  /*
   * original opens the picture being viewed in a tab of its own.
   */
  original(): void {
    const image = this.target<HTMLImageElement>("viewer-image");
    if (image?.src) {
      window.open(image.src, "_blank", "noopener");
    }
  }

  /*
   * unview closes the picture and gives the panel back its size.
   */
  unview(event?: Event): void {
    const viewer = this.target("viewer");
    if (!viewer || viewer.hidden) {
      return;
    }
    if (event && event.target !== event.currentTarget && (event.target as Element).closest("img")) {
      return;
    }
    viewer.dataset.shown = "false";
    viewer.hidden = true;
    if (this.#viewerExpanded) {
      this.#expand(false);
      this.#viewerExpanded = false;
    }
  }

  dismiss(event: KeyboardEvent): void {
    if (event.key === "Escape" && this.target("viewer")?.hidden === false) {
      this.unview();
      return;
    }
    if (event.key === "Escape" && this.isOpen) {
      this.close();
      this.target("launcher")?.focus();
    }
  }

  /*
   * latest jumps the thread to its newest message at once, without the
   * smooth scrolling new messages get while the visitor reads.
   */
  #latest(): void {
    const thread = this.element.querySelector<HTMLElement>(".thread");
    if (!thread) {
      return;
    }

    thread.style.scrollBehavior = "auto";
    thread.scrollTop = thread.scrollHeight;
    thread.style.removeProperty("scroll-behavior");
  }

  get #count(): number {
    return Number(this.element.dataset.unread) || 0;
  }

  /*
   * unread shows the number of unread messages on the launcher and the
   * Messages tab, and marks the conversation's rows while there are any.
   */
  #unread(count: number): void {
    this.element.dataset.unread = String(count);
    for (const badge of [this.target("count"), this.target("tab-count")]) {
      if (badge) {
        badge.textContent = String(count);
        badge.hidden = count === 0;
      }
    }
    if (count === 0) {
      for (const row of this.element.querySelectorAll<HTMLElement>('[data-unread="true"]')) {
        row.dataset.unread = "false";
      }
    }
    this.target("launcher")?.setAttribute("aria-label", count ? `Open messages, ${count} unread` : "Open messages");
  }

  /*
   * preview puts the new message on its conversation's rows, marked unread.
   */
  #preview(message: Incoming): void {
    for (const row of this.element.querySelectorAll<HTMLElement>(`[data-conversation="${message.conversation}"]`)) {
      const preview = row.querySelector("small");
      if (preview) {
        preview.textContent = message.body ?? "";
      }
      row.querySelector("time")?.replaceChildren(message.at);
      row.dataset.unread = String(!(this.isOpen && this.tab === "conversation"));
    }
  }

  #load(view: HTMLElement): void {
    if (!(view.hasAttribute("data-load") && this.isOpen) || view.dataset.state) {
      return;
    }

    view.dataset.state = "loading";
    view.setAttribute("aria-busy", "true");
    const arrive = () => {
      view.dataset.state = "ready";
      view.removeAttribute("aria-busy");
      if (view.dataset.view === "conversation") {
        this.#latest();
      }
    };
    const latency = Number(this.element.dataset.latency) || 0;
    if (latency > 0) {
      setTimeout(arrive, latency);
    } else {
      arrive();
    }
  }

  #view(name: string): HTMLElement | null {
    if (!name) {
      return null;
    }

    for (const view of this.element.querySelectorAll<HTMLElement>(".view")) {
      if (view.dataset.view === name) {
        return view;
      }
    }

    return null;
  }

  #push(name: string): void {
    if (!this.#view(name) || name === this.tab) {
      return;
    }

    this.#stack.push(this.tab);
    this.show(name);
  }

  /*
   * fit expands the panel for a page that wants room and shrinks it again
   * on the way out, but only when it was the page that expanded it.
   */
  #fit(wantsRoom: boolean): void {
    if (wantsRoom && !this.expanded) {
      this.#autoExpanded = true;
      this.#expand(true);
    } else if (!wantsRoom && this.#autoExpanded) {
      this.#autoExpanded = false;
      this.#expand(false);
    }
  }

  #expand(expanded: boolean): void {
    this.element.dataset.expanded = String(expanded);
    for (const button of this.element.querySelectorAll('[data-action="click->expand"]')) {
      button.setAttribute("aria-label", expanded ? "Shrink messages" : "Expand messages");
    }
  }

  #toggle(open: boolean, sound = true): void {
    if (open === this.isOpen) {
      return;
    }

    this.element.dataset.open = String(open);
    this.target("launcher")?.setAttribute("aria-expanded", String(open));
    this.target("launcher")?.setAttribute("aria-label", open ? "Close messages" : "Open messages");
    this.target("panel")?.toggleAttribute("inert", !open);
    if (open) {
      this.ignore();
    } else {
      this.#unread(this.#count);
    }
    const current = this.#view(this.tab);
    if (open && current) {
      this.#load(current);
    }
    if (open && sound && this.element.dataset.sound === "true") {
      pop();
    }
    this.dispatch(open ? "open" : "close");
  }
}
