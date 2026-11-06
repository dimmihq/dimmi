/*
 * Suggestions while typing, such as emoji after a colon or teammates
 * after an at sign. Typing the
 * trigger and a few letters finds matches; the arrow keys move through
 * them, Enter or Tab takes one and Escape lets them go. The editor
 * plugin finds the words typed; this file keeps which match is chosen
 * and tells the composer what to show.
 */
import { PluginKey } from "@tiptap/pm/state";
import { type Content, Extension } from "@tiptap/react";
import Suggestion, { type SuggestionKeyDownProps, type SuggestionProps } from "@tiptap/suggestion";

/*
 * Suggested is what the list shows: the matches, which one is chosen,
 * where the caret is and how to take a match.
 */
export interface Suggested<T> {
  items: T[];
  index: number;
  caret: DOMRect | null;
  pick: (item: T) => void;
}

export interface SuggestionSource<T> {
  name: string;
  char: string;
  minLength: number;
  find: (query: string) => Promise<T[]> | T[];
  content: (item: T) => Content;
  onChange: (suggested: Suggested<T> | null) => void;
}

/*
 * moveKey says how a key moves through the list, or that it takes the
 * chosen match, closes the list, or is not the list's.
 */
function moveKey(key: string): number | "pick" | "close" | null {
  const moves: Record<string, number | "pick" | "close"> = {
    ArrowDown: 1,
    ArrowUp: -1,
    Enter: "pick",
    Tab: "pick",
    Escape: "close",
  };

  return moves[key] ?? null;
}

/*
 * renderer keeps the matches and the chosen one while the person types,
 * telling the source each time they change.
 */
function renderer<T>(source: SuggestionSource<T>) {
  let current: SuggestionProps<T, T> | null = null;
  let index = 0;
  const push = () => {
    const items = current?.items ?? [];
    source.onChange(
      current && items.length > 0
        ? { items, index, caret: current.clientRect?.() ?? null, pick: (item) => current?.command(item) }
        : null,
    );
  };
  const start = (props: SuggestionProps<T, T>) => {
    current = props;
    index = 0;
    push();
  };
  const press = ({ event }: SuggestionKeyDownProps) => {
    const move = moveKey(event.key);
    const items = current?.items ?? [];
    if (move === null || items.length === 0) {
      return false;
    }
    const chosen = items[index];
    if (move === "close") {
      current = null;
    } else if (move === "pick" && chosen !== undefined) {
      current?.command(chosen);
    } else if (typeof move === "number") {
      index = (index + move + items.length) % items.length;
    }
    push();
    return true;
  };

  return {
    onStart: start,
    onUpdate: start,
    onKeyDown: press,
    onExit: () => {
      current = null;
      push();
    },
  };
}

export function suggestionExtension<T>(source: SuggestionSource<T>) {
  return Extension.create({
    name: source.name,
    addProseMirrorPlugins() {
      return [
        Suggestion<T, T>({
          editor: this.editor,
          pluginKey: new PluginKey(source.name),
          char: source.char,
          minQueryLength: source.minLength,
          items: ({ query }) => source.find(query),
          command: ({ editor, range, props }) => {
            editor.chain().focus().insertContentAt(range, source.content(props)).run();
          },
          render: () => renderer(source),
        }),
      ];
    },
  });
}
