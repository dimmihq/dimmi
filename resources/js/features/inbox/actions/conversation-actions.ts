/*
 * The moves on the open conversation: walking the list, closing,
 * reopening, snoozing and taking it.
 */
import {
  RiAlarmLine,
  RiArrowDownLine,
  RiArrowUpLine,
  RiCheckboxCircleLine,
  RiRefreshLine,
  RiUserReceivedLine,
} from "@remixicon/react";

import type { InboxAction } from "./registry";

export const conversationActions: InboxAction[] = [
  {
    id: "conversation.next",
    label: "Next conversation",
    group: "Conversation",
    icon: RiArrowDownLine,
    shortcut: "j",
    keys: ["J"],
    when: (ctx) => ctx.conversation !== null,
    run: (ctx) => ctx.conversation?.next(),
  },
  {
    id: "conversation.previous",
    label: "Previous conversation",
    group: "Conversation",
    icon: RiArrowUpLine,
    shortcut: "k",
    keys: ["K"],
    when: (ctx) => ctx.conversation !== null,
    run: (ctx) => ctx.conversation?.previous(),
  },
  {
    id: "conversation.close",
    label: "Mark as done",
    group: "Conversation",
    icon: RiCheckboxCircleLine,
    shortcut: "$mod+Shift+Y",
    keys: ["⌘", "Shift", "Y"],
    when: (ctx) => ctx.conversation !== null && ctx.conversation.status !== "closed",
    run: (ctx) => ctx.conversation?.close(),
  },
  {
    id: "conversation.reopen",
    label: "Reopen",
    group: "Conversation",
    icon: RiRefreshLine,
    shortcut: "$mod+Shift+O",
    keys: ["⌘", "Shift", "O"],
    when: (ctx) => ctx.conversation !== null && ctx.conversation.status !== "open",
    run: (ctx) => ctx.conversation?.reopen(),
  },
  {
    id: "conversation.snooze",
    label: "Snooze",
    group: "Conversation",
    icon: RiAlarmLine,
    shortcut: "z",
    keys: ["Z"],
    when: (ctx) => ctx.conversation !== null && ctx.conversation.status !== "snoozed",
    run: (ctx) => ctx.conversation?.snooze(),
  },
  {
    id: "conversation.assign-me",
    label: "Assign to me",
    group: "Conversation",
    icon: RiUserReceivedLine,
    shortcut: "Shift+A",
    keys: ["Shift", "A"],
    when: (ctx) => ctx.conversation !== null,
    run: (ctx) => ctx.conversation?.assignToMe(),
  },
];
