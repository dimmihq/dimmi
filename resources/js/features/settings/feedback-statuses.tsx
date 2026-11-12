/*
 * The statuses of feature requests, in the order the portal's roadmap
 * and every status list in the app show them. A member who may change
 * settings adds a status, renames, recolors and recategorizes one,
 * deletes one after picking where its requests go, and drags a row by
 * its handle, with the pointer or the keyboard; the new order saves on
 * drop. The list moves at once and goes back,
 * with a note, when the save fails. The drag hooks are always handed to
 * the list and only disabled without the permission, since the list may
 * not gain or lose them between renders.
 */
import { RiDeleteBinLine, RiDraggable, RiEditLine } from "@remixicon/react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Button as AriaButton, DropIndicator, GridList, GridListItem, useDragAndDrop } from "react-aria-components";

import { toast } from "@/components/base/toast/toast";
import { StatusDot } from "@/features/feedback/status-chip";
import { useStatuses } from "@/features/feedback/use-statuses";
import { RowMenu } from "@/features/shell/row-menu";
import {
  categories,
  createStatus,
  deleteStatus,
  type FeedbackStatus,
  feedbackKeys,
  reorderStatuses,
  updateStatus,
} from "@/lib/api/feedback";
import { cx } from "@/utils/cx";

import { Action } from "./controls";
import { MoveAndDelete, StatusDialog } from "./feedback-dialogs";
import { usePermission } from "./inboxes";
import { Section } from "./layout";
import { moved } from "./status-order";
import { useAccount } from "./use-account";

function useReorder(workspace: string) {
  const client = useQueryClient();
  const key = feedbackKeys.statuses(workspace);

  return useMutation({
    mutationFn: (next: FeedbackStatus[]) =>
      reorderStatuses(
        workspace,
        next.map((s) => s.id),
      ),
    onMutate: async (next) => {
      await client.cancelQueries({ queryKey: key });
      const previous = client.getQueryData<FeedbackStatus[]>(key);
      client.setQueryData(key, next);

      return { previous };
    },
    onError: (_error, _next, context) => {
      client.setQueryData(key, context?.previous);
      toast.error("The new order was not saved.");
    },
    onSuccess: (saved) => client.setQueryData(key, saved),
  });
}

type Open = { kind: "edit"; status: FeedbackStatus | null } | { kind: "delete"; status: FeedbackStatus } | null;

const actions = [
  { kind: "edit" as const, label: "Edit", icon: RiEditLine },
  { kind: "delete" as const, label: "Delete", icon: RiDeleteBinLine, danger: true },
];

function detailOf(s: FeedbackStatus): string {
  const category = categories.find((c) => c.id === s.category)?.name ?? s.category;
  return s.roadmap ? `${category} · On the roadmap` : category;
}

export function FeedbackStatusesSection() {
  const workspace = useAccount().workspace.id;
  const client = useQueryClient();
  const list = useStatuses(workspace);
  const canChange = usePermission(workspace, "settings:write");
  const reorder = useReorder(workspace);
  const [open, setOpen] = useState<Open>(null);
  const refresh = () => client.invalidateQueries({ queryKey: ["feedback", workspace] });
  const save = async (status: FeedbackStatus | null, change: Parameters<typeof createStatus>[1] & { name: string }) => {
    try {
      await (status ? updateStatus(workspace, status.id, change) : createStatus(workspace, change));
      await refresh();
      toast.success(status ? "Status saved" : "Status added");
    } catch (error) {
      toast.error((error as Error).message);
      throw error;
    }
  };
  const { dragAndDropHooks } = useDragAndDrop({
    isDisabled: !canChange,
    getItems: (keys) => [...keys].map((key) => ({ "text/plain": String(key) })),
    onReorder: (event) => {
      const order = moved(
        list.map((s) => s.id),
        event.keys,
        event.target.key,
        event.target.dropPosition,
      );
      reorder.mutate(order.flatMap((id) => list.find((s) => s.id === id) ?? []));
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
      title="Statuses"
      description="Where requests stand. Drag to set the order of the roadmap and every status list."
      action={canChange ? <Action onPress={() => setOpen({ kind: "edit", status: null })}>New status</Action> : null}
    >
      <GridList
        aria-label="Statuses"
        items={list}
        dragAndDropHooks={dragAndDropHooks}
        className="flex flex-col outline-none"
      >
        {(status) => (
          <GridListItem
            id={status.id}
            textValue={status.name}
            className={({ isDragging, isFocusVisible }) =>
              cx(
                "flex min-h-[52px] items-center gap-3 border-b border-separator-border py-2.5 pe-2.5 outline-none last:border-b-0",
                isDragging && "opacity-50",
                isFocusVisible && "rounded-md ring-2 ring-border-focus-ring",
              )
            }
          >
            {canChange ? (
              <AriaButton
                slot="drag"
                aria-label={`Move ${status.name}`}
                className="flex size-6 shrink-0 cursor-grab items-center justify-center rounded-md text-foreground-icon-tertiary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
              >
                <RiDraggable aria-hidden className="size-4" />
              </AriaButton>
            ) : null}
            <StatusDot color={status.color} />
            <span className="flex min-w-0 flex-1 flex-col">
              <span className="truncate text-body-regular text-text-primary">{status.name}</span>
              <span className="truncate text-body-2-regular text-text-secondary">{detailOf(status)}</span>
            </span>
            {canChange ? (
              <RowMenu
                label={status.name}
                actions={list.length > 1 ? actions : actions.slice(0, 1)}
                onPick={(kind) => setOpen({ kind, status })}
              />
            ) : null}
          </GridListItem>
        )}
      </GridList>
      {open?.kind === "edit" ? (
        <StatusDialog
          status={open.status}
          onSave={(change) => save(open.status, change)}
          onClose={() => setOpen(null)}
        />
      ) : null}
      {open?.kind === "delete" ? (
        <MoveAndDelete
          noun="status"
          name={open.status.name}
          options={list.filter((s) => s.id !== open.status.id)}
          onConfirm={async (target) => {
            await deleteStatus(workspace, open.status.id, target);
            await refresh();
            setOpen(null);
          }}
          onClose={() => setOpen(null)}
        />
      ) : null}
    </Section>
  );
}
