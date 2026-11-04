/*
 * The moves of every area, in the order the sheet and menu list them.
 */
import { conversationActions } from "./conversation-actions";
import { navigationActions } from "./navigation-actions";
import type { InboxAction } from "./registry";

export const allActions: InboxAction[] = [...conversationActions, ...navigationActions];
