/*
 * The list of suggestions above the caret while typing, such as emoji
 * after a colon, or below it when there is no room above. A long list
 * scrolls and keeps the chosen match in view. A click takes a match
 * without moving the focus out of the editor.
 */
import type { ReactNode } from "react";
import { createPortal } from "react-dom";

import { MENU_ITEM, MENU_ITEM_ACTIVE, MENU_POPOVER_SURFACE } from "@/components/base/dropdown/menu-styles";
import { cx } from "@/utils/cx";

import type { Suggested } from "./suggestion-extension";

/*
 * roomAbove is how far below the top of the window the caret must be for
 * the list to open above it.
 */
const roomAbove = 340;

export function SuggestionList<T>({
  label,
  suggested,
  render,
  keyOf,
}: {
  label: string;
  suggested: Suggested<T> | null;
  render: (item: T) => ReactNode;
  keyOf: (item: T) => string;
}) {
  if (!suggested?.caret) {
    return null;
  }
  const { caret } = suggested;
  const below = caret.top < roomAbove;
  const place = below
    ? { left: caret.left, top: caret.bottom + 6 }
    : { left: caret.left, bottom: window.innerHeight - caret.top + 6 };

  return createPortal(
    <div
      role="listbox"
      aria-label={label}
      style={place}
      className={cx(
        MENU_POPOVER_SURFACE,
        "fixed flex max-h-80 w-72 flex-col gap-0.5 p-1.5 motion-safe:animate-menu-in",
        below ? "origin-top-left" : "origin-bottom-left",
      )}
    >
      {suggested.items.map((item, index) => (
        <div
          key={keyOf(item)}
          role="option"
          tabIndex={-1}
          aria-selected={index === suggested.index}
          ref={index === suggested.index ? (node) => node?.scrollIntoView({ block: "nearest" }) : undefined}
          onMouseDown={(event) => {
            event.preventDefault();
            suggested.pick(item);
          }}
          className={cx(MENU_ITEM, "py-1.5 text-body-medium", index === suggested.index && MENU_ITEM_ACTIVE)}
        >
          {render(item)}
        </div>
      ))}
    </div>,
    document.body,
  );
}
