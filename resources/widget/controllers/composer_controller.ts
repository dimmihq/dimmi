/*
 * The message composer at the bottom of a conversation. The text box grows
 * with what is typed up to a few lines, Enter sends and Shift+Enter breaks
 * the line, and send stays disabled until there is something to send.
 * Attached files wait as chips above the text and go out as file bubbles.
 * The microphone switches the composer to a recording bar with a running
 * timer; finishing sends a voice note, the bin throws it away.
 *
 * Sending hands the text, files or voice note on through a "dimmi:send"
 * event: a connected messenger shows the message at once and sends it, a
 * preview shows it and answers with a placeholder reply. Typing tells a
 * connected messenger at most every few seconds through "dimmi:typing".
 */

import { Chip } from "../components/composer";
import { Controller } from "../lib/controller";

/*
 * typingEvery is the shortest time between two typing notices.
 */
const typingEvery = 2000;

function clock(seconds: number): string {
  return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, "0")}`;
}

export class ComposerController extends Controller {
  #files: File[] = [];
  #seconds = 0;
  #typedAt = 0;
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
    if (this.#live && input.value.trim() && Date.now() - this.#typedAt > typingEvery) {
      this.#typedAt = Date.now();
      this.dispatch("typing", { started: true });
    }
  }

  get #live(): boolean {
    return this.element.closest<HTMLElement>(".messenger")?.dataset.live === "true";
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
    if (!body && !this.#files.length) {
      return;
    }

    this.dispatch("send", { body, files: this.#files });
    this.#typedAt = 0;
    this.#input.value = "";
    this.#files = [];
    this.#chips();
    this.resize();
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
    this.dispatch("send", { body: "", files: [], voice: duration });
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
}
