/*
 * The boards of the default help center's feedback portal, in the order
 * its tabs show them. A member who may change settings adds a board,
 * edits who sees it and whether it holds posts for approval, drags rows
 * into a new order, and deletes a board after picking where its requests
 * go. The last board stays.
 */
import { RiDeleteBinLine, RiDraggable, RiEditLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Button as AriaButton, DropIndicator, GridList, GridListItem, useDragAndDrop } from "react-aria-components";

import { toast } from "@/components/base/toast/toast";
import { RowMenu } from "@/features/shell/row-menu";
import {
  type BoardChange,
  boardsQuery,
  createBoard,
  deleteBoard,
  type FeedbackBoard,
  feedbackKeys,
  reorderBoards,
  updateBoard,
  visibilities,
} from "@/lib/api/feedback";
import { cx } from "@/utils/cx";

import { Action } from "./controls";
import { BoardDialog, MoveAndDelete } from "./feedback-dialogs";
import { usePermission } from "./inboxes";
import { Section } from "./layout";
import { moved } from "./status-order";

type Open = { kind: "edit"; board: FeedbackBoard | null } | { kind: "delete"; board: FeedbackBoard } | null;

const actions = [
  { kind: "edit" as const, label: "Edit", icon: RiEditLine },
  { kind: "delete" as const, label: "Delete", icon: RiDeleteBinLine, danger: true },
];

function detailOf(b: FeedbackBoard): string {
  const who = visibilities.find((v) => v.id === b.visibility)?.name ?? b.visibility;
  const count = b.post_count === 1 ? "1 request" : `${b.post_count} requests`;
  return [who, count, b.approval_required ? "Approval first" : null].filter(Boolean).join(" · ");
}

export function FeedbackBoardsSection({ workspace, helpCenter }: { workspace: string; helpCenter: string }) {
  const client = useQueryClient();
  const key = feedbackKeys.boards(workspace, helpCenter);
  const { data: boards = [], isPending } = useQuery(boardsQuery(workspace, helpCenter));
  const canChange = usePermission(workspace, "settings:write");
  const [open, setOpen] = useState<Open>(null);
  const refresh = () => client.invalidateQueries({ queryKey: key });
  const reorder = useMutation({
    mutationFn: (next: FeedbackBoard[]) =>
      reorderBoards(
        workspace,
        helpCenter,
        next.map((b) => b.id),
      ),
    onMutate: (next) => {
      const previous = client.getQueryData<FeedbackBoard[]>(key);
      client.setQueryData(key, next);
      return { previous };
    },
    onError: (_error, _next, context) => {
      client.setQueryData(key, context?.previous);
      toast.error("The new order was not saved.");
    },
    onSuccess: (saved) => client.setQueryData(key, saved),
  });
  const save = async (board: FeedbackBoard | null, change: BoardChange) => {
    try {
      await (board ? updateBoard(workspace, board.id, change) : createBoard(workspace, helpCenter, change));
      await refresh();
      toast.success(board ? "Board saved" : "Board added");
    } catch (error) {
      toast.error((error as Error).message);
      throw error;
    }
  };
  const { dragAndDropHooks } = useDragAndDrop({
    isDisabled: !canChange,
    getItems: (keys) => [...keys].map((k) => ({ "text/plain": String(k) })),
    onReorder: (event) => {
      const order = moved(
        boards.map((b) => b.id),
        event.keys,
        event.target.key,
        event.target.dropPosition,
      );
      reorder.mutate(order.flatMap((id) => boards.find((b) => b.id === id) ?? []));
    },
    renderDropIndicator: (target) => (
      <DropIndicator
        target={target}
        className={({ isDropTarget }) => cx("me-2.5 h-0.5 rounded-full", isDropTarget ? "bg-accent-500" : "")}
      />
    ),
  });

  return (
    <Section
      title="Boards"
      description="Tabs of your portal. Drag to set their order."
      loading={isPending}
      rows={2}
      action={canChange ? <Action onPress={() => setOpen({ kind: "edit", board: null })}>New board</Action> : null}
    >
      <GridList aria-label="Boards" items={boards} dragAndDropHooks={dragAndDropHooks} className="flex flex-col">
        {(board) => (
          <GridListItem
            id={board.id}
            textValue={board.name}
            className={({ isDragging, isFocusVisible }) =>
              cx(
                "flex min-h-[60px] items-center gap-3 border-b border-separator-border py-2.5 pe-2.5 outline-none last:border-b-0",
                isDragging && "opacity-50",
                isFocusVisible && "rounded-md ring-2 ring-border-focus-ring",
              )
            }
          >
            {canChange ? (
              <AriaButton
                slot="drag"
                aria-label={`Move ${board.name}`}
                className="flex size-6 shrink-0 cursor-grab items-center justify-center rounded-md text-foreground-icon-tertiary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
              >
                <RiDraggable aria-hidden className="size-4" />
              </AriaButton>
            ) : null}
            <span className="flex min-w-0 flex-1 flex-col">
              <span className="truncate text-body-regular text-text-primary">{board.name}</span>
              <span className="truncate text-body-2-regular text-text-secondary">{detailOf(board)}</span>
            </span>
            {canChange ? (
              <RowMenu
                label={board.name}
                actions={boards.length > 1 ? actions : actions.slice(0, 1)}
                onPick={(kind) => setOpen({ kind, board })}
              />
            ) : null}
          </GridListItem>
        )}
      </GridList>
      {open?.kind === "edit" ? (
        <BoardDialog board={open.board} onSave={(change) => save(open.board, change)} onClose={() => setOpen(null)} />
      ) : null}
      {open?.kind === "delete" ? (
        <MoveAndDelete
          noun="board"
          name={open.board.name}
          count={open.board.post_count}
          options={boards.filter((b) => b.id !== open.board.id)}
          onConfirm={async (target) => {
            await deleteBoard(workspace, open.board.id, target);
            await refresh();
            setOpen(null);
          }}
          onClose={() => setOpen(null)}
        />
      ) : null}
    </Section>
  );
}
