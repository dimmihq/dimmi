/*
 * The open conversation's less frequent moves, behind one button in its
 * header: marking it unread again, marking it as spam or taking it out,
 * merging it into another of the customer's conversations, and turning
 * the AI agent off it or back on. Each has
 * a shortcut and a place in the command menu too.
 */
import {
  RiMailUnreadLine,
  RiMergeCellsHorizontal,
  RiPauseCircleLine,
  RiPlayCircleLine,
  RiSpam2Line,
} from "@remixicon/react";

import { type RowAction, RowMenu } from "@/features/shell/row-menu";

import type { ConversationMoves } from "./actions/registry";

type Kind = "unread" | "spam" | "merge" | "ai";

export function ConversationMenu({ moves }: { moves: ConversationMoves }) {
  const actions: RowAction<Kind>[] = [
    { kind: "unread", label: "Mark as unread", icon: RiMailUnreadLine },
    { kind: "merge", label: "Merge into…", icon: RiMergeCellsHorizontal },
    moves.aiOff
      ? { kind: "ai", label: "Turn on AI", icon: RiPlayCircleLine }
      : { kind: "ai", label: "Turn off AI", icon: RiPauseCircleLine },
    { kind: "spam", label: moves.spam ? "Not spam" : "Mark as spam", icon: RiSpam2Line, danger: !moves.spam },
  ];
  const run: Record<Kind, () => void> = {
    unread: moves.markUnread,
    spam: moves.toggleSpam,
    merge: moves.merge,
    ai: moves.toggleAI,
  };

  return <RowMenu label="this conversation" actions={actions} onPick={(kind) => run[kind]()} />;
}
