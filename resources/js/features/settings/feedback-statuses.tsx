/*
 * The statuses of feature requests, in the order the portal's roadmap
 * and every status list in the app show them. A member who may change
 * settings drags a row by its handle, with the pointer or the keyboard,
 * and the new order saves on drop. The list moves at once and goes back,
 * with a note, when the save fails. The drag hooks are always handed to
 * the list and only disabled without the permission, since the list may
 * not gain or lose them between renders.
 */
import { RiDraggable } from "@remixicon/react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Button as AriaButton, DropIndicator, GridList, GridListItem, useDragAndDrop } from "react-aria-components";

import { toast } from "@/components/base/toast/toast";
import { StatusDot } from "@/features/feedback/status-chip";
import { useStatuses } from "@/features/feedback/use-statuses";
import { type FeedbackStatus, feedbackKeys, reorderStatuses } from "@/lib/api/feedback";
import { cx } from "@/utils/cx";

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

export function FeedbackStatusesSection({ action }: { action?: React.ReactNode }) {
  const workspace = useAccount().workspace.id;
  const list = useStatuses(workspace);
  const canChange = usePermission(workspace, "settings:write");
  const reorder = useReorder(workspace);
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
    <Section title="Statuses" description="Drag to set the order of the roadmap and every status list." action={action}>
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
            <span className="text-body-regular text-text-primary">{status.name}</span>
          </GridListItem>
        )}
      </GridList>
    </Section>
  );
}
