/*
 * Gives the dashboard its keyboard: binds every move's shortcut, and
 * holds the command menu and the shortcut sheet, which open from any
 * page.
 */
import { useNavigate } from "@tanstack/react-router";
import { type ReactNode, useCallback, useMemo, useState } from "react";

import { CommandMenu } from "@/features/command-menu/command-menu";
import { useInbox } from "../conversations";
import { ShortcutSheet } from "../keyboard/shortcut-sheet";
import { useShortcuts } from "../keyboard/use-shortcuts";
import { ActionStateContext } from "./action-context";
import { allActions } from "./all-actions";
import type { ActionContext, ConversationMoves } from "./registry";

export function ActionProvider({ children }: { children: ReactNode }) {
  const navigate = useNavigate();
  const { workspace } = useInbox();
  const [conversation, setConversation] = useState<ConversationMoves | null>(null);
  const [panel, setPanel] = useState<"command-menu" | "shortcuts" | null>(null);
  const open = useCallback((next: "command-menu" | "shortcuts") => setPanel(next), []);
  const ctx: ActionContext = {
    workspace,
    conversation,
    go: (path) => void navigate({ to: path }),
    open,
  };
  useShortcuts(allActions, ctx);
  const state = useMemo(() => ({ setConversation, open }), [open]);

  return (
    <ActionStateContext.Provider value={state}>
      {children}
      <CommandMenu
        isOpen={panel === "command-menu"}
        onOpenChange={(next) => setPanel(next ? "command-menu" : null)}
        context={ctx}
      />
      <ShortcutSheet isOpen={panel === "shortcuts"} onOpenChange={(next) => setPanel(next ? "shortcuts" : null)} />
    </ActionStateContext.Provider>
  );
}
