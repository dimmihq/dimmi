/*
 * The messenger's two sounds, drawn with the Web Audio API instead of
 * shipped as files: "in" when a message from the team or the AI agent
 * arrives, two soft sine plucks a whole step apart with the second one
 * stronger, and "out" when the visitor's own message leaves, two quick
 * plucks a fifth apart. Both run through a low-pass filter so they stay
 * round rather than beepy. The dashboard plays the same ones to preview
 * them. The messenger loads this module on the first sound, so a page
 * that never plays one never fetches it.
 *
 * Browsers only allow sound once the visitor has interacted with the page,
 * so a page nobody has touched stays silent rather than queueing a sound
 * for later, and a browser without Web Audio stays silent too.
 */

export type Sound = "in" | "out";

/*
 * Note is one tone: its wave, the pitch it glides from and to in hertz,
 * its peak level, when it starts and how long it rings, in seconds.
 */
type Note = [wave: OscillatorType, from: number, to: number, level: number, at: number, length: number];

/*
 * notes are the tones of each sound.
 */
export const notes: Record<Sound, Note[]> = {
  in: [
    ["sine", 392, 392, 0.1, 0, 0.16],
    ["sine", 440, 440, 0.2, 0.13, 0.26],
  ],
  out: [
    ["sine", 523, 523, 0.13, 0, 0.12],
    ["sine", 784, 784, 0.13, 0.11, 0.14],
  ],
};

/*
 * playing reads a sound setting: "none" turns that sound off, and any
 * other value plays the messenger's sound. The settings once named one of
 * several sounds, so older values still mean on.
 */
export function playing(value: unknown): boolean {
  return value !== "none";
}

let context: AudioContext | undefined;

/*
 * audible reports whether the page may play a sound now: the browser has
 * Web Audio and the visitor has interacted with the page.
 */
function audible(): boolean {
  if (typeof AudioContext === "undefined") {
    return false;
  }
  const activation = (globalThis.navigator as Navigator & { userActivation?: { hasBeenActive: boolean } })
    ?.userActivation;

  return activation?.hasBeenActive !== false;
}

/*
 * play plays a sound now. A sound is never worth breaking the messenger
 * for, so a browser whose Web Audio falls short stays silent instead.
 */
export function play(sound: Sound): void {
  if (!audible()) {
    return;
  }

  try {
    if (!(context instanceof AudioContext)) {
      context = new AudioContext();
    }
    draw(context, sound);
  } catch {
    // Silence is the fallback.
  }
}

function draw(audio: AudioContext, sound: Sound): void {
  if (audio.state === "suspended") {
    void audio.resume().catch(() => undefined);
  }
  const now = audio.currentTime;
  const tone = audio.createBiquadFilter();
  tone.type = "lowpass";
  tone.frequency.value = 2600;
  tone.connect(audio.destination);

  for (const [wave, from, to, level, at, length] of notes[sound]) {
    const note = audio.createOscillator();
    const volume = audio.createGain();
    const start = now + at;
    note.type = wave;
    note.frequency.setValueAtTime(from, start);
    if (to !== from) {
      note.frequency.exponentialRampToValueAtTime(to, start + Math.min(0.06, length / 2));
    }
    fade(volume.gain, start, level, length);
    note.connect(volume).connect(tone);
    note.start(start);
    note.stop(start + length + 0.02);
  }
}

function fade(gain: AudioParam, start: number, level: number, length: number): void {
  gain.setValueAtTime(0.0001, start);
  gain.exponentialRampToValueAtTime(level, start + 0.008);
  gain.exponentialRampToValueAtTime(0.0001, start + length);
}
