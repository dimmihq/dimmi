/*
 * The formatting editor lives in its own chunk. loadEditor fetches it,
 * once; preloadEditor starts that ahead of the first Reply or Note, so
 * the composer opens with it.
 */
export const loadEditor = () => import("./rich-editor");

export function preloadEditor() {
  loadEditor().catch(() => undefined);
}
