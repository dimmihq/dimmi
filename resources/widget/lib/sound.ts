/*
 * The soft "pop" the messenger makes when it opens: a short sine note that
 * glides up a fifth as it blooms, with a quiet octave above for air, all
 * through a low-pass filter so it stays round rather than beepy. It is
 * drawn with the Web Audio API instead of shipped as a file. Browsers only
 * allow sound after the visitor interacts with the page, which is why it
 * plays on open, a click, and never on page load. A browser without Web
 * Audio stays silent.
 */

let context: AudioContext | undefined;

export function pop(): void {
  if (typeof AudioContext === "undefined") {
    return;
  }

  context ??= new AudioContext();
  const now = context.currentTime;
  const tone = context.createBiquadFilter();
  tone.type = "lowpass";
  tone.frequency.value = 2600;
  tone.connect(context.destination);

  for (const [from, to, level] of [
    [523, 784, 0.16],
    [1046, 1568, 0.035],
  ] as const) {
    const note = context.createOscillator();
    const volume = context.createGain();
    note.frequency.setValueAtTime(from, now);
    note.frequency.exponentialRampToValueAtTime(to, now + 0.06);
    volume.gain.setValueAtTime(0.0001, now);
    volume.gain.exponentialRampToValueAtTime(level, now + 0.008);
    volume.gain.exponentialRampToValueAtTime(0.0001, now + 0.28);
    note.connect(volume).connect(tone);
    note.start(now);
    note.stop(now + 0.3);
  }
}
