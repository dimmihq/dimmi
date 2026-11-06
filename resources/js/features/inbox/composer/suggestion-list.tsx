/*
 * The list of suggestions above the caret while typing, such as emoji
 * after a colon. A click takes a match without moving the focus out of
 * the editor.
 */
import type { ReactNode } from "react";
import { createPortal } from "react-dom";

import { MENU_POPOVER_SURFACE } from "@/components/base/dropdown/menu-styles";
import { cx } from "@/utils/cx";

import type { Suggested } from "./suggestion-extension";

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

  return createPortal(
    <div
      role="listbox"
      aria-label={label}
      style={{ left: caret.left, bottom: window.innerHeight - caret.top + 6 }}
      className={cx(MENU_POPOVER_SURFACE, "fixed z-50 flex w-64 flex-col gap-0.5 p-1")}
    >
      {suggested.items.map((item, index) => (
        <div
          key={keyOf(item)}
          role="option"
          tabIndex={-1}
          aria-selected={index === suggested.index}
          onMouseDown={(event) => {
            event.preventDefault();
            suggested.pick(item);
          }}
          className={cx(
            "flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-body-medium text-text-primary",
            index === suggested.index && "bg-background-primary-hover",
          )}
        >
          {render(item)}
        </div>
      ))}
    </div>,
    document.body,
  );
}
