/*
 * Choices are the things a picker offers, as the ids settings store and
 * the names people read.
 */
export type Choices = ReadonlyArray<readonly [id: string, name: string]>;

/*
 * choicesOf lists a record of names in its own order.
 */
export function choicesOf(names: Record<string, string>): Choices {
  return Object.entries(names);
}
