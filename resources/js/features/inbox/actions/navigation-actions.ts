/*
 * The moves that go somewhere: the views by G and a letter, the view
 * picker by V, the command menu, the shortcut sheet and the search box.
 */
import {
  RiCommandLine,
  RiInbox2Line,
  RiKeyboardLine,
  RiLayoutColumnLine,
  RiQuestionAnswerLine,
  RiSearchLine,
  RiSpam2Line,
  RiStackLine,
} from "@remixicon/react";

import type { InboxAction } from "./registry";

function goTo(id: string, label: string, letter: string, path: string, icon: InboxAction["icon"]): InboxAction {
  return {
    id,
    label,
    group: "Navigation",
    icon,
    shortcut: `g ${letter}`,
    keys: ["G", letter.toUpperCase()],
    when: () => true,
    run: (ctx) => ctx.go(path),
  };
}

export const navigationActions: InboxAction[] = [
  goTo("go.mine", "Go to my inbox", "m", "/d/inbox/mine", RiInbox2Line),
  goTo("go.inbox", "Go to the inbox", "i", "/d/inbox", RiQuestionAnswerLine),
  goTo("go.unassigned", "Go to unassigned", "u", "/d/inbox/unassigned", RiQuestionAnswerLine),
  goTo("go.all", "Go to all conversations", "a", "/d/inbox/all", RiStackLine),
  goTo("go.spam", "Go to spam", "s", "/d/inbox/spam", RiSpam2Line),
  {
    id: "help.command-menu",
    label: "Open the command menu",
    group: "Help",
    icon: RiCommandLine,
    shortcut: "$mod+k",
    keys: ["⌘", "K"],
    allowInInput: true,
    when: () => true,
    run: (ctx) => ctx.open("command-menu"),
  },
  {
    id: "help.shortcuts",
    label: "Keyboard shortcuts",
    group: "Help",
    icon: RiKeyboardLine,
    shortcut: "Shift+?",
    keys: ["?"],
    when: () => true,
    run: (ctx) => ctx.open("shortcuts"),
  },
  {
    id: "go.view",
    label: "Switch view",
    group: "Navigation",
    icon: RiLayoutColumnLine,
    shortcut: "v",
    keys: ["V"],
    when: () => true,
    run: () => document.querySelector<HTMLButtonElement>("button[data-view-picker]")?.click(),
  },
  {
    id: "go.search",
    label: "Search the list",
    group: "Navigation",
    icon: RiSearchLine,
    shortcut: "/",
    keys: ["/"],
    when: () => true,
    run: () => document.querySelector<HTMLInputElement>("input[aria-label='Search conversations']")?.focus(),
  },
];
