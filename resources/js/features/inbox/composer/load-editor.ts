/*
 * The formatting editor lives in its own chunk. loadEditor fetches it,
 * once; preloadEditor starts that ahead of the first Reply or Note, so
 * the composer opens with it. Once it has arrived, loadedEditor hands
 * it over directly, so a field opened later never shows the plain
 * stand-in, whose typing a swap could drop.
 */
import type { ComponentType } from "react";

import type { RichEditorProps } from "./rich-editor";

let loaded: ComponentType<RichEditorProps> | null = null;

export const loadEditor = () =>
  import("./rich-editor").then((module) => {
    loaded = module.default;
    return module;
  });

export function loadedEditor(): ComponentType<RichEditorProps> | null {
  return loaded;
}

export function preloadEditor() {
  loadEditor().catch(() => undefined);
}
