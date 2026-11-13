/*
 * The options of a choice as they are saved: trimmed, without blanks or
 * repeats, in the order given.
 */
export function optionList(options: string[]): string[] {
  return [...new Set(options.map((o) => o.trim()))].filter(Boolean);
}
