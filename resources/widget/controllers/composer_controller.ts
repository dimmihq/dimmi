/*
 * The message composer at the bottom of a conversation. The text box grows
 * with what is typed up to a few lines, Enter sends and Shift+Enter breaks
 * the line, and send stays disabled until there is something to send.
 * Attached files wait as chips above the text and go out as file bubbles.
 * The microphone switches the composer to a recording bar with a running
 * timer; finishing sends a voice note, the bin throws it away.
 *
 * Nothing reaches a server yet: sent messages land in the thread, and a
 * typing indicator followed by a placeholder reply shows how the other
 * side will answer once the backend is wired.
 */

import { Chip } from "../components/composer";
import { Said, Typing } from "../components/thread";
import type { Message } from "../content";
import { Controller } from "../lib/controller";

const reply = "Thanks for the details. A teammate will pick this up here in a moment.";

function size(bytes: number): string {
  return bytes < 1024 * 1024 ? `${Math.max(1, Math.round(bytes / 1024))} KB` : `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

function clock(seconds: number): string {
  return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, "0")}`;
}

function now(): string {
  return new Date().toTimeString().slice(0, 5);
}

export class ComposerController extends Controller {
  #files: File[] = [];
  #seconds = 0;
  #timer: ReturnType<typeof setInterval> | undefined;

  connect(): void {
    for (const bar of this.element.querySelectorAll<HTMLElement>(".live i")) {
      bar.style.height = `${bar.dataset.height ?? 8}px`;
    }
  }

  disconnect(): void {
    clearInterval(this.#timer);
  }

  get #form(): HTMLFormElement {
    return this.element as HTMLFormElement;
  }

  get #input(): HTMLTextAreaElement {
    return this.target("input") as HTMLTextAreaElement;
  }

  resize(): void {
    const input = this.#input;
    input.style.height = "auto";
    input.style.height = `${Math.min(input.scrollHeight, 120)}px`;
    this.#ready();
  }

  enter(event: KeyboardEvent): void {
    if (event.key === "Enter" && !event.shiftKey && !event.isComposing) {
      event.preventDefault();
      this.#form.requestSubmit();
    }
  }

  attach(event: Event): void {
    const picker = event.target as HTMLInputElement;
    this.#files.push(...Array.from(picker.files ?? []));
    picker.value = "";
    this.#chips();
  }

  unattach(event: Event): void {
    const index = Number((event.target as Element).closest<HTMLElement>("[data-index]")?.dataset.index);
    this.#files.splice(index, 1);
    this.#chips();
  }

  send(event: SubmitEvent): void {
    event.preventDefault();
    const body = this.#input.value.trim();
    const messages: Message[] = this.#files.map((file) => ({
      from: "visitor",
      attachment: { kind: file.type.startsWith("image/") ? "image" : "file", name: file.name, size: size(file.size) },
      at: now(),
    }));
    if (body) {
      messages.unshift({ from: "visitor", body, at: now() });
    }
    if (!messages.length) {
      return;
    }

    this.#input.value = "";
    this.#files = [];
    this.#chips();
    this.resize();
    this.#post(messages);
  }

  record(): void {
    this.element.dataset.recording = "true";
    this.#seconds = 0;
    this.#tick();
    this.#timer = setInterval(() => {
      this.#seconds += 1;
      this.#tick();
    }, 1000);
  }

  discard(): void {
    clearInterval(this.#timer);
    delete this.element.dataset.recording;
  }

  finish(): void {
    const duration = clock(Math.max(this.#seconds, 1));
    this.discard();
    this.#post([{ from: "visitor", attachment: { kind: "voice", name: "Voice note", duration }, at: now() }]);
  }

  #tick(): void {
    const timer = this.target("timer");
    if (timer) {
      timer.textContent = clock(this.#seconds);
    }
  }

  #ready(): void {
    this.target("send")?.toggleAttribute("disabled", !this.#input.value.trim() && !this.#files.length);
  }

  #chips(): void {
    const chips = this.target("chips");
    if (chips) {
      chips.innerHTML = this.#files.map((file, index) => String(Chip({ name: file.name, index }))).join("");
    }
    this.#ready();
  }

  /*
   * post adds the visitor's messages to the thread, then shows the other
   * side typing and their placeholder reply.
   */
  #post(messages: Message[]): void {
    const thread = this.element.parentElement?.querySelector<HTMLElement>(".thread");
    const list = thread?.querySelector<HTMLElement>(".loaded") ?? thread;
    if (!thread || !list) {
      return;
    }

    list.querySelector(".seen")?.remove();
    list.insertAdjacentHTML("beforeend", String(Said({ messages })) + String(Typing()));
    this.#scroll(thread);
    setTimeout(() => {
      list.querySelector("[data-typing]")?.remove();
      const answer: Message = { from: "team", author: "Maya Kerr", body: reply, at: now() };
      list.insertAdjacentHTML("beforeend", String(Said({ messages: [answer] })));
      this.dispatch("connect", thread);
      this.#scroll(thread);
    }, 1600);
    this.dispatch("connect", thread);
  }

  #scroll(thread: HTMLElement): void {
    thread.scrollTop = thread.scrollHeight;
  }
}
