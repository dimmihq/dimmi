/*
 * Records a voice note from the visitor's microphone. The browser picks
 * the format it records best: Opus in WebM in Chrome and Firefox, AAC in
 * MP4 in Safari. The microphone is asked for only when the visitor starts
 * recording, and let go as soon as they finish or throw the note away,
 * so the browser's recording light never stays on. A level reading lets
 * the recording bar move with the visitor's voice.
 */

const formats = [
  { type: "audio/webm;codecs=opus", extension: "webm" },
  { type: "audio/mp4", extension: "m4a" },
  { type: "audio/ogg;codecs=opus", extension: "ogg" },
  { type: "audio/webm", extension: "webm" },
];

/*
 * maxSeconds is the longest voice note; the recording finishes itself
 * there.
 */
export const maxSeconds = 300;

/*
 * canRecord reports whether this page may record: the browser can, and
 * the page is served securely, since browsers keep the microphone from
 * any other.
 */
export function canRecord(win: Window = window): boolean {
  return Boolean(win.isSecureContext && "mediaDevices" in win.navigator && "MediaRecorder" in win);
}

function format(): { type: string; extension: string } {
  const found = formats.find((f) => MediaRecorder.isTypeSupported(f.type));
  return found ?? { type: "", extension: "webm" };
}

export class Recorder {
  #stream: MediaStream | undefined;
  #recorder: MediaRecorder | undefined;
  #chunks: Blob[] = [];
  #context: AudioContext | undefined;
  #analyser: AnalyserNode | undefined;
  #extension = "webm";

  /*
   * start asks for the microphone and starts recording. It rejects when
   * the visitor or the page refuses the microphone.
   */
  async start(): Promise<void> {
    this.#stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    const chosen = format();
    this.#extension = chosen.extension;
    this.#chunks = [];
    this.#recorder = new MediaRecorder(this.#stream, chosen.type ? { mimeType: chosen.type } : undefined);
    this.#recorder.addEventListener("dataavailable", (event) => {
      if (event.data.size > 0) {
        this.#chunks.push(event.data);
      }
    });
    this.#recorder.start(250);
    this.#listen(this.#stream);
  }

  /*
   * level is how loud the microphone is right now, from 0 to 1.
   */
  level(): number {
    if (!this.#analyser) {
      return 0;
    }
    const samples = new Uint8Array(this.#analyser.fftSize);
    this.#analyser.getByteTimeDomainData(samples);
    let sum = 0;
    for (const sample of samples) {
      const centred = (sample - 128) / 128;
      sum += centred * centred;
    }

    return Math.min(1, Math.sqrt(sum / samples.length) * 4);
  }

  /*
   * stop finishes the recording and returns it as a file to send.
   */
  async stop(): Promise<File> {
    const recorder = this.#recorder;
    if (!recorder) {
      throw new Error("not recording");
    }
    const done = new Promise<void>((resolve) => recorder.addEventListener("stop", () => resolve(), { once: true }));
    recorder.stop();
    await done;
    const type = (recorder.mimeType || this.#chunks[0]?.type || "audio/webm").split(";")[0] ?? "audio/webm";
    const file = new File(this.#chunks, `Voice note.${this.#extension}`, { type });
    this.#release();

    return file;
  }

  /*
   * cancel throws the recording away.
   */
  cancel(): void {
    if (this.#recorder && this.#recorder.state !== "inactive") {
      this.#recorder.stop();
    }
    this.#release();
  }

  #listen(stream: MediaStream): void {
    try {
      this.#context = new AudioContext();
      this.#analyser = this.#context.createAnalyser();
      this.#analyser.fftSize = 512;
      this.#context.createMediaStreamSource(stream).connect(this.#analyser);
    } catch {
      this.#analyser = undefined;
    }
  }

  #release(): void {
    for (const track of this.#stream?.getTracks() ?? []) {
      track.stop();
    }
    void this.#context?.close().catch(() => undefined);
    this.#stream = undefined;
    this.#recorder = undefined;
    this.#context = undefined;
    this.#analyser = undefined;
    this.#chunks = [];
  }
}
