import { cx } from "@/utils/cx";

/*
 * quiet is how a dropdown's trigger looks as a row's value: text that
 * gains a surface under the pointer, the same as the quiet select. Its
 * negative margin keeps the text on the row's edge.
 */
export const quiet = cx(
  "-ms-1.5 flex h-7 min-w-0 max-w-full cursor-pointer items-center gap-1.5 rounded-lg px-1.5 text-start",
  "text-body-2-regular text-text-primary outline-none transition-colors duration-150",
  "hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring",
);

/*
 * onRowHover hides a row's small button until the pointer is on the row
 * or the keyboard is on the button.
 */
export const onRowHover =
  "opacity-0 transition-opacity duration-150 focus-visible:opacity-100 group-hover/row:opacity-100";
