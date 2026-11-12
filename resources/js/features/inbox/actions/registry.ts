/*
 * Every move a teammate makes in the inbox, declared once: its name, its
 * keyboard shortcut, when it applies and what it does. Keyboard
 * shortcuts, the command menu and the shortcut sheet all read this list,
 * so a move shows the same name and key everywhere. Each area adds its
 * moves in a file of its own beside this one.
 */
import type { RemixiconComponentType } from "@remixicon/react";

export type ActionGroup = "Conversation" | "Navigation" | "Help";

/*
 * ActionContext is what a move can act on: the workspace, the open
 * conversation's controls when one is open, and how to move around.
 */
export interface ActionContext {
  workspace: string;
  conversation: ConversationMoves | null;
  go: (path: string) => void;
  open: (panel: "command-menu" | "shortcuts") => void;
}

/*
 * ConversationMoves are what the open conversation lets a move do.
 */
export interface ConversationMoves {
  id: string;
  status: "open" | "snoozed" | "closed";
  close: () => void;
  reopen: () => void;
  snooze: () => void;
  assignToMe: () => void;
  assign: () => void;
  canTrash: boolean;
  trash: () => void;
  spam: boolean;
  markUnread: () => void;
  toggleSpam: () => void;
  merge: () => void;
  toggleDetails: () => void;
  toggleEvents: () => void;
  next: () => void;
  previous: () => void;
  advance: () => void;
}

export interface InboxAction {
  id: string;
  label: string;
  group: ActionGroup;
  icon?: RemixiconComponentType;
  /*
   * shortcut is in tinykeys' notation, such as "Shift+A" or "g i";
   * keys is how the sheet and menu show it.
   */
  shortcut?: string;
  keys?: string[];
  allowInInput?: boolean;
  when: (ctx: ActionContext) => boolean;
  run: (ctx: ActionContext) => void;
}

/*
 * available returns the moves that apply in the context.
 */
export function available(actions: InboxAction[], ctx: ActionContext): InboxAction[] {
  return actions.filter((action) => action.when(ctx));
}
