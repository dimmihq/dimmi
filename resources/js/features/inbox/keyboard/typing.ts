/*
 * Single-key shortcuts never fire while someone is typing or working in
 * an open dialog: a P in a reply is a letter, not a command.
 */
export function isTyping(target: EventTarget | null): boolean {
  return (
    target instanceof Element &&
    target.closest("input, textarea, select, [contenteditable='true'], [role='dialog'], [role='alertdialog']") !== null
  );
}
