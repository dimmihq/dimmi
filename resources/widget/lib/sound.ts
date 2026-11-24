/*
 * The messenger's sounds, drawn with the Web Audio API instead of shipped
 * as files: a few sine and triangle notes that glide and fade, and for the
 * swoosh a burst of noise swept through a band-pass filter, all through a
 * low-pass filter so they stay round rather than beepy. The messenger
 * plays "pop" when it opens and the sounds its settings pick when the
 * visitor sends a message and when a reply arrives; the dashboard plays
 * the same ones to preview them. The messenger loads this module on the
 * first sound, so a page that never plays one never fetches it.
 *
 * Browsers only allow sound once the visitor has interacted with the page,
 * so a page nobody has touched stays silent rather than queueing a sound
 * for later, and a browser without Web Audio stays silent too.
 */

export const sounds = ["none", "pop", "chime", "bubble", "swoosh", "tick"] as const;

export type Sound = (typeof sounds)[number];

/*
 * Note is one tone: its wave, the pitch it glides from and to in hertz,
 * its peak level, when it starts and how long it rings, in seconds.
 */
type Note = [wave: OscillatorType, from: number, to: number, level: number, at: number, length: number];

/*
 * notes are the tones of each sound; swoosh has none, it is noise.
 */
export const notes: Record<Exclude<Sound, "none">, Note[]> = {
  pop: [
    ["sine", 523, 784, 0.16, 0, 0.28],
    ["sine", 1046, 1568, 0.035, 0, 0.28],
  ],
  chime: [
    ["sine", 880, 880, 0.1, 0, 0.55],
    ["sine", 1319, 1319, 0.08, 0.09, 0.65],
  ],
  bubble: [
    ["sine", 320, 960, 0.16, 0, 0.12],
    ["sine", 520, 1320, 0.08, 0.07, 0.11],
  ],
  swoosh: [],
  tick: [["triangle", 1760, 1180, 0.09, 0, 0.06]],
};

/*
 * soundOf reads a setting as a sound, the fallback when it names none the
 * messenger knows.
 */
export function soundOf(value: unknown, fallback: Sound): Sound {
  return sounds.includes(value as Sound) ? (value as Sound) : fallback;
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
  if (sound === "none" || !audible()) {
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

function draw(audio: AudioContext, sound: Exclude<Sound, "none">): void {
  if (audio.state === "suspended") {
    void audio.resume().catch(() => undefined);
  }
  const now = audio.currentTime;
  const tone = audio.createBiquadFilter();
  tone.type = "lowpass";
  tone.frequency.value = 2600;
  tone.connect(audio.destination);

  if (sound === "swoosh") {
    swoosh(audio, tone, now);
    return;
  }

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

/*
 * swoosh sweeps a quarter second of noise up through a band-pass filter,
 * the sound of a message leaving.
 */
function swoosh(audio: AudioContext, out: AudioNode, now: number): void {
  const length = 0.26;
  const buffer = audio.createBuffer(1, Math.ceil(audio.sampleRate * length), audio.sampleRate);
  const samples = buffer.getChannelData(0);
  for (let i = 0; i < samples.length; i++) {
    samples[i] = Math.random() * 2 - 1;
  }
  const noise = audio.createBufferSource();
  noise.buffer = buffer;
  const band = audio.createBiquadFilter();
  band.type = "bandpass";
  band.Q.value = 1.4;
  band.frequency.setValueAtTime(500, now);
  band.frequency.exponentialRampToValueAtTime(2400, now + length);
  const volume = audio.createGain();
  volume.gain.setValueAtTime(0.0001, now);
  volume.gain.exponentialRampToValueAtTime(0.22, now + 0.08);
  volume.gain.exponentialRampToValueAtTime(0.0001, now + length);
  noise.connect(band).connect(volume).connect(out);
  noise.start(now);
}
