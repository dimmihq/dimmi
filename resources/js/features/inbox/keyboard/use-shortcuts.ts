/*
 * Binds the moves' keyboard shortcuts with tinykeys. A shortcut never
 * fires while the person types or works in a dialog, unless the move
 * allows it, such as the command menu; a letter in a reply stays a
 * letter. A shortcut that fires takes the key from the browser.
 */
import { useEffect, useRef } from "react";
import { tinykeys } from "tinykeys";

import type { ActionContext, InboxAction } from "../actions/registry";
import { isTyping } from "./typing";

/*
 * guarded returns the handler of a move's shortcut, which does nothing
 * while typing unless allowed, or when the move does not apply.
 */
export function guarded(action: InboxAction, context: () => ActionContext) {
  return (event: KeyboardEvent) => {
    if (!action.allowInInput && isTyping(event.target)) {
      return;
    }
    const ctx = context();
    if (!action.when(ctx)) {
      return;
    }
    event.preventDefault();
    action.run(ctx);
  };
}

export function useShortcuts(actions: InboxAction[], ctx: ActionContext) {
  const current = useRef(ctx);
  current.current = ctx;

  useEffect(() => {
    const bindings: Record<string, (event: KeyboardEvent) => void> = {};
    for (const action of actions) {
      if (action.shortcut) {
        bindings[action.shortcut] = guarded(action, () => current.current);
      }
    }

    return tinykeys(window, bindings);
  }, [actions]);
}
