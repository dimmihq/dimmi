/*
 * A voice note's player. The waveform's bars take their heights when the
 * bubble appears, and play fills them from the left over the note's
 * length, then rewinds. There is no audio yet; the player shows how a
 * voice note will behave once recordings are stored.
 */
import { Controller } from "../lib/controller";

export class VoiceController extends Controller {
  #timer: ReturnType<typeof setInterval> | undefined;
  #elapsed = 0;

  connect(): void {
    for (const bar of this.#bars) {
      bar.style.height = `${bar.dataset.height ?? 8}px`;
    }
  }

  disconnect(): void {
    clearInterval(this.#timer);
  }

  get #bars(): HTMLElement[] {
    return Array.from(this.element.querySelectorAll<HTMLElement>(".wave i"));
  }

  get #length(): number {
    const [minutes = 0, seconds = 0] = (this.element.dataset.length ?? "0:05").split(":").map(Number);
    return Math.max(1, minutes * 60 + seconds) * 1000;
  }

  toggle(): void {
    if (this.element.dataset.playing === "true") {
      this.#stop(false);
      return;
    }

    this.element.dataset.playing = "true";
    this.element.querySelector("button")?.setAttribute("aria-label", "Pause voice note");
    this.#timer = setInterval(() => {
      this.#elapsed += 100;
      this.#paint();
      if (this.#elapsed >= this.#length) {
        this.#stop(true);
      }
    }, 100);
  }

  #stop(rewind: boolean): void {
    clearInterval(this.#timer);
    this.element.dataset.playing = "false";
    this.element.querySelector("button")?.setAttribute("aria-label", "Play voice note");
    if (rewind) {
      this.#elapsed = 0;
      this.#paint();
    }
  }

  #paint(): void {
    const bars = this.#bars;
    const played = Math.round((this.#elapsed / this.#length) * bars.length);
    for (const [index, bar] of bars.entries()) {
      bar.toggleAttribute("data-played", index < played);
    }
  }
}
