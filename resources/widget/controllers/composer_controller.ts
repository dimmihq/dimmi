/*
 * The message composer at the bottom of a conversation. The text box grows
 * with what is typed up to a few lines, Enter sends and Shift+Enter breaks
 * the line, and send stays disabled until there is something to send.
 * Attached files wait as chips above the text and go out as file bubbles.
 * The microphone switches the composer to a recording bar with a running
 * timer whose bars follow the visitor's voice; finishing sends the voice
 * note as an audio file, the bin throws it away. A connected messenger
 * records from the real microphone and says so when it is refused; a
 * preview only plays the motions. Where the page cannot record, the
 * microphone button is not offered. The smiley opens a small emoji
 * picker above the text box; a picked emoji lands where the cursor was,
 * and Escape or the smiley closes the picker again.
 *
 * Sending hands the text, files or voice note on through a "dimmi:send"
 * event: a connected messenger shows the message at once and sends it, a
 * preview shows it and answers with a placeholder reply. Typing tells a
 * connected messenger at most every few seconds through "dimmi:typing".
 */

import { Chip } from "../components/composer";
import { Controller } from "../lib/controller";
import { canRecord, maxSeconds } from "../lib/microphone";
import type { Recorder } from "../lib/recorder";

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
  #meter: ReturnType<typeof setInterval> | undefined;
  #recorder: Recorder | undefined;
  #emoji: typeof import("../lib/emoji") | undefined;

  connect(): void {
    for (const bar of this.element.querySelectorAll<HTMLElement>(".live i")) {
      bar.style.height = `${bar.dataset.height ?? 8}px`;
    }
    if (this.#live && !canRecord()) {
      this.element.querySelector<HTMLElement>('[data-action="click->record"]')?.setAttribute("hidden", "");
    }
  }

  disconnect(): void {
    clearInterval(this.#timer);
    clearInterval(this.#meter);
    this.#recorder?.cancel();
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

  async emoji(): Promise<void> {
    const picker = this.target("emoji");
    if (!picker) {
      return;
    }
    this.#emoji ??= await import("../lib/emoji");
    if (!picker.firstChild) {
      picker.innerHTML = this.#emoji.buttons();
    }
    this.#picker(Boolean(picker.hidden));
  }

  pick(event: Event): void {
    const value = (event.target as Element).closest<HTMLElement>("[data-emoji]")?.dataset.emoji;
    if (value && this.#emoji) {
      this.#picker(false);
      this.#emoji.insert(this.#input, value);
      this.resize();
    }
  }

  unpick(event: KeyboardEvent): void {
    if (event.key === "Escape") {
      event.preventDefault();
      this.#picker(false);
      this.target("emoji-button")?.focus();
    }
  }

  #picker(open: boolean): void {
    const picker = this.target("emoji");
    if (!picker) {
      return;
    }
    picker.hidden = !open;
    this.target("emoji-button")?.setAttribute("aria-expanded", String(open));
    if (open) {
      picker.querySelector<HTMLElement>("button")?.focus();
    }
  }

  send(event: SubmitEvent): void {
    event.preventDefault();
    this.#picker(false);
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

  async record(): Promise<void> {
    this.#say("");
    this.#picker(false);
    if (this.#live) {
      let recorder: Recorder;
      try {
        const { Recorder } = await import("../lib/recorder");
        recorder = new Recorder();
        await recorder.start();
      } catch {
        this.#say("Allow the microphone to record a voice note.");
        return;
      }
      this.#recorder = recorder;
      this.#follow(recorder);
    }
    this.element.dataset.recording = "true";
    this.#seconds = 0;
    this.#tick();
    this.#timer = setInterval(() => {
      this.#seconds += 1;
      this.#tick();
      if (this.#seconds >= maxSeconds) {
        void this.finish();
      }
    }, 1000);
  }

  discard(): void {
    this.#recorder?.cancel();
    this.#stop();
  }

  async finish(): Promise<void> {
    const duration = clock(Math.max(this.#seconds, 1));
    const recorder = this.#recorder;
    this.#stop();
    if (!recorder) {
      this.dispatch("send", { body: "", files: [], voice: duration });
      return;
    }
    try {
      const note = await recorder.stop();
      this.dispatch("send", { body: "", files: [note], voice: duration });
    } catch {
      this.#say("The voice note could not be saved. Try again.");
    }
  }

  #stop(): void {
    clearInterval(this.#timer);
    clearInterval(this.#meter);
    this.#recorder = undefined;
    delete this.element.dataset.recording;
    delete this.element.dataset.listening;
  }

  /*
   * follow moves the recording bar with the microphone's level, newest on
   * the right.
   */
  #follow(recorder: Recorder): void {
    const bars = [...this.element.querySelectorAll<HTMLElement>(".live i")];
    this.element.dataset.listening = "true";
    this.#meter = setInterval(() => {
      for (let i = 0; i < bars.length - 1; i++) {
        (bars[i] as HTMLElement).style.height = (bars[i + 1] as HTMLElement).style.height;
      }
      const last = bars.at(-1);
      if (last) {
        last.style.height = `${4 + Math.round(recorder.level() * 18)}px`;
      }
    }, 80);
  }

  #say(message: string): void {
    const note = this.target("note");
    if (note) {
      note.textContent = message;
      note.hidden = message === "";
    }
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
