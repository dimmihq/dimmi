/*
 * A voice note's player. With the recording at hand it plays the audio:
 * the waveform fills from the left as it plays, the time counts down what
 * is left, and a click on the waveform jumps there. Recordings straight
 * from a browser often do not say how long they are, so the player finds
 * out by seeking to the end once. A note without audio yet, such as one
 * still sending or one in a preview, plays the motions over its stated
 * length.
 */
import { Controller } from "../lib/controller";

function clock(seconds: number): string {
  const whole = Math.max(0, Math.round(seconds));
  return `${Math.floor(whole / 60)}:${String(whole % 60).padStart(2, "0")}`;
}

export class VoiceController extends Controller {
  #timer: ReturnType<typeof setInterval> | undefined;
  #elapsed = 0;
  #measuring = false;

  connect(): void {
    for (const bar of this.#bars) {
      bar.style.height = `${bar.dataset.height ?? 8}px`;
    }
    const audio = this.#audio;
    if (!audio) {
      return;
    }
    audio.addEventListener("loadedmetadata", () => this.#measure());
    audio.addEventListener("durationchange", () => this.#settle());
    audio.addEventListener("timeupdate", () => this.#paintAudio());
    audio.addEventListener("ended", () => this.#ended());
  }

  disconnect(): void {
    clearInterval(this.#timer);
    this.#audio?.pause();
  }

  get #audio(): HTMLAudioElement | null {
    return this.element.querySelector("audio");
  }

  get #bars(): HTMLElement[] {
    return Array.from(this.element.querySelectorAll<HTMLElement>(".wave i"));
  }

  get #length(): number {
    const [minutes = 0, seconds = 0] = (this.element.dataset.length ?? "0:05").split(":").map(Number);
    return Math.max(1, minutes * 60 + seconds) * 1000;
  }

  toggle(): void {
    const audio = this.#audio;
    if (audio) {
      this.#toggleAudio(audio);
      return;
    }
    if (this.element.dataset.playing === "true") {
      this.#stop(false);
      return;
    }

    this.#playing(true);
    this.#timer = setInterval(() => {
      this.#elapsed += 100;
      this.#paint(this.#elapsed / this.#length);
      if (this.#elapsed >= this.#length) {
        this.#stop(true);
      }
    }, 100);
  }

  /*
   * seek jumps to where the waveform was clicked.
   */
  seek(event: MouseEvent): void {
    const audio = this.#audio;
    const wave = (event.target as Element).closest<HTMLElement>(".wave");
    if (!audio || !wave || !Number.isFinite(audio.duration)) {
      return;
    }
    const box = wave.getBoundingClientRect();
    const share = Math.min(1, Math.max(0, (event.clientX - box.left) / box.width));
    audio.currentTime = share * audio.duration;
    this.#paintAudio();
  }

  #toggleAudio(audio: HTMLAudioElement): void {
    if (!audio.paused) {
      audio.pause();
      this.#playing(false);
      return;
    }
    this.#playing(true);
    void audio.play().catch(() => this.#playing(false));
  }

  /*
   * measure finds a recording's length when it does not state one, by
   * asking for a time far past its end and reading where the browser
   * stops.
   */
  #measure(): void {
    const audio = this.#audio;
    if (audio && !Number.isFinite(audio.duration)) {
      this.#measuring = true;
      audio.currentTime = 1e7;
      return;
    }
    this.#settle();
  }

  #settle(): void {
    const audio = this.#audio;
    if (!audio || !Number.isFinite(audio.duration)) {
      return;
    }
    if (this.#measuring) {
      this.#measuring = false;
      audio.currentTime = 0;
    }
    const time = this.element.querySelector("time");
    if (time) {
      time.textContent = clock(audio.duration);
    }
  }

  #paintAudio(): void {
    const audio = this.#audio;
    if (!audio || this.#measuring || !Number.isFinite(audio.duration) || audio.duration === 0) {
      return;
    }
    this.#paint(audio.currentTime / audio.duration);
    const time = this.element.querySelector("time");
    if (time) {
      time.textContent = clock(audio.paused ? audio.duration : audio.duration - audio.currentTime);
    }
  }

  #ended(): void {
    const audio = this.#audio;
    this.#playing(false);
    this.#paint(0);
    if (audio) {
      audio.currentTime = 0;
    }
    this.#paintAudio();
  }

  #playing(on: boolean): void {
    this.element.dataset.playing = String(on);
    this.element.querySelector("button")?.setAttribute("aria-label", on ? "Pause voice note" : "Play voice note");
  }

  #stop(rewind: boolean): void {
    clearInterval(this.#timer);
    this.#playing(false);
    if (rewind) {
      this.#elapsed = 0;
      this.#paint(0);
    }
  }

  #paint(share: number): void {
    const bars = this.#bars;
    const played = Math.round(share * bars.length);
    for (const [index, bar] of bars.entries()) {
      bar.toggleAttribute("data-played", index < played);
    }
  }
}
