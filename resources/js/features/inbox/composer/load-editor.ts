/*
 * The formatting editor lives in its own chunk. loadEditor fetches it,
 * once; preloadEditor starts that ahead of the first Reply or Note, so
 * the composer opens with it. loadedEditor hands it over once it has
 * arrived, so a field opened after that starts as the editor.
 */
type EditorModule = typeof import("./rich-editor");

let arrived: EditorModule | undefined;

export const loadEditor = () =>
  import("./rich-editor").then((module) => {
    arrived = module;
    return module;
  });

export const loadedEditor = () => arrived?.default;

export function preloadEditor() {
  loadEditor().catch(() => undefined);
}
