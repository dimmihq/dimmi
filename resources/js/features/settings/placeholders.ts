/*
 * Keys for stand-in rows while a list loads, so each skeleton row has a
 * key of its own without leaning on its position.
 */

/*
 * placeholders names as many stand-in rows as asked for.
 */
export function placeholders(count: number): string[] {
  return Array.from({ length: count }, (_, i) => `placeholder-${i}`);
}
