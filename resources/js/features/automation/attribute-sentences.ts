/*
 * What the card of a step that sets conversation attributes says.
 */
export function attributesSentence(step: { settings?: Record<string, unknown> }): string {
  const list = step.settings?.attributes;
  const count = Array.isArray(list) ? list.length : 0;
  if (count === 0) {
    return "Not set up yet";
  }

  return `Set ${count === 1 ? "1 conversation attribute" : `${count} conversation attributes`}`;
}
