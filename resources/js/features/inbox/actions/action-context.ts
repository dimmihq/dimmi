/*
 * Where the moves' context lives while the app runs: the open
 * conversation registers what it lets a move do, and the command menu and
 * shortcut sheet open from anywhere.
 */
import { createContext, useContext, useEffect } from "react";

import type { ConversationMoves } from "./registry";

export interface ActionState {
  setConversation: (moves: ConversationMoves | null) => void;
  open: (panel: "command-menu" | "shortcuts") => void;
}

export const ActionStateContext = createContext<ActionState>({
  setConversation: () => undefined,
  open: () => undefined,
});

/*
 * useActionState returns the setters of the moves' context.
 */
export function useActionState(): ActionState {
  return useContext(ActionStateContext);
}

/*
 * useConversationMoves offers the open conversation's moves to shortcuts
 * and the command menu while it is open.
 */
export function useConversationMoves(moves: ConversationMoves) {
  const { setConversation } = useActionState();

  useEffect(() => {
    setConversation(moves);
    return () => setConversation(null);
  }, [moves, setConversation]);
}
