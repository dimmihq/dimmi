/*
 * Responsive sources for the captures in public/shots. Each one has a
 * half-width .1x.webp beside it (site/shots/downscale.ts), and
 * shot-files.json holds both pixel widths, so the browser can pick the
 * smaller file wherever the shot shows small or the screen is 1x.
 */
import files from "./shot-files.json";

const widths: Record<string, number[]> = files;

/* The srcset for /shots/{file}.webp, or undefined when it has no variant. */
export function shotSrcSet(file: string) {
  const w = widths[file];
  if (!w) {
    return undefined;
  }
  return `/shots/${file}.1x.webp ${w[0]}w, /shots/${file}.webp ${w[1]}w`;
}

/*
 * The sizes value for a shot drawn at most `px` CSS pixels wide that
 * otherwise fills the screen inside the sections' 20px side gutters.
 */
export function upTo(px: number) {
  return `(max-width: ${px + 40}px) calc(100vw - 40px), ${px}px`;
}

/*
 * A shot spanning a section's content: the 1200px frame and its border,
 * minus the frame's side padding (px-5, sm:px-12) and `inset` more px.
 */
export function sectionWidth(inset: number) {
  return `(min-width: 1200px) ${1200 - 2 - 96 - inset}px, (min-width: 640px) calc(100vw - ${2 + 96 + inset}px), calc(100vw - ${2 + 40 + inset}px)`;
}
