/*
 * Small text helpers the components share.
 */

export function initials(name: string): string {
  return name
    .split(" ")
    .map((part) => part[0])
    .join("")
    .slice(0, 2);
}

/*
 * The bar heights of a voice note's waveform, in pixels. Recordings will
 * carry their own levels; until then every note draws this shape.
 */
export const levels = [5, 9, 14, 8, 18, 12, 20, 10, 15, 7, 17, 11, 19, 9, 13, 6, 16, 10, 12, 8, 14, 6, 9, 5];
