/*
 * The open conversation's less frequent moves, behind one button in its
 * header: marking it unread again, marking it as spam or taking it out,
 * and merging it into another of the customer's conversations. Each has
 * a shortcut and a place in the command menu too.
 */
import { RiMailUnreadLine, RiMergeCellsHorizontal, RiSpam2Line } from "@remixicon/react";

import { type RowAction, RowMenu } from "@/features/shell/row-menu";

import type { ConversationMoves } from "./actions/registry";

type Kind = "unread" | "spam" | "merge";

export function ConversationMenu({ moves }: { moves: ConversationMoves }) {
  const actions: RowAction<Kind>[] = [
    { kind: "unread", label: "Mark as unread", icon: RiMailUnreadLine },
    { kind: "merge", label: "Merge into…", icon: RiMergeCellsHorizontal },
    { kind: "spam", label: moves.spam ? "Not spam" : "Mark as spam", icon: RiSpam2Line, danger: !moves.spam },
  ];
  const run: Record<Kind, () => void> = { unread: moves.markUnread, spam: moves.toggleSpam, merge: moves.merge };

  return <RowMenu label="this conversation" actions={actions} onPick={(kind) => run[kind]()} />;
}
